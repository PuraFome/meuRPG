import { signal } from '@angular/core';
import { DeathSaveOutcome, type Encounter } from '../../../../gen/meurpg/play/v1/combat_pb';
import type { CombatClient } from '../../../core/combat/combat-client';
import type { AttackDie } from '../../../core/combat/combat-client-types';
import type { CombatCalls } from '../../../core/combat/combat-calls';
import { outcomeText, saveAnnouncement } from '../../../core/combat/death-saves';
import { ActionKey } from '../../../core/connect/idempotency';
import { type RollAnimator, showOfDice } from '../../../shared/roll-overlay/roll-animator';
import { type DeathSaveRoll } from './death-save-due/death-save-due';

/** What `CombatDeaths` reads from the screen it serves. */
export interface CombatDeathsContext {
  readonly api: Pick<CombatClient, 'combatants'>;
  readonly animator: Pick<RollAnimator, 'play'>;
  readonly calls: Pick<CombatCalls, 'run'>;
  readonly campaignId: () => string;
  readonly encounter: () => Encounter | null;
  readonly ownId: () => string | undefined;
  readonly ownLabel: () => string | undefined;
  /** The line the page keeps under the actions ("Toren volta com 1 PV."). */
  readonly note: (text: string) => void;
}

/**
 * The death saves of the screen: the player's own on their turn, the master's for a player who is away, the master's
 * confirmation of a death and the question when he passes the turn with a save still due. Each sentence is kept until the
 * turn changes (the screen clears them).
 */
export class CombatDeaths {
  /** The death save just rolled this turn. */
  readonly result = signal('');
  /** The sentence of the death save the master rolled for an absent player, kept after the save is no longer due. */
  readonly masterResult = signal('');
  /** The characters at three failures the master put away with "Ainda não". */
  readonly later = signal<ReadonlySet<string>>(new Set());
  /** The master tapped "Próximo turno" while a death save was due: the question waits for "Rolar agora" or "Passar sem rolar". */
  readonly asking = signal<{ readonly discard: boolean } | null>(null);
  private readonly saveKey = new ActionKey();
  private readonly confirmKey = new ActionKey();

  constructor(private readonly ctx: CombatDeathsContext) {}

  /** The death save of the player's own turn (RN-03, RN-18 per roll). */
  async roll(die: AttackDie): Promise<void> {
    const id = this.ctx.ownId();
    if (!id) {
      return;
    }
    const text = await this.rollOf(id, this.ctx.ownLabel() ?? '', die);
    if (text) {
      this.result.set(text);
    }
  }

  /** The master rolls the death save of a player who is not there (the server accepts the master, RN-03). */
  async rollMaster(roll: DeathSaveRoll): Promise<void> {
    const who = this.ctx.encounter()?.combatants.find((c) => c.id === roll.id);
    if (!who) {
      return;
    }
    this.asking.set(null);
    const text = await this.rollOf(who.id, who.label, roll.die);
    if (text) {
      this.masterResult.set(text);
    }
  }

  /** One death save call, with the dice animation; the sentence it announced, or '' when the call failed. */
  async rollOf(id: string, label: string, die: AttackDie): Promise<string> {
    let text = '';
    await this.ctx.calls.run(async (e) => {
      const key = this.saveKey.keyFor([e.id, id, die]);
      const res = await this.ctx.api.combatants.rollDeathSave(
        this.ctx.campaignId(),
        e.id,
        id,
        die,
        key,
      );
      this.saveKey.renew();
      text = saveAnnouncement(res.save, label);
      // The death save, as the line under the sheet tells it (hidden saves arrive without a roll: nothing plays).
      const show = showOfDice('Teste contra a morte', res.save.roll, {
        withTotal: true,
        outcome: {
          word: outcomeText(res.save.outcome),
          good:
            res.save.outcome === DeathSaveOutcome.SUCCESS ||
            res.save.outcome === DeathSaveOutcome.REVIVED,
        },
        critical: res.save.outcome === DeathSaveOutcome.REVIVED,
        fumble: res.save.outcome === DeathSaveOutcome.CRITICAL_FAILURE,
      });
      if (show) {
        this.ctx.animator.play(show);
      }
      if (res.save.outcome === DeathSaveOutcome.REVIVED && id === this.ctx.ownId()) {
        this.ctx.note(text);
      }
      return res.encounter;
    });
    return text;
  }

  /** The master's "Confirmar a morte" (ConfirmDeath): the character is dead for good. */
  async confirm(id: string): Promise<void> {
    await this.ctx.calls.run(async (e) => {
      const res = await this.ctx.api.combatants.confirmDeath(
        this.ctx.campaignId(),
        e.id,
        id,
        this.confirmKey.keyFor([e.id, id]),
      );
      this.confirmKey.renew();
      return res;
    });
  }

  /** "Ainda não": the question is put away while the character stays dying. */
  postpone(id: string): void {
    this.later.update((s) => new Set(s).add(id));
  }

  /** The question comes back for that character. */
  askAgain(id: string): void {
    this.later.update((s) => {
      const next = new Set(s);
      next.delete(id);
      return next;
    });
  }
}
