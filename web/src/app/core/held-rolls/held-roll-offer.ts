import { create } from '@bufbuild/protobuf';

import {
  type DiceRoll,
  type Encounter,
  type HeldRollOffer,
  HeldRollOfferSchema,
  HeldRollKind,
  type InspirationOffer,
  type LuckyOffer,
  type OutsideInspirationOffer,
} from '../../../gen/meurpg/play/v1/combat_pb';
import type { Reroll } from '../../../gen/meurpg/play/v1/combat_rolls_pb';
import { rollText } from '../combat/combat-dice';
import { LUCKY_TRAIT, type WithRerolls } from '../combat/lucky';

/**
 * The question a roll that waits asks, in the one shape every held roll has (`HeldRollOffer`): the halfling's Lucky
 * ("Tirou 1: rolar de novo?") first, then the Bardic Inspiration die. The server sends it as `held_roll_offer` beside the
 * older `lucky_offer` and `inspiration_offer`; `heldOfferOf` reads the new one and falls back on the old ones, so a
 * response that carries only those still asks.
 */

/** How the new d20 of Lucky, or the die of the Bardic Inspiration, comes (RN-18): the app rolls it, or the player typed
 * the face of a real die. */
export type HeldRolled = { readonly inApp: true } | { readonly face: number };

/** The player's answer to the question that waits: Lucky's (roll the d20 that showed the 1 again, or keep it) or the
 * die's (add it, or keep it for later). `roll` is only for `reroll` and `use`. */
export type HeldAnswer =
  | { readonly kind: 'lucky'; readonly reroll: boolean; readonly roll: HeldRolled | null }
  | { readonly kind: 'die'; readonly use: boolean; readonly roll: HeldRolled | null };

/** The ways a response carries a question: the new offer, or the two older ones. */
export interface HeldOfferSource {
  readonly heldRollOffer?: HeldRollOffer;
  readonly luckyOffer?: LuckyOffer;
  readonly inspirationOffer?: InspirationOffer | OutsideInspirationOffer;
}

/** What a roll that answered a question instead of its result throws, for the clients whose call returns the result
 * alone (a cast): nothing was spent, the d20 is kept on the server, and the sheet asks. */
export class HeldRollAsked extends Error {
  constructor(
    readonly offer: HeldRollOffer,
    readonly encounter: Encounter,
  ) {
    super('The roll waits for the answer to its question');
    this.name = 'HeldRollAsked';
  }
}

/** The question a response asks now: the new offer, else the old Lucky one (it comes first), else the old die one;
 * `undefined` when the roll does not wait. */
export function heldOfferOf(source: HeldOfferSource): HeldRollOffer | undefined {
  if (source.heldRollOffer) {
    return source.heldRollOffer;
  }
  const lucky = source.luckyOffer;
  if (lucky) {
    return create(HeldRollOfferSchema, {
      holdId: lucky.holdId,
      d20: lucky.d20,
      question: { case: 'lucky', value: { luckyIndex: lucky.luckyIndex } },
      ...attackOf(lucky),
    });
  }
  const die = source.inspirationOffer;
  if (die) {
    const combat = die.$typeName === 'meurpg.play.v1.InspirationOffer';
    return create(HeldRollOfferSchema, {
      holdId: die.holdId,
      d20: die.d20,
      question: {
        case: 'die',
        value: combat
          ? { sides: die.die?.sides ?? 0, fromName: die.die?.fromLabel ?? '' }
          : { sides: die.sides, fromName: die.fromName },
      },
      ...(combat ? attackOf(die) : {}),
    });
  }
  return undefined;
}

/** The weapon attack an old offer names (empty for a roll that is not one). */
function attackOf(offer: { readonly attackKey: string; readonly targetId: string }) {
  return offer.attackKey
    ? { kind: HeldRollKind.WEAPON_ATTACK, attackKey: offer.attackKey, targetId: offer.targetId }
    : {};
}

/** Whether the question is the halfling's Lucky (otherwise it is the die's). */
export function asksLucky(offer: HeldRollOffer | null | undefined): boolean {
  return offer?.question.case === 'lucky';
}

/** The size and giver of the die the question offers, or `undefined` when it is Lucky's. */
export function dieOf(
  offer: HeldRollOffer | null | undefined,
): { readonly sides: number; readonly fromName: string } | undefined {
  return offer?.question.case === 'die' ? offer.question.value : undefined;
}

/** "Seu d20: 1d20 (1) + 5 = 6. O resultado só aparece depois da sua escolha.": the held d20 above the question, on a
 * sheet that has no result to show it in yet. `''` without a d20. */
export function luckyHeldLine(offer: { readonly d20?: DiceRoll } | null | undefined): string {
  return offer?.d20
    ? `Seu d20: ${rollText(offer.d20)}. O resultado só aparece depois da sua escolha.`
    : '';
}

/** The d20 Lucky rolled again on a roll (the faces already hold the new one), for the die that tumbles after an answer
 * rolled in the app; `undefined` when it rolled nothing. */
export function luckyReroll(roll: WithRerolls | undefined): Reroll | undefined {
  return roll?.rerolled.find((r) => r.sourceKey === LUCKY_TRAIT);
}
