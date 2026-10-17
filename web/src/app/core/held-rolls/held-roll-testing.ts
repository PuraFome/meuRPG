import { type MessageInitShape, create } from '@bufbuild/protobuf';
import { timestampFromMs } from '@bufbuild/protobuf/wkt';

import {
  type HeldRollOffer,
  HeldRollOfferSchema,
  RollAttackResponseSchema,
} from '../../../gen/meurpg/play/v1/combat_pb';
import {
  type AnswerHeldRollResponse,
  AnswerHeldRollResponseSchema,
} from '../../../gen/meurpg/play/v1/resources_pb';
import type { AttackResult } from '../combat/combat-client-types';
import { LUCKY_TRAIT } from '../combat/lucky';

/** What a spec may set on a question besides the d20 and what it asks. */
type HeldOfferOver = Partial<
  Pick<
    HeldRollOffer,
    | 'holdId'
    | 'rollerLabel'
    | 'createdAt'
    | 'combatantId'
    | 'characterId'
    | 'kind'
    | 'attackKey'
    | 'targetId'
  >
>;

/** The d20 of a held roll for specs: the faces, the bonus and the die that counts. */
function held20(
  faces: readonly number[],
  modifier: number,
  counted = 0,
  rerolled: readonly number[] = [],
) {
  return {
    diceCount: faces.length,
    diceSides: 20,
    faces: [...faces],
    countedIndex: counted,
    modifier,
    total: (faces[counted] ?? 0) + modifier,
    // The d20 Lucky rolled again: the 1 it replaced, then the new face.
    rerolled: rerolled.map((from) => ({
      index: counted,
      from,
      to: faces[counted] ?? 0,
      sourceKey: LUCKY_TRAIT,
    })),
  };
}

/** The halfling's question (Sortudo) for specs: a d20 that shows a 1 (both dice with advantage or disadvantage). */
export function heldLucky(
  faces: readonly number[] = [1],
  modifier = 5,
  over: HeldOfferOver = {},
): HeldRollOffer {
  return create(HeldRollOfferSchema, {
    holdId: 'h1',
    rollerLabel: 'Brisa',
    d20: held20(faces, modifier),
    question: { case: 'lucky', value: { luckyIndex: Math.max(0, faces.indexOf(1)) } },
    ...over,
  });
}

/** The Bardic Inspiration question for specs: a d20 of `face`, the die the roller holds and who gave it. When Lucky rolled
 * the d20 again, `rolledFrom` is the 1 it replaced. */
export function heldDie(
  face = 11,
  modifier = 5,
  sides = 8,
  over: HeldOfferOver = {},
  rolledFrom?: number,
): HeldRollOffer {
  return create(HeldRollOfferSchema, {
    holdId: 'h1',
    rollerLabel: 'Brisa',
    d20: held20([face], modifier, 0, rolledFrom === undefined ? [] : [rolledFrom]),
    question: { case: 'die', value: { sides, fromName: 'Orla' } },
    ...over,
  });
}

/** When a question was asked, for the master's wait. */
export function askedAt(ms: number): { createdAt: ReturnType<typeof timestampFromMs> } {
  return { createdAt: timestampFromMs(ms) };
}

/** What `AnswerHeldRoll` answers: the path's own response and, while the roll waits, the next question. */
export function heldAnswer(
  over: MessageInitShape<typeof AnswerHeldRollResponseSchema> = {},
): AnswerHeldRollResponse {
  return create(AnswerHeldRollResponseSchema, over);
}

/** What `AnswerHeldRoll` answers for a weapon attack: the attack as the path answers it, resolved, or still held with its
 * next question (copied out into `next`, as the server does). */
export function heldAttackAnswer(res: AttackResult): AnswerHeldRollResponse {
  return heldAnswer({
    result: {
      case: 'attack',
      value: create(RollAttackResponseSchema, {
        encounter: res.encounter,
        roll: res.roll,
        pendingDamage: res.pending,
        heldRollOffer: res.held,
      }),
    },
    next: res.held,
  });
}
