import type { WritableSignal } from '@angular/core';

import { DiceMode, DicePreference } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import type { Encounter } from '../../../../gen/meurpg/play/v1/combat_pb';
import type { ContestView } from '../../../../gen/meurpg/play/v1/contest_types_pb';
import { effectivePreference } from '../../../core/campaigns/dice-labels';
import type { ResourceClient } from '../../../core/resources/resources-client';
import { HeldRollController } from '../../../core/held-rolls/held-roll-controller';

/** What the sheet of a contest roll tells the question it asks. */
export interface ContestHeldOptions {
  readonly resources: () => ResourceClient;
  readonly campaignId: string;
  readonly busy: WritableSignal<boolean>;
  readonly error: WritableSignal<string>;
  /** Which side's roll waits: the initiator's (`contest`) or the defender's (`contestAnswer`). */
  readonly side: 'contest' | 'contestAnswer';
  /** The contest the answer settled, with the combat after it. */
  readonly settled: (encounter: Encounter, contest: ContestView) => void;
  /** The combat after an answer that asked the next question of the same roll. */
  readonly asked: (encounter: Encounter) => void;
}

/**
 * The question of a contest roll whose d20 the server kept (a halfling's 1, a Bardic Inspiration die the combatant holds):
 * the player answers before the contest opens (the initiator's) or settles (the defender's). The answer settles the contest,
 * or asks the next question of the same roll.
 */
export function contestHeld(o: ContestHeldOptions): HeldRollController {
  return new HeldRollController({
    send: (holdId, answer, key) => o.resources().answerHeldRoll(o.campaignId, holdId, answer, key),
    settle: (res) => {
      const done = res.result.case === o.side ? res.result.value : null;
      if (!done?.encounter || (!done.contest && !res.next)) {
        throw new Error('AnswerHeldRoll answered without its contest');
      }
      if (done.contest) {
        o.settled(done.encounter, done.contest);
      } else {
        o.asked(done.encounter);
      }
    },
    busy: o.busy,
    error: o.error,
  });
}

/** The ways the campaign lets the player answer a question: the app's roll, a typed face, and the one they prefer. */
export function waysOf(data: {
  readonly diceMode: DiceMode;
  readonly preference: DicePreference;
}): {
  readonly canApp: boolean;
  readonly canType: boolean;
  readonly preferApp: boolean;
} {
  return {
    canApp: data.diceMode !== DiceMode.PHYSICAL,
    canType: data.diceMode !== DiceMode.APP,
    preferApp: effectivePreference(data.diceMode, data.preference) === DicePreference.APP,
  };
}
