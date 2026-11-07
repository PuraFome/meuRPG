import {
  ChangeDetectionStrategy,
  Component,
  computed,
  inject,
  input,
  output,
  signal,
} from '@angular/core';
import { Code, ConnectError } from '@connectrpc/connect';

import type { Encounter } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { CombatClient } from '../../../../core/combat/combat-client';
import { combatErrorMessage } from '../../../../core/combat/combat-errors';
import type { CombatLogState } from '../../../../core/combat/combat-log-state';
import type { CombatState } from '../../../../core/combat/combat-state';
import { CombatUndone } from '../../../../core/combat/combat-undone';
import { type LegendaryNames, type LegendaryRow } from '../../../../core/combat/legendary-card';
import { legendarySpentLine } from '../../../../core/combat/legendary-left';
import { masterQueue } from '../../../../core/combat/reaction-queue';
import { ActionKey } from '../../../../core/connect/idempotency';
import { offerFailure } from '../reaction-queue/offer-answer';
import { LegendaryCard, type LegendaryFailure } from './legendary-card';

/** The legendary option the master armed to move: its token is dragged on the map next. */
export interface ArmedMove {
  readonly combatantId: string;
  readonly optionKey: string;
}

/** A tap on an attack or a move row: which creature, which option. */
export interface LegendaryPick {
  readonly combatantId: string;
  readonly row: LegendaryRow;
}

/**
 * The legendary actions of the master's screen (#557): one card for each legendary creature that is not on turn, under
 * the windows the master answers (the queue projects this into its "rest" part). It owns what belongs to the screen and
 * is never sent: which card is folded (kept per creature, for the turn it was folded on, so an encounter update does not
 * open it again), and the answers in the air. "Usar" spends the option's cost with `SpendLegendaryAction`, under a key
 * kept across a retry; "Atacar" and a move hand the pick to the page (the attack sheet and the drag of the token); the
 * undo is the log's own (`UndoLastAction` of the log's undoable event). Master only: the page never draws it for a player.
 */
@Component({
  selector: 'app-legendary-offers',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [LegendaryCard],
  template: `
    @for (card of cards(); track card.combatantId) {
      <app-legendary-card
        [card]="card"
        [folded]="isFolded(card.combatantId)"
        [sending]="sending()?.combatantId === card.combatantId ? sending()!.key : ''"
        [failure]="failures()[card.combatantId] ?? null"
        [spentLine]="spentLine(card.combatantId)"
        [undoing]="undoing()"
        [armed]="armed()?.combatantId === card.combatantId ? armed()!.optionKey : ''"
        [theatre]="theatre()"
        (fold)="setFolded(card.combatantId, true)"
        (unfold)="setFolded(card.combatantId, false)"
        (use)="pick(card.combatantId, $event)"
        (retry)="retry(card.combatantId)"
        (undo)="undo()"
        (cancelMove)="cancelMove.emit()"
      />
    }
  `,
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
})
export class LegendaryOffers {
  private readonly api = inject(CombatClient);
  private readonly undone = inject(CombatUndone);
  private readonly keys = new ActionKey();

  readonly encounter = input.required<Encounter>();
  readonly campaignId = input.required<string>();
  readonly state = input.required<CombatState>();
  readonly log = input.required<Pick<CombatLogState, 'undoable' | 'undoableId'>>();
  /** The options' Portuguese names of the creatures seen so far (`legendaryNames`), for the line of a spend. */
  readonly names = input<LegendaryNames | undefined>(undefined);
  readonly theatre = input(false);
  readonly armed = input<ArmedMove | null>(null);

  /** "Atacar": the page opens the attack sheet of the creature with the option. */
  readonly attack = output<LegendaryPick>();
  /** A move row: the page arms the drag of the creature's token with the option. */
  readonly move = output<LegendaryPick>();
  readonly cancelMove = output<void>();

