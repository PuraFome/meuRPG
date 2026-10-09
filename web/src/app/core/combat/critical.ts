import { CriticalDamageRule } from '../../../gen/meurpg/play/v1/combat_pb';
import { diceName, sumRange } from './combat-dice';

/**
 * The critical hint (RN-24, 10.4b): what to roll for the damage of a critical hit, said the way the table's
 * rule asks. The server works the numbers (`PendingDamage.dice_count` is already doubled under "dados dobrados",
 * and `critical_max` is the part that comes without rolling under "o máximo mais uma rolagem"); the browser only
 * says it, next to the dice, and does no dice math. Pure functions, tested without a DOM.
 */

/** What a critical hit's damage step says. */
export interface CriticalHint {
  /** The short rule: "role os dados duas vezes" or "o máximo mais uma rolagem". */
  readonly rule: string;
  /** The sentence for the step: "Acerto crítico: role os dados duas vezes (4d6 no total)." */
  readonly line: string;
  /** The fixed part under "o máximo mais uma rolagem", when the rule has one: "12 de máximo dos dados" */
  readonly fixed: string;
}

/** The hint for a damage, or `null` when the hit was not a critical one (or the rule is not told). `dice` is the
 * dice to roll as the server gave them (`dice_count`, `dice_sides`); `max` is `critical_max`. Shown only when the player rolls
 * physical dice: with the app's dice the server rolls them. */
export function criticalHint(
  rule: CriticalDamageRule,
  count: number,
  sides: number,
  max: number,
): CriticalHint | null {
  const dice = diceName(count, sides);
  switch (rule) {
    case CriticalDamageRule.DOUBLED_DICE:
      return {
        rule: 'role os dados duas vezes',
        line: `Acerto crítico: role os dados duas vezes (${dice} no total).`,
        fixed: '',
      };
    case CriticalDamageRule.MAX_PLUS_ROLL:
      return {
        rule: 'o máximo mais uma rolagem',
        line:
          max > 0
            ? `Acerto crítico: o máximo mais uma rolagem. O máximo dos dados (${max}) já vale sem rolar; role ${dice} uma vez.`
            : `Acerto crítico: o máximo mais uma rolagem. Role ${dice} uma vez.`,
        fixed: max > 0 ? `${max} de máximo dos dados` : '',
      };
    default:
      return null;
  }
}

/** The line under the typed field: for a critical hit under "o máximo mais uma rolagem" one clear instruction ("Role 1d8 e digite só o
 * que saiu"), since the maximum and the modifier are the app's to add; otherwise the sum of the dice. */
export function criticalTypedHint(
  rule: CriticalDamageRule,
  dice: string,
  min: number,
  max: number,
  fixed: number,
): string {
  if (rule === CriticalDamageRule.MAX_PLUS_ROLL && fixed > 0) {
    return `Role ${dice} e digite só o que saiu, de ${min} a ${max}. O app soma o resto.`;
  }
  return `Digite a soma dos dados, de ${min} a ${max}. O app soma o modificador.`;
}

/** The fixed parts the app adds to what is typed, said next to the field: "+ 8 do crítico + 3 de modificador". `max` is the server's
 * `critical_max`, `bonus` its modifier: the browser adds nothing it was not sent. Empty when there is no fixed critical part. */
export function fixedParts(max: number, bonus: number): string {
  if (max <= 0) {
    return '';
  }
  const mod = bonus === 0 ? '' : ` ${bonus < 0 ? '−' : '+'} ${Math.abs(bonus)} de modificador`;
  return `+ ${max} do crítico${mod}`;
}

// ---- the extra dice of a critical hit (Crítico Brutal, PM-03b) ----

/** What the critical damage's screens need of a pending damage: its own dice, the rule, and the feature's dice. */
export interface CriticalDice {
  /** The critical's own dice (already doubled under the SRD rule). */
  readonly diceCount: number;
  readonly diceSides: number;
  readonly bonus: number;
  readonly criticalRule: CriticalDamageRule;
  readonly criticalMax: number;
  /** The weapon dice the feature adds (1, 2 or 3); 0 without it. */
  readonly extraDiceCount: number;
  readonly extraDiceNamePt: string;
}

/** The barbarian level at which Crítico Brutal gives each number of extra dice (SRD 5.1, Bárbaro). */
const BRUTAL_LEVEL_BY_DICE: Readonly<Record<number, number>> = { 1: 9, 2: 13, 3: 17 };

