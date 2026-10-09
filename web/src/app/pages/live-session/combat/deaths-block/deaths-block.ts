import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  computed,
  effect,
  inject,
  input,
  signal,
  untracked,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import {
  type CombatDeath,
  type Combatant,
  type Encounter,
  EncounterStatus,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  type LivingRefusal,
  describeCharacterError,
  livingRefusal,
} from '../../../../core/characters/character-errors';
import { describeConnectError } from '../../../../core/connect/connect-errors';
import { ActionKey } from '../../../../core/connect/idempotency';
import { combatantInitial, isPlayer } from '../../../../core/combat/combat-view';
import { isCreature } from '../../../../core/combat/creature-names';
import { article } from '../../../../core/format/article';
import { joinDots } from '../../../../core/format/text';
import { RevivifyClient } from '../../../../core/revivify/revivify-client';
import { CombatantToken } from '../../../../shared/combatant-token/combatant-token';
import { HiddenSwitch } from '../../../../shared/hidden-switch/hidden-switch';
import { ReviveBlocked } from '../../../../shared/revive/revive-blocked';
import { ReviveConfirm } from '../../../../shared/revive/revive-confirm';
import type { CombatantInfo } from '../combat-info';

/** Revivify reaches a creature that died up to this many rounds ago ("um minuto são 10 rodadas"). */
const WINDOW_ROUNDS = 10;

/** One creature of the list, ready to draw. */
interface DeathRow {
  readonly death: CombatDeath;
  readonly combatant: Combatant | undefined;
  /** "Guerreiro 4 · morreu na rodada 3 (há 5)". */
  readonly line: string;
  readonly fits: boolean;
  readonly blocked: boolean;
  /** "ela" or "ele", for the notice of the switch. */
  readonly pronoun: string;
}

/**
 * "Mortos nesta luta (2)": the end of the master's order of initiative (E6-11, PM-08d state 4). It lists who died
 * in this combat (a player's character the master confirmed, an NPC taken to 0 hit points) with the round, how
 * long ago, and whether Revivify still fits in time ("Cabe em Revivificar" while 10 rounds or fewer have passed
 * and the master's switch is off). A player's character has "Reviver", the master's power with the same question
 * and the same refusal as on its page; an NPC has none. The switch "Revivificar não funciona nesta morte" takes
 * the creature out of the player's list of targets, with no reason; only the master sees it. The reasons a
 * creature does not fit stay with the master and are not drawn here.
 */
@Component({
  selector: 'app-deaths-block',
  imports: [
    CombatantToken,
    HiddenSwitch,
    MatButtonModule,
    MatIconModule,
    ReviveBlocked,
    ReviveConfirm,
  ],
  templateUrl: './deaths-block.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
  styleUrl: './deaths-block.scss',
})
export class DeathsBlock {
  private readonly api = inject(RevivifyClient);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);

  readonly campaignId = input.required<string>();
  readonly encounter = input.required<Encounter>();
  /** The class line and the kind of NPC, by character. */
  readonly info = input<ReadonlyMap<string, CombatantInfo>>(new Map());

  /** The switch as the master just set it, until the combat the server sends says the same. */
  private readonly blockedNow = signal<ReadonlyMap<string, boolean>>(new Map());
  /** Characters that were brought back here: their rows go until the combat says so. */
  private readonly revived = signal<ReadonlySet<string>>(new Set());
  /** The character whose "Reviver" question is open, whom it is saving, and what was refused. */
  protected readonly asking = signal<string | null>(null);
  protected readonly saving = signal(false);
  protected readonly refused = signal<{ characterId: string; living: LivingRefusal } | null>(null);
  protected readonly error = signal<{ id: string; message: string } | null>(null);
  protected readonly announcement = signal('');
  protected readonly switching = signal<string | null>(null);
  private readonly reviveKey = new ActionKey();

  protected readonly live = computed(() => this.encounter().status !== EncounterStatus.ENDED);

  protected readonly rows = computed<readonly DeathRow[]>(() => {
    const e = this.encounter();
    const gone = this.revived();
    return e.deaths
      .filter((d) => !(d.characterId && gone.has(d.characterId)))
      .map((d) => {
        const combatant = e.combatants.find((c) => c.id === d.combatantId);
        const blocked = this.blockedNow().get(d.combatantId) ?? d.revivifyBlocked ?? false;
        const since = Math.max(0, e.round - d.deathRound);
        const who = combatant ? this.whoLine(combatant) : '';
        const when = `morreu na rodada ${d.deathRound} (${since === 0 ? 'agora' : `há ${since}`})`;
        return {
          death: d,
          combatant,
          line: who ? joinDots([who, when]) : `Morreu${when.slice('morreu'.length)}`,
          fits: (d.fitsRevivify ?? e.round - d.deathRound <= WINDOW_ROUNDS) && !blocked,
          blocked,
          pronoun: article(d.name) === 'a' ? 'ela' : 'ele',
        };
      });
  });

  protected readonly subtitle = computed(
    () => `Rodada ${this.encounter().round} · um minuto são ${WINDOW_ROUNDS} rodadas`,
  );

  constructor() {
    // The combat now says what the master set: the local copy of the switch has done its job.
    effect(() => {
      const deaths = this.encounter().deaths;
      untracked(() => {
        const now = this.blockedNow();
        if (now.size === 0) {
          return;
        }
        const left = new Map(
          [...now].filter(([id, value]) => {
            const server = deaths.find((d) => d.combatantId === id);
            return server !== undefined && (server.revivifyBlocked ?? false) !== value;
          }),
        );
        if (left.size !== now.size) {
          this.blockedNow.set(left);
        }
      });
    });
  }

  /** "Guerreiro 4" for a player's character, "Inimigo" for an NPC. */
  private whoLine(c: Combatant): string {
    const info = this.info().get(c.characterId);
    return isPlayer(c)
      ? (info?.classSummary ?? '')
      : isCreature(c)
        ? ''
        : (info?.kindLabel ?? 'NPC');
  }

  protected initial(row: DeathRow): string {
    return combatantInitial(row.death.name);
  }

  protected npc(row: DeathRow): boolean {
    return !row.death.isPlayerCharacter;
  }

  protected canRevive(row: DeathRow): boolean {
    return this.live() && row.death.isPlayerCharacter && row.death.characterId !== '';
  }

  protected askToRevive(row: DeathRow): void {
    this.error.set(null);
    this.refused.set(null);
    this.asking.set(row.death.characterId);
  }

  protected cancelRevive(characterId: string): void {
    this.asking.set(null);
    this.refused.set(null);
    afterNextRender(
      () =>
        this.host.nativeElement
          .querySelector<HTMLElement>(`[data-revive="${characterId}"]`)
          ?.focus(),
      { injector: this.injector },
    );
  }

  protected async revive(row: DeathRow): Promise<void> {
    const characterId = row.death.characterId;
    this.saving.set(true);
    try {
      const key = this.reviveKey.keyFor({ campaignId: this.campaignId(), characterId });
      await this.api.revive(this.campaignId(), characterId, key);
      this.reviveKey.renew();
      this.asking.set(null);
      this.revived.update((set) => new Set(set).add(characterId));
      this.announcement.set(`${row.death.name} voltou à vida`);
    } catch (err) {
      this.asking.set(null);
      const living = livingRefusal(err);
      if (living) {
        this.refused.set({ characterId, living });
      } else {
        this.error.set({ id: characterId, message: describeCharacterError(err) });
      }
    } finally {
      this.saving.set(false);
    }
  }

  protected async setBlocked(row: DeathRow, blocked: boolean): Promise<void> {
    const id = row.death.combatantId;
    this.switching.set(id);
    this.error.set(null);
    this.blockedNow.update((m) => new Map(m).set(id, blocked));
    try {
      await this.api.setBlocked(this.campaignId(), id, blocked);
    } catch (err) {
      this.blockedNow.update((m) => {
        const next = new Map(m);
        next.delete(id);
        return next;
      });
      this.error.set({ id: row.death.characterId || id, message: describeConnectError(err, {}) });
    } finally {
      this.switching.set(null);
    }
  }
}
