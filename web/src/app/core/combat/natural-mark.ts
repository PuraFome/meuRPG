import type { Combatant, DiceRoll } from '../../../gen/meurpg/play/v1/combat_pb';

/**
 * The "20 natural" / "1 natural" tag. A natural 20 or 1 has no effect on an ability check, a saving throw, a contest or initiative
 * (SRD 5.1: only attack rolls and death saves give the natural die a mechanical effect), so the tag is information for the table and
 * never says the result changed. It lives apart from `contest-view` so `combat-log` can use it without an import cycle.
 */

/** The faces of a d20 that carry a mark. */
const NATURAL_MAX = 20;
const NATURAL_MIN = 1;

export type NaturalMark = '' | '20 natural' | '1 natural';

/** The mark of a d20 face (`0` or any other number has none). */
export function naturalMark(face: number): NaturalMark {
  if (face === NATURAL_MAX) {
    return '20 natural';
  }
  return face === NATURAL_MIN ? '1 natural' : '';
}

/** The mark of a `DiceRoll` of one d20 (a scene roll): the face at `countedIndex` (the higher with advantage, the lower with
 * disadvantage); none for any other dice. */
export function diceNaturalMark(
  roll: Pick<DiceRoll, 'diceSides' | 'faces' | 'countedIndex'> | undefined,
): NaturalMark {
  if (!roll || roll.diceSides !== NATURAL_MAX) {
    return '';
  }
  return naturalMark(roll.faces[roll.countedIndex] ?? roll.faces[0] ?? 0);
}

/** The mark of the d20 of a combatant's initiative; empty until it is rolled. */
export function initiativeMark(c: Pick<Combatant, 'initiativeFace'>): NaturalMark {
  return c.initiativeFace === undefined ? '' : naturalMark(c.initiativeFace);
}
