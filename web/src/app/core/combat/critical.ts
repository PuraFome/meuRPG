import { CriticalDamageRule } from '../../../gen/meurpg/play/v1/combat_pb';
import { diceName } from './combat-dice';

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
export function criticalHint(rule: CriticalDamageRule, count: number, sides: number, max: number): CriticalHint | null {
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
export function criticalTypedHint(rule: CriticalDamageRule, dice: string, min: number, max: number, fixed: number): string {
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
