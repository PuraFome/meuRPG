import type { AbilityResult } from '../dice/dice';
import type { AbilityKey } from './characters.types';
import { ABILITY_KEYS } from './characters.types';

/**
 * The ways a player makes the base scores of a new sheet (RN-24), as the screen needs them. The numbers (the array, the
 * costs, the budget, the ranges) are the server's, read from `GetTableRules`; this adds the costs it gives and shows the
 * dice it stored. The server checks every method again at `CreateCharacter`.
 */

/** The cost of a score in the server's table (`costs[0]` is the cost of `minScore`), or `null` outside it. */
export function costOf(score: number, costs: readonly number[], minScore: number): number | null {
  const at = score - minScore;
  return Number.isInteger(at) && at >= 0 && at < costs.length ? costs[at] : null;
}

/** What the six scores cost together, adding the server's costs; a score outside the table counts as costing nothing. */
export function pointsSpent(
  scores: Readonly<Record<AbilityKey, number>>,
  costs: readonly number[],
  minScore: number,
): number {
  return ABILITY_KEYS.reduce((sum, key) => sum + (costOf(scores[key], costs, minScore) ?? 0), 0);
}

/** Whether a score can go one up with the points left: the next score exists and its extra cost fits. */
export function canRaise(score: number, left: number, costs: readonly number[], minScore: number): boolean {
  const now = costOf(score, costs, minScore);
  const next = costOf(score + 1, costs, minScore);
  return now !== null && next !== null && next - now <= left;
}

/** Whether a score can go one down: it stays inside the server's table. */
export function canLower(score: number, costs: readonly number[], minScore: number): boolean {
  return costOf(score - 1, costs, minScore) !== null;
}

/** A stored set as a result the placing shows: the lowest die (the first of the lowest) is the one struck. */
export function resultOfSet(set: { readonly dice: readonly number[]; readonly total: number }): AbilityResult {
  const dropped = set.dice.length === 0 ? -1 : set.dice.indexOf(Math.min(...set.dice));
  return { total: set.total, dice: [...set.dice], dropped };
}

/** A value of the array as a result with no dice. */
export function resultOfValue(total: number): AbilityResult {
  return { total, dice: [], dropped: -1 };
}

/** Whether a typed value is a whole number inside the server's range. */
export function typedInRange(value: unknown, min: number, max: number): boolean {
  return typeof value === 'number' && Number.isInteger(value) && value >= min && value <= max;
}

/** The first missing die of the typed rolls, as words ("Falta o quarto dado da rolagem 6."), or `''` when all 24 are 1 to 6. */
export function missingDie(rows: readonly (readonly string[])[]): string {
  const ordinals = ['primeiro', 'segundo', 'terceiro', 'quarto'];
  for (let r = 0; r < rows.length; r++) {
    for (let d = 0; d < rows[r].length; d++) {
      const v = rows[r][d].trim();
      if (!/^[1-6]$/.test(v)) {
        return v === ''
          ? `Falta o ${ordinals[d]} dado da rolagem ${r + 1}.`
          : `O ${ordinals[d]} dado da rolagem ${r + 1} vai de 1 a 6.`;
      }
    }
  }
  return '';
}
