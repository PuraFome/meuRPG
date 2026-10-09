import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  input,
  signal,
  untracked,
} from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';

import type { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  type Encounter,
  type GetTurnOptionsResponse,
  EncounterStatus,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import { ContestStatus } from '../../../../../gen/meurpg/play/v1/contest_types_pb';
import { combatErrorMessage } from '../../../../core/combat/combat-errors';
import type { CombatState } from '../../../../core/combat/combat-state';
import { ContestClient } from '../../../../core/combat/contest-client';
import { ContestState } from '../../../../core/combat/contest-state';
import { currentCombatant, isPlayer } from '../../../../core/combat/combat-view';
import { named, whoIs } from '../../../../core/combat/contest-view';
import { contestIdOf, contestWindows } from '../../../../core/combat/reactions';
import { ActionKey } from '../../../../core/connect/idempotency';
import { ContestCard } from './contest-card';
import { HideMasterCard } from './hide-master-card';
import { openNpcGrappleSheet } from './npc-grapple-sheet';

let nextId = 0;

/**
 * The master's side of the contests of a combat (W7-X, boards W7-Xa 3, W7-Xb 4 to 7 and W7-Xc 8): under the reaction queue, in the
 * order the server gives. A contest that waits for him is the entry of its CONTEST window (`forYou`): the answer card, or the
 * shove's choice. A Hide waits in its own card. The grapples he can read ("O Hobgoblin segura Brisa", with the escape DC that is
 * his alone) each have "Soltar". On an NPC's turn, "Agarrar ou empurrar por um NPC" opens the sheet where he rolls for the
 * creature or sets its fixed escape DC. It reads the contest facts again whenever the combat changes (every contest change reaches
 * the table as `encounter_changed`). Nothing is kept in the browser; a CONTEST window is never answered with `AnswerReaction`.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-contest-master',
  imports: [ContestCard, HideMasterCard, MatButtonModule, MatIconModule],
  styleUrl: './master-card.scss',
  styles: `
    :host {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-3);
    }
    :host:empty {
      display: none;
    }
  `,
  template: `
    @for (c of cards(); track c.id) {
      <app-contest-card
        [contest]="c"
        [encounter]="encounter()"
        [campaignId]="campaignId()"
        [state]="state()"
        [contests]="contests"
        [diceMode]="diceMode()"
        [preference]="preference()"
        (said)="said.set($event)"
      />
    }
    @for (a of contests.pendingAttempts(); track a.id) {
      <app-hide-master-card
        [attempt]="a"
        [encounter]="encounter()"
        [campaignId]="campaignId()"
        [state]="state()"
        [contests]="contests"
        (said)="said.set($event)"
      />
    }
    @if (grapples().length > 0 || npcTurn()) {
      <section class="card" [attr.aria-labelledby]="uid + 'g'" data-testid="grapples">
        <h2 class="card__title" [id]="uid + 'g'">Agarrões</h2>
        @if (grapples().length > 0) {
          <ul class="rows">
            @for (g of grapples(); track g.grappledId) {
              <li class="row">
                <span class="row__main">
                  <span class="row__name">{{ g.text }}</span>
                  @if (g.dc) {
                    <span class="row__sub">CD de escape {{ g.dc }}</span>
                  }
                </span>
                <button
                  mat-stroked-button
                  type="button"
                  class="btn btn--small"
                  [attr.aria-label]="'Soltar ' + g.grappledLabel"
                  [disabled]="busy()"
                  (click)="release(g.grappledId)"
                >
                  Soltar
                </button>
              </li>
            }
          </ul>
        }
        @if (npcTurn()) {
          <div class="btns">
            <button mat-stroked-button type="button" class="btn" [disabled]="busy()" (click)="openNpcGrapple()">
              Agarrar ou empurrar por um NPC
            </button>
          </div>
        }
      </section>
    }
    @if (error()) {
      <div class="mr-notice mr-notice--danger" role="alert">
        <mat-icon aria-hidden="true">error</mat-icon>
        <p>{{ error() }}</p>
      </div>
    }
    <span class="mr-visually-hidden" role="status" aria-live="polite">{{ said() }}</span>
  `,
})
export class ContestMaster {
  private readonly api = inject(ContestClient);
  private readonly dialog = inject(MatDialog);
  private readonly bottomSheet = inject(MatBottomSheet);
  private readonly keys = new ActionKey();

  readonly encounter = input.required<Encounter | null>();
  readonly campaignId = input.required<string>();
  readonly state = input.required<CombatState>();
  readonly diceMode = input.required<DiceMode>();
  readonly preference = input.required<DicePreference>();
  /** The master's `GetTurnOptions` of the creature whose turn it is (the special attacks, for its Athletics modifier). */
  readonly options = input<GetTurnOptionsResponse | null>(null);

  /** The contest facts of this combat, read as the master (all of them). */
  readonly contests = new ContestState();

  protected readonly uid = `cm-${nextId++}-`;
  protected readonly busy = signal(false);
  protected readonly error = signal('');
  protected readonly said = signal('');

  /** The contests that wait in a CONTEST window the master answers, in the order of the windows. */
  protected readonly cards = computed(() => {
    const e = this.encounter();
    if (!e) {
      return [];
    }
    return contestWindows(e)
      .filter((w) => w.forYou)
      .flatMap((w) => {
        const c = this.contests.contest(contestIdOf(w));
        const mine =
          c &&
          ((c.status === ContestStatus.AWAITING_DEFENDER && c.youAnswer) ||
            (c.status === ContestStatus.AWAITING_OUTCOME && c.youChoose));
        return c && mine ? [c] : [];
      });
  });
  protected readonly grapples = computed(() => {
    const e = this.encounter();
    return this.contests.grapples().map((g) => {
      const grappler = e?.combatants.find((c) => c.id === g.grapplerId);
      const grappled = e?.combatants.find((c) => c.id === g.grappledId);
      const who = named(whoIs(grappler));
      return {
        grappledId: g.grappledId,
        grappledLabel: grappled?.label ?? '',
        dc: g.escapeDc,
        text: `${who.charAt(0).toUpperCase()}${who.slice(1)} segura ${named(whoIs(grappled))}`,
      };
    });
  });
  /** An NPC's turn: the master can make its creature grapple or shove. */
  protected readonly npcTurn = computed(() => {
    const e = this.encounter();
    const turn = e && e.status === EncounterStatus.ACTIVE ? currentCombatant(e) : null;
    return !!turn && !isPlayer(turn) && !turn.defeated;
  });

  constructor() {
    effect(() => {
      const e = this.encounter();
      const campaignId = this.campaignId();
      if (!e || e.status !== EncounterStatus.ACTIVE) {
        untracked(() => this.contests.clear());
        return;
      }
      void e.revision;
      untracked(() => void this.contests.load(this.api, campaignId, e.id));
    });
  }

  protected openNpcGrapple(): void {
    const e = this.encounter();
    const turn = e ? currentCombatant(e) : null;
    if (!e || !turn) {
      return;
    }
    openNpcGrappleSheet(this.dialog, this.bottomSheet, {
      campaignId: this.campaignId(),
      encounterId: e.id,
      state: this.state(),
      contests: this.contests,
      diceMode: this.diceMode(),
      preference: this.preference(),
      initiatorId: turn.id,
      attacks: this.options()?.contestAttackOptions ?? [],
    }).subscribe();
  }

  protected async release(grappledId: string): Promise<void> {
    const e = this.encounter();
    if (!e || this.busy()) {
      return;
    }
    this.busy.set(true);
    this.error.set('');
    try {
      const next = await this.api.releaseGrapple(
        this.campaignId(),
        e.id,
        grappledId,
        this.keys.keyFor({ release: grappledId }),
      );
      this.state().apply(next);
      this.keys.renew();
      this.said.set('Agarrão solto.');
      void this.contests.load(this.api, this.campaignId(), e.id);
    } catch (err) {
      this.error.set(combatErrorMessage(err, 'soltar'));
    } finally {
      this.busy.set(false);
    }
  }
}
