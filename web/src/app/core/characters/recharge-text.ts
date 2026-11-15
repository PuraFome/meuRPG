import {
  ItemRechargeKind,
  type ItemRecharge,
} from '../../../gen/meurpg/characters/v1/characters_pb';

/** The dice of a recharge as the boards draw them: "1d6+1" is "1d6 + 1" and "1d4-1" is "1d4 − 1" (the minus sign). */
export function diceText(dice: string): string {
  return dice.replace(/\s+/g, '').replace(/([+-])/g, (sign) => ` ${sign === '-' ? '−' : '+'} `);
}

/** "volta 1d6 + 1": the short line of the master's panel. Empty for an item with no recharge rules. */
export function rechargeShort(recharge: ItemRecharge | undefined): string {
  if (!recharge) {
    return '';
  }
  switch (recharge.kind) {
    case ItemRechargeKind.DAWN:
      return recharge.regainDice ? `volta ${diceText(recharge.regainDice)}` : 'volta tudo';
    case ItemRechargeKind.DAWNS_AFTER_EMPTY:
      return `volta tudo ${recharge.waitDawns} amanheceres depois da última carga`;
    default:
      return 'não volta';
  }
}
