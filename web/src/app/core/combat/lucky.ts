import type { DiceRoll, LuckyOffer } from '../../../gen/meurpg/play/v1/combat_pb';

/**
 * The halfling's Lucky, "Sortudo" (SRD 5.1, Halfling): a 1 on the d20 of an attack roll may be rolled again, and the new
 * roll stands. The server holds the roll and asks (`RollAttackResponse.lucky_offer`); these are the words the attack
 * sheet and the combat log use for it.
 */

/** The content key of the trait: a d20 it rolled again names it in `DiceRoll.rerolled`. */
export const LUCKY_TRAIT = 'trait:lucky';

/** The question's title. */
export const LUCKY_TITLE = 'Tirou 1: rolar de novo?';

/** The line under the title. With advantage or disadvantage only the die that showed the 1 is rolled again (SRD 5.1,
 * "Advantage and Disadvantage"), so the line names that die. */
export function luckyWhat(offer: LuckyOffer | null | undefined): string {
  const pair = (offer?.d20?.faces.length ?? 0) > 1;
  return pair
    ? 'Sortudo: você pode rolar de novo o dado que deu 1 e fica com o novo resultado.'
    : 'Sortudo: você pode rolar esse d20 de novo e fica com o novo resultado.';
}

/** "Sortudo: 1 → 14", beside the d20 Lucky rolled again; `''` when it rolled nothing (no 1, or the 1 kept). */
export function luckyText(roll: DiceRoll | undefined): string {
  return (roll?.rerolled ?? [])
    .filter((r) => r.sourceKey === LUCKY_TRAIT)
    .map((r) => `Sortudo: ${r.from} → ${r.to}`)
    .join(', ');
}