  protected readonly cards = computed(() => masterQueue(this.encounter()).legendary);
  protected readonly sending = signal<{ combatantId: string; key: string } | null>(null);
  protected readonly failures = signal<Readonly<Record<string, LegendaryFailure | undefined>>>({});
  protected readonly undoing = signal(false);
  /** The turn the cards were folded on, and which ones: a new turn opens them all again. */
  private readonly foldedOn = signal<{ turn: string; ids: ReadonlySet<string> }>({
    turn: '',
    ids: new Set(),
  });
  private readonly last = new Map<string, LegendaryRow>();

  private readonly turn = computed(() => {
    const e = this.encounter();
    return `${e.id}:${e.round}:${e.currentCombatantId}`;
  });

  protected isFolded(id: string): boolean {
    const f = this.foldedOn();
    return f.turn === this.turn() && f.ids.has(id);
  }

  /** "Agora não" and "Abrir": client only, nothing is sent. */
  protected setFolded(id: string, on: boolean): void {
    const f = this.foldedOn();
    const ids = new Set(f.turn === this.turn() ? f.ids : []);
    if (on) {
      ids.add(id);
    } else {
      ids.delete(id);
    }
    this.foldedOn.set({ turn: this.turn(), ids });
  }

  /** The line of the last spend, under the card of the creature that made it, while the log says it can be undone. */
  protected spentLine(id: string): string {
    const entry = this.log().undoable();
    return entry?.actorId === id ? legendarySpentLine(entry, this.names()) : '';
  }

  protected pick(combatantId: string, row: LegendaryRow): Promise<void> {
    this.setFailure(combatantId, null);
    if (row.action === 'attack') {
      this.attack.emit({ combatantId, row });
      return Promise.resolve();
    }
    if (row.action === 'move') {
      this.move.emit({ combatantId, row });
      return Promise.resolve();
    }
    this.last.set(combatantId, row);
    return this.spend(combatantId, row);
  }

  protected retry(combatantId: string): Promise<void> {
    const row = this.last.get(combatantId);
    return row ? this.spend(combatantId, row) : Promise.resolve();
  }

  /** The same option again after a lost answer is a retry: it keeps its key, so the cost is not charged twice. */
  private async spend(combatantId: string, row: LegendaryRow): Promise<void> {
    if (this.sending()) {
      return;
    }
    const e = this.encounter();
    this.sending.set({ combatantId, key: row.key });
    this.setFailure(combatantId, null);
    try {
      const next = await this.api.combatants.spendLegendaryAction(
        this.campaignId(),
        e.id,
        combatantId,
        row.key,
        this.keys.keyFor({ encounter: e.id, combatantId, option: row.key }),
      );
      this.keys.renew();
      this.state().apply(next);
    } catch (err) {
      this.setFailure(combatantId, offerFailure(err, 'usar a ação lendária'));
    } finally {
      this.sending.set(null);
    }
  }

  /** "Desfazer última ação": the log's own undo, so the cost goes back with the line. */
  protected async undo(): Promise<void> {
    const id = this.log().undoableId();
    if (!id || this.undoing()) {
      return;
    }
    this.undoing.set(true);
    try {
      this.state().apply(await this.api.turn.undo(this.campaignId(), this.encounter().id, id));
      this.undone.mark();
    } catch (err) {
      const who = this.log().undoable()?.actorId ?? '';
      this.setFailure(
        who,
        ConnectError.from(err).code === Code.Aborted
          ? { text: 'Outra ação aconteceu antes: confira o registro.', connection: false }
          : { text: combatErrorMessage(err, 'desfazer a ação'), connection: false },
      );
    } finally {
      this.undoing.set(false);
    }
  }

  private setFailure(id: string, failure: LegendaryFailure | null): void {
    this.failures.update((all) => {
      const { [id]: _gone, ...rest } = all;
      return failure ? { ...rest, [id]: failure } : rest;
    });
  }
}
