/**
 * The "Pontos de vida" box of the editor (E6-21): what the rolls typed so
 * far add up to. It is a preview, not the sheet: the server computes the
 * real maximum when the character is saved (`rules.Derive`), and this
 * follows the same arithmetic only so the player sees where the number
 * comes from while rolling. Constitution here is the final score, the base
 * plus what the race gives (the manual bonuses are added by the caller).
 */

export interface HitPointsLevel {
  readonly level: number;
  /** The roll, or `null` while the row is empty. */
  readonly roll: number | null;
  /** What the level adds (`roll + CON`, never less than 1), or `null` while empty. */
  readonly gain: number | null;
  /** The die of this level: the class's, which differs between the classes of a multiclass. */
  readonly die: number;
}

export interface HitPointsPreview {
  readonly constitutionModifier: number;
  /** The 1st level always takes the die's maximum. */
  readonly firstLevel: number;
  readonly levels: readonly HitPointsLevel[];
  /** The maximum so far: level 1 plus the rows that have a roll. */
  readonly total: number;
  /** Levels (2 and up) with no roll yet. */
  readonly missing: readonly number[];
  /** The range the final total can reach once the missing levels are rolled. */
  readonly min: number;
  readonly max: number;
}

/** The modifier of a score, rounded down: 16 gives +3, 9 gives -1. */
export function abilityModifier(score: number): number {
  return Math.floor((score - 10) / 2);
}

/** A roll that fits the die, or `null` (the field is empty, text or out of range). */
export function validRoll(value: number | null | undefined, hitDie: number): number | null {
  return typeof value === 'number' && Number.isInteger(value) && value >= 1 && value <= hitDie
    ? value
    : null;
}

export function hitPointsPreview(
  hitDie: number,
  level: number,
  constitution: number,
  rolls: readonly (number | null | undefined)[],
  /** The die of every level after the first, when the classes differ (index 0 = level 2). */
  levelDice: readonly number[] | null = null,
): HitPointsPreview {
  const mod = abilityModifier(constitution);
  const gainOf = (roll: number) => Math.max(roll + mod, 1);
  const firstLevel = Math.max(hitDie + mod, 1);

  const levels: HitPointsLevel[] = [];
  for (let l = 2; l <= level; l++) {
    const die = levelDice?.[l - 2] || hitDie;
    const roll = validRoll(rolls[l - 2], die);
    levels.push({ level: l, roll, gain: roll === null ? null : gainOf(roll), die });
  }
  const missing = levels.filter((row) => row.roll === null).map((row) => row.level);
  const total = levels.reduce((sum, row) => sum + (row.gain ?? 0), firstLevel);
  return {
    constitutionModifier: mod,
    firstLevel,
    levels,
    total,
    missing,
    min: total + missing.length * gainOf(1),
    max: levels.filter((row) => row.roll === null).reduce((sum, row) => sum + gainOf(row.die), total),
  };
}
