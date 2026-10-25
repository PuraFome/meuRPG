import { signal } from '@angular/core';
import {
  type DeathSave,
  DeathSaveOutcome,
  type Encounter,
  type HeldRollOffer,
} from '../../../../gen/meurpg/play/v1/combat_pb';
import type { AnswerHeldRollResponse } from '../../../../gen/meurpg/play/v1/resources_pb';
import type { CombatClient } from '../../../core/combat/combat-client';
import type { AttackDie } from '../../../core/combat/combat-client-types';
import type { CombatCalls } from '../../../core/combat/combat-calls';
import { outcomeText, saveAnnouncement } from '../../../core/combat/death-saves';
import { ActionKey } from '../../../core/connect/idempotency';
import { HeldRollController } from '../../../core/held-rolls/held-roll-controller';
import type { HeldAnswer } from '../../../core/held-rolls/held-roll-offer';
import { type RollAnimator, showOfDice } from '../../../shared/roll-overlay/roll-animator';
import { type DeathSaveRoll } from './death-save-due/death-save-due';

/** What `CombatDeaths` reads from the screen it serves. */
export interface CombatDeathsContext {
  readonly api: Pick<CombatClient, 'combatants'>;
  /** Answers the question of a held roll (`ResourceClient.answerHeldRoll`). */
  readonly answerHeld: (
    holdId: string,
    answer: HeldAnswer,
    idempotencyKey: string,
  ) => Promise<AnswerHeldRollResponse>;
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
  /** The question of the player's own death save when the d20 was kept for one: the halfling's Lucky about a 1 ("Tirou 1: rolar de
   * novo?"), then the Bardic Inspiration die; nothing is written, and the counts do not move, until it is answered. */
  readonly held: HeldRollController;
  private readonly saveKey = new ActionKey();
  private readonly confirmKey = new ActionKey();

  constructor(private readonly ctx: CombatDeathsContext) {
    this.held = new HeldRollController({
      send: (holdId, answer, key) => ctx.answerHeld(holdId, answer, key),
      settle: (res) => this.answered(res),
    });
  }

  /** Shows the question the combat says waits for the player's own death save (a reload, or another screen asked it). */
  resume(offer: HeldRollOffer | undefined): void {
    if (offer?.holdId !== this.held.offer()?.holdId) {
      this.held.ask(offer);
    }
  }

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
      if (!res.save) {
        // The d20 is kept for a question: the player's own screen asks it; the master's for an absent player waits for the
        // player (the session panel lists it, and he may answer after ten seconds).
        if (id === this.ctx.ownId()) {
          this.held.ask(res.held);
        } else if (res.held) {
          text = `O d20 do teste contra a morte de ${label} espera uma resposta.`;
        }
        return res.encounter;
      }
      text = this.announce(res.save, id, label);
      return res.encounter;
    });
    return text;
  }

  /** The death save as the line under the sheet tells it: the sentence, the dice and, for a return, the note. */
  private announce(save: DeathSave, id: string, label: string): string {
    const text = saveAnnouncement(save, label);
    // Hidden saves arrive without a roll: nothing plays.
    const show = showOfDice('Teste contra a morte', save.roll, {
      withTotal: true,
      outcome: {
        word: outcomeText(save.outcome),
        good:
          save.outcome === DeathSaveOutcome.SUCCESS || save.outcome === DeathSaveOutcome.REVIVED,
      },
      critical: save.outcome === DeathSaveOutcome.REVIVED,
      fumble: save.outcome === DeathSaveOutcome.CRITICAL_FAILURE,
    });
    if (show) {
      this.ctx.animator.play(show);
    }
    // A natural 20, or a hook that stood the character up: the card of the dying is gone, so the line goes to the note.
    if (
      (save.outcome === DeathSaveOutcome.REVIVED || save.hook?.healed) &&
      id === this.ctx.ownId()
    ) {
      this.ctx.note(text);
    }
    return text;
  }

  /** The answer to the question settled the death save: the sentence and the combat after it. */
  private async answered(res: AnswerHeldRollResponse): Promise<void> {
    if (res.result.case !== 'deathSave') {
      return;
    }
    const done = res.result.value;
    const save = done.deathSave;
    const label = this.ctx.ownLabel() ?? '';
    if (save) {
      this.result.set(this.announce(save, this.ctx.ownId() ?? '', label));
    }
    const encounter = done.encounter;
    if (encounter) {
      await this.ctx.calls.run(() => Promise.resolve(encounter));
    }
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
