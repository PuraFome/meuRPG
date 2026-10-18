import { type FreeRollVm, naturalWord } from '../../../core/free-dice/free-roll';
import { type DieSides, type RollShow } from '../../../shared/roll-overlay/roll-animator';

/**
 * What the table sees when someone rolls with the free dice roller: "Brisa rolou", the dice, the total and
 * "Rolagem livre · 2d6 + 3" (the existing dice overlay draws it). A natural 20 or 1 says so in words under the dice.
 * The roller sees the result card in the sheet instead, so this is only for the others.
 */
export function freeRollShow(roll: FreeRollVm): RollShow | null {
  if (roll.dice.length === 0) {
    return null;
  }
  const natural = naturalWord(roll);
  return {
    label: `${roll.rollerName} rolou`,
    dice: roll.dice.map((d) => ({
      sides: roll.sides as DieSides,
      face: d.face,
      counts: !d.discarded,
    })),
    line: String(roll.total),
    note: `Rolagem livre · ${roll.formula}`,
    outcome: natural ? { word: natural, good: natural === '20 natural' } : undefined,
  };
}
