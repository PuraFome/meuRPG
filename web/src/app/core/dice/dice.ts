import { InjectionToken } from '@angular/core';

import { ABILITY_KEYS, AbilityKey } from '../characters/characters.types';

/**
 * The dice of character creation (MR-004): the six ability scores rolled as
 * 4d6 drop lowest, the standard array and the hit-point dice. They are
 * rolled here, in the browser, and never recorded (RN-18, question 40):
 * what the player keeps is the number they place on the sheet, and the
 * server only ever sees that.
 *
 * Pure on purpose: every function takes the random source, so the tests
 * can feed it known dice.
 */

/** One fair die: a whole number from 1 to `sides`. */
export type RollDie = (sides: number) => number;

/**
 * The browser's own cryptographic source. `Math.random` would do for a
 * game, but `getRandomValues` is just as easy and is what a table that
 * argues about luck would want. The `% sides` of a naive version favours
 * the low faces (2^32 is not a multiple of 6); throwing away the top
 * sliver of the range keeps every face equally likely.
 */
export const rollDie: RollDie = (sides) => {
  const limit = 0x1_0000_0000 - (0x1_0000_0000 % sides);
  const buffer = new Uint32Array(1);
  do {
    crypto.getRandomValues(buffer);
  } while (buffer[0] >= limit);
  return (buffer[0] % sides) + 1;
};

/** One rolled ability score: four d6, the lowest of them not counted. */
export interface AbilityResult {
  /** The score: the three highest dice added up (or the array value). */
  readonly total: number;
  /** The four dice as they fell; empty for a standard-array value. */
  readonly dice: readonly number[];
  /** Index in `dice` of the die that does not count, or -1 without dice. */
  readonly dropped: number;
}

/** The die the components roll with: the real one, unless a test swaps it. */
export const ROLL_DIE = new InjectionToken<RollDie>('ROLL_DIE', {
  providedIn: 'root',
  factory: () => rollDie,
});

export const STANDARD_ARRAY: readonly number[] = [15, 14, 13, 12, 10, 8];

/** 4d6, lowest die dropped. On a tie the first of the lowest is the one struck. */
export function rollAbilityScore(roll: RollDie): AbilityResult {
  const dice = [roll(6), roll(6), roll(6), roll(6)];
  const dropped = dice.indexOf(Math.min(...dice));
  const total = dice.reduce((sum, die) => sum + die, 0) - dice[dropped];
  return { total, dice, dropped };
}

/** Six scores, best first (that is how the screen lists them, to make placing easier). */
export function rollAbilityScores(roll: RollDie): AbilityResult[] {
  return Array.from({ length: ABILITY_KEYS.length }, () => rollAbilityScore(roll)).sort(
    (a, b) => b.total - a.total,
  );
}

/** The standard array as results with no dice. */
export function standardArrayResults(): AbilityResult[] {
  return STANDARD_ARRAY.map((total) => ({ total, dice: [], dropped: -1 }));
}

/** Where each result went: the index of a result in the list, or `null` for a free ability. */
export type Placement = Record<AbilityKey, number | null>;

export function emptyPlacement(): Placement {
  return { str: null, dex: null, con: null, int: null, wis: null, cha: null };
}

/** The ability that holds result `index`, or `null` while it is free. */
export function holderOf(placement: Placement, index: number): AbilityKey | null {
  return ABILITY_KEYS.find((key) => placement[key] === index) ?? null;
}

/**
 * Puts result `index` on `ability`. A result sits on one ability only: if
 * another ability had it, the two swap (that one gets whatever `ability`
 * held, which may be nothing).
 */
export function place(placement: Placement, index: number, ability: AbilityKey): Placement {
  const next = { ...placement };
  const holder = holderOf(placement, index);
  if (holder !== null) {
    next[holder] = placement[ability];
  }
  next[ability] = index;
  return next;
}

/** How many abilities still wait for a result. */
export function freeCount(placement: Placement): number {
  return ABILITY_KEYS.filter((key) => placement[key] === null).length;
}
