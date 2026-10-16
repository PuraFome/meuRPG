import type { Encounter, LuckyOffer } from '../../../gen/meurpg/play/v1/combat_pb';
import type { Reroll } from '../../../gen/meurpg/play/v1/combat_rolls_pb';
import { rollText } from './combat-dice';
import { LUCKY_TRAIT, type WithRerolls } from './lucky';

/**
 * The halfling's Lucky on the d20 rolls of the combat that are not a weapon attack (a spell attack against one target,
 * an effect's saving throw, a concentration save): the server keeps the d20 that shows a 1 and the roll's response
 * carries the question (`lucky_offer`) instead of its result; `ResourceService.AnswerOutsideLucky` answers it. These are
 * the pieces the cast sheet, the effect-save sheet and the reaction sheet share.
 */

/**
 * What a cast answers instead of a cast when the caster is the player's own halfling and the spell attack's d20 shows a
 * 1 (SRD 5.1, Lucky: the die may be rolled again, and the new roll stands): nothing was spent, the d20 is kept on the
 * server and the offer is the question. The cast sheet catches it, as it catches `InspirationQuestion`; the answer
 * brings the cast, or the Bardic Inspiration question on the new roll.
 */
export class LuckyQuestion extends Error {
  constructor(
    readonly offer: LuckyOffer,
    readonly encounter: Encounter,
  ) {
    super('A cast waits for the answer to the Lucky question');
    this.name = 'LuckyQuestion';
  }
}

/** "Seu d20: 1d20 (1) + 5 = 6. O resultado só aparece depois da sua escolha.": the held d20 above the question, on a sheet
 * that has no result to show it in yet. `''` without an offer. */
export function luckyHeldLine(offer: LuckyOffer | null | undefined): string {
  return offer?.d20
    ? `Seu d20: ${rollText(offer.d20)}. O resultado só aparece depois da sua escolha.`
    : '';
}

/** The d20 Lucky rolled again on a roll (the faces already hold the new one), for the die that tumbles after an answer
 * rolled in the app; `undefined` when it rolled nothing. */
export function luckyReroll(roll: WithRerolls | undefined): Reroll | undefined {
  return roll?.rerolled.find((r) => r.sourceKey === LUCKY_TRAIT);
}
