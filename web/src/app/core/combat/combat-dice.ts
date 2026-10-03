import type { DiceRoll } from '../../../gen/meurpg/play/v1/combat_pb';

/**
 * How the combat screens write dice (timeline.md, shared decision 3): the
 * dice, the faces in parentheses, the modifier and the total
 * (`1d20 (13) + 6 = 19`); a typed physical roll without the dice
 * (`16 + 5 = 21`, with the note "dado físico" next to it). Pure functions,
 * tested without a DOM.
 */

/** " + 6", " − 1" or nothing for 0. */
function modifierText(modifier: number): string {
  if (modifier === 0) {
    return '';
  }
  return ` ${modifier < 0 ? '−' : '+'} ${Math.abs(modifier)}`;
}

/** A roll as the formula the table reads, the total included. */
export function rollFormula(roll: DiceRoll): string {
  const mod = modifierText(roll.modifier);
  if (roll.diceCount === 0) {
    return `${roll.total}`;
  }
  if (roll.physical || roll.faces.length === 0) {
    // The player typed what the dice showed, not each die.
    return `${roll.total - roll.modifier}${mod} = ${roll.total}`;
  }
  return `${roll.diceCount}d${roll.diceSides} (${roll.faces.join(', ')})${mod} = ${roll.total}`;
}

/** A roll for a sentence: the app's `1d20 (14) = 14`, a typed one `16 + 5 = 21
 * · dado físico` (or just `1 · dado físico` for a bare d20: `1 = 1` reads as a typo). */
export function rollText(roll: DiceRoll): string {
  if (!roll.physical) {
    return rollFormula(roll);
  }
  return roll.modifier === 0 ? `${roll.total} · dado físico` : `${rollFormula(roll)} · dado físico`;
}

/** The typed d20 and its bonus, before it is sent: `16 + 5 = 21`. */
export function typedTotal(face: number, bonus: number): string {
  return `${face}${modifierText(bonus)} = ${face + bonus}`;
}

/** "7 de fogo", "5 de dano perfurante": the physical types are adjectives. */
export function damageWords(amount: number, typePt: string): string {
  if (!typePt) {
    return `${amount} de dano`;
  }
  const adjective = /^(cortante|perfurante|contundente)$/.test(typePt);
  return `${amount} de ${adjective ? `dano ${typePt}` : typePt}`;
}

/** The damage line: `1d10 (7) = 7 de fogo`, `2d6 (5, 4) + 2 = 11 de dano`. */
export function damageFormula(roll: DiceRoll, typePt: string): string {
  const words = damageWords(roll.total, typePt);
  const shown = rollFormula(roll).replace(/ = \d+$/, '');
  // A flat number, or a typed sum with no bonus, has no formula to show.
  return roll.diceCount === 0 || shown === `${roll.total}` ? words : `${shown} = ${words}`;
}

/** The dice a damage is made of, as the field asks for it: "2d6". */
export function diceName(count: number, sides: number): string {
  return `${count}d${sides}`;
}

/** What a typed sum of physical dice may be (Q38): from one per die to every
 * die at its face. */
export function sumRange(count: number, sides: number): { min: number; max: number } {
  return { min: count, max: count * sides };
}

/** The face typed for a physical d20: a whole number from 1 to 20, or `null`. */
export function parseFace(text: string): number | null {
  return parseWhole(text, 1, 20);
}

/** A typed sum inside its range, or `null`. */
export function parseSum(text: string, min: number, max: number): number | null {
  return parseWhole(text, min, max);
}

function parseWhole(text: string, min: number, max: number): number | null {
  const value = text.trim();
  if (!/^\d{1,4}$/.test(value)) {
    return null;
  }
  const n = Number(value);
  return n >= min && n <= max ? n : null;
}
