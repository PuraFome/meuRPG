import { type WritableSignal } from '@angular/core';

import type { Encounter, SpellCast } from '../../../../../gen/meurpg/play/v1/combat_pb';
import type { AnswerHeldRollResponse } from '../../../../../gen/meurpg/play/v1/resources_pb';
import type { CombatState } from '../../../../core/combat/combat-state';
import { HeldRollController } from '../../../../core/held-rolls/held-roll-controller';
import { type HeldAnswer, HeldRollAsked } from '../../../../core/held-rolls/held-roll-offer';
import type { ResourceClient } from '../../../../core/resources/resources-client';
import { rerollShow } from '../../../../shared/held-roll-prompt/held-roll-shows';
import { showOfDice, type RollAnimator } from '../../../../shared/roll-overlay/roll-animator';

/** What `CastQuestions` reads from the sheet it asks for. */
export interface CastQuestionsContext {
  /** Read when the player answers: a sheet that never asks does not build the client. */
  readonly resources: () => Pick<ResourceClient, 'answerHeldRoll'>;
  readonly campaignId: string;
  readonly name: string;
  readonly state: Pick<CombatState, 'apply'>;
  readonly animator: Pick<RollAnimator, 'play'>;
  readonly busy: WritableSignal<boolean>;
  readonly error: WritableSignal<string>;
  readonly typing: WritableSignal<boolean>;
  /** The cast as the result step shows it. */
  readonly showCast: (encounter: Encounter, cast: SpellCast) => void;
}

/**
 * The questions a spell attack's d20 can be held for: the halfling's Lucky ("Tirou 1: rolar de novo?") first, then the
 * Bardic Inspiration die's. The d20 is rolled and kept on the server; the sheet shows the question (`held`), and the
 * answer brings the cast (or the next question on the new roll). Plain class the cast sheet builds.
 */
export class CastQuestions {
  /** The question that waits, Lucky's or the die's: nothing was spent yet. */
  readonly held: HeldRollController;

  constructor(private readonly host: CastQuestionsContext) {
    this.held = new HeldRollController({
      send: (holdId, answer, key) =>
        host.resources().answerHeldRoll(host.campaignId, holdId, answer, key),
      settle: (res, done) => this.answered(res, done.answer),
      busy: host.busy,
      error: host.error,
    });
  }

  /** A cast that answered a question instead of the cast (nothing was spent, the d20 is kept on the server): the sheet
   * shows it, Lucky's about a 1 first, else the Bardic Inspiration die's. False for any other error. */
  asks(err: unknown): boolean {
    if (!(err instanceof HeldRollAsked)) {
      return false;
    }
    this.host.state.apply(err.encounter);
    this.held.ask(err.offer);
    this.host.typing.set(false);
    // The d20 alone, no total and no word: the result is told after the answer.
    const shown = showOfDice(this.host.name, err.offer.d20);
    if (shown) {
      this.host.animator.play(shown);
    }
    return true;
  }

  /** What the answer brought: the spell cast now with the new d20 (or the 1), or, when the caster holds a Bardic
   * Inspiration die, the die's question next, on the new roll (the d20 the app rolled again tumbles). */
  private answered(res: AnswerHeldRollResponse, answer: HeldAnswer): void {
    const settled = res.result.case === 'spellCast' ? res.result.value : null;
    if (!settled?.encounter || (!settled.cast && !res.next)) {
      throw new Error('AnswerHeldRoll answered without its cast');
    }
    this.host.typing.set(false);
    if (res.next) {
      this.host.state.apply(settled.encounter);
      const show = answer.kind === 'lucky' ? rerollShow(res.next.d20, answer.roll) : null;
      if (show) {
        this.host.animator.play(show);
      }
      return;
    }
    this.host.showCast(settled.encounter, settled.cast!);
  }
}