const NUMBER_WORDS = [
  'zero',
  'um',
  'dois',
  'três',
  'quatro',
  'cinco',
  'seis',
  'sete',
  'oito',
  'nove',
  'dez',
] as const;

/** "um", "três", or the digits past ten. */
function numberWord(n: number): string {
  return NUMBER_WORDS[n] ?? String(n);
}

/** " + 3", " − 1" or nothing for 0. */
function plusText(n: number): string {
  return n === 0 ? '' : ` ${n < 0 ? '−' : '+'} ${Math.abs(n)}`;
}

/** Whether the damage carries the dice of a feature to the critical. */
export function hasExtraDice(p: CriticalDice): boolean {
  return p.extraDiceCount > 0;
}

/** How many dice the player rolls for the whole damage: the critical's and the feature's. */
export function diceToRoll(p: CriticalDice): number {
  return p.diceCount + p.extraDiceCount;
}

/** What a typed sum of those dice may be: one per die up to every die at its face. */
export function typedRange(p: CriticalDice): { min: number; max: number } {
  return sumRange(diceToRoll(p), p.diceSides);
}

/** "Dano do crítico" before the roll: the whole sum, "2d12 + 1d12 + 3" (with the maximum first when the critical came
 * with one: "12 + 1d12 + 1d12 + 3"). */
export function criticalSum(p: CriticalDice): string {
  const own = diceName(p.diceCount, p.diceSides);
  const extra = diceName(p.extraDiceCount, p.diceSides);
  const max = p.criticalMax > 0 ? `${p.criticalMax} + ` : '';
  return `${max}${own} + ${extra}${plusText(p.bonus)}`;
}

/** The sentence under the sum: which part is the critical and which the feature, with its level. */
export function criticalSentence(p: CriticalDice, damageTypePt: string): string {
  const own = diceName(p.diceCount, p.diceSides);
  const extra = diceName(p.extraDiceCount, p.diceSides);
  const level = BRUTAL_LEVEL_BY_DICE[p.extraDiceCount];
  const feature = `${extra} do ${p.extraDiceNamePt}${level === undefined ? '' : ` (nível ${level})`}`;
  const rule = p.criticalRule === CriticalDamageRule.DOUBLED_DICE ? ' (dados dobrados)' : '';
  const first =
    p.criticalMax > 0
      ? `O máximo do crítico (${p.criticalMax}) vale sem rolar; ${own} do crítico e ${feature}`
      : `${own} do crítico${rule} e ${feature}`;
  const bonus = p.bonus === 0 ? '' : `, mais ${p.bonus} de modificador`;
  return `${first}${bonus}${damageTypePt ? `, de ${damageTypePt}` : ''}.`;
}

/** The title of the typed roll: "Role 3d12 para o Machado grande (+3)". `weapon` is the name with its article ("o Machado grande"). */
export function brutalLabel(p: CriticalDice, weapon: string): string {
  return `Role ${diceName(diceToRoll(p), p.diceSides)} para ${weapon}${p.bonus === 0 ? '' : ` (${p.bonus < 0 ? '−' : '+'}${Math.abs(p.bonus)})`}`;
}

/** The line under that title: which dice are whose and what to type, with the range. */
export function brutalTypedHint(p: CriticalDice): string {
  const own = diceName(p.diceCount, p.diceSides);
  const extra = diceName(p.extraDiceCount, p.diceSides);
  const { min, max } = typedRange(p);
  const total = diceToRoll(p);
  if (p.criticalMax > 0) {
    return `O máximo do crítico (${p.criticalMax}) já está contado. Role ${own} do crítico e ${extra} do ${p.extraDiceNamePt}; digite a soma (${min} a ${max}).`;
  }
  return `${own} do crítico e ${extra} do ${p.extraDiceNamePt}. Role ${total === 2 ? 'os dois' : `os ${numberWord(total)}`} dados e digite a soma (${min} a ${max}).`;
}

/** The live total of a typed sum, with the groups: "22 (3d12) + 3 = 25", "12 (máximo) + 10 (2d12) + 3 = 25". */
export function brutalTyped(p: CriticalDice, sum: number): string {
  const max = p.criticalMax > 0 ? `${p.criticalMax} (máximo) + ` : '';
  const total = sum + p.criticalMax + p.bonus;
  return `${max}${sum} (${diceName(diceToRoll(p), p.diceSides)})${plusText(p.bonus)} = ${total}`;
}
