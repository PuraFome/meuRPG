/** Roll a single d6 (1-6) using the provided random source. */
function rollDie(random: () => number): number {
  return Math.floor(random() * 6) + 1;
}

/**
 * Roll 4d6 and drop the lowest — the classic D&D / TTRPG attribute method.
 * Each call consumes 4 values from `random`.
 * Result range: 3-18.
 */
export function roll4d6DropLowest(random: () => number = Math.random): number {
  const rolls = [rollDie(random), rollDie(random), rollDie(random), rollDie(random)];
  rolls.sort((a, b) => a - b);
  // Drop the lowest (rolls[0]), sum the three highest
  return rolls[1] + rolls[2] + rolls[3];
}

/**
 * Generate a full set of 6 attribute scores using 4d6-drop-lowest.
 * Each call consumes 24 values from `random`.
 * Returns results sorted descending (highest first).
 */
export function rollAttributeSet(random: () => number = Math.random): number[] {
  const scores = Array.from({ length: 6 }, () => roll4d6DropLowest(random));
  scores.sort((a, b) => b - a);
  return scores;
}