import {
  ItemEmptyRule,
  ItemLoss,
  ItemRechargeKind,
  type ItemRecharge,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { articleOf } from './article-of';

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

/** A line under an item row: its icon and its sentence. */
export interface RechargeLine {
  readonly icon: string;
  readonly text: string;
}

const NEVER = 'Não volta: cada carga gasta se vai.';

const dawns = (n: number) => `${n} ${n === 1 ? 'amanhecer' : 'amanheceres'}`;

/** When the charges come back, as a clause that follows a subject: "volta 1d6 + 1 ao amanhecer", "volta tudo 7 amanheceres
 * depois da última carga". `''` for an item that never recharges. */
function comesBack(recharge: ItemRecharge): string {
  switch (recharge.kind) {
    case ItemRechargeKind.DAWN:
      return recharge.regainDice
        ? `volta ${diceText(recharge.regainDice)} ao amanhecer`
        : 'volta tudo ao amanhecer';
    case ItemRechargeKind.DAWNS_AFTER_EMPTY:
      return `volta tudo ${dawns(recharge.waitDawns)} depois da última carga`;
    default:
      return '';
  }
}

/** The line of an item that holds charges: "Volta 1d6 + 1 ao amanhecer", "Não volta: cada carga gasta se vai." `null` for an
 * item with no recharge rules (its row stays as it is). */
export function rechargeLine(recharge: ItemRecharge | undefined): RechargeLine | null {
  if (!recharge) {
    return null;
  }
  const when = comesBack(recharge);
  return when
    ? { icon: 'wb_twilight', text: `${when[0].toUpperCase()}${when.slice(1)}` }
    : { icon: 'info', text: NEVER };
}

/** The footnote of the use sheet: "Volta 1d6 + 1 ao amanhecer, até 7." (the dice never take it past the most it holds). */
export function rechargeWithCap(recharge: ItemRecharge, max: number): string {
  const line = rechargeLine(recharge)?.text ?? '';
  return recharge.kind === ItemRechargeKind.DAWN && recharge.regainDice
    ? `${line}, até ${max}.`
    : line.endsWith('.')
      ? line
      : `${line}.`;
}

/** "Sem cargas. A Varinha de segredos volta ao amanhecer.": the refusal of a use of an item with none left. */
export function noChargesWithRecharge(name: string, recharge: ItemRecharge): string {
  const subject = `${articleOf(name)} ${name}`;
  const when = recharge.kind === ItemRechargeKind.DAWN ? 'volta ao amanhecer' : comesBack(recharge);
  return `Sem cargas. ${subject} ${when || 'não volta'}.`;
}

/** "Sem cargas: volta 1d3 ao amanhecer.": the line of an item that has none left. */
export function emptyLine(recharge: ItemRecharge): string {
  const when = comesBack(recharge);
  return when ? `Sem cargas: ${when}.` : 'Sem cargas.';
}

/** "O mestre define as cargas iniciais (1d4 − 1).": a copy whose start is rolled and not set yet. */
export function startLine(recharge: ItemRecharge | undefined): string {
  return recharge?.startDice
    ? `O mestre define as cargas iniciais (${diceText(recharge.startDice)}).`
    : 'O mestre define as cargas iniciais.';
}

/** The noun an item name leads with, in lower case: "Bordão do poder" is "bordão". */
function nounOf(name: string): string {
  return (name.trim().split(/\s+/)[0] ?? '').toLowerCase();
}

/** "o" or "a" for the noun (the catalog carries no gender, so the ending decides). */
function genderOf(name: string): 'o' | 'a' {
  return articleOf(nounOf(name)) === 'A' ? 'a' : 'o';
}

/** What is left of an item that lost its magic, as the line under its row: "Virou um bordão comum.", "Ficou só o +2 no ataque e
 * no dano.", "A última carga se foi e o d20 deu 1." */
export function lostLine(
  name: string,
  recharge: ItemRecharge,
  state: 'destroyed' | 'plain' | 'partly',
): string {
  if (state === 'plain') {
    return `Virou ${genderOf(name) === 'a' ? 'uma' : 'um'} ${nounOf(name)} comum.`;
  }
  if (state === 'partly') {
    return 'Ficou só o +2 no ataque e no dano.';
  }
  return recharge.empty === ItemEmptyRule.D20
    ? 'A última carga se foi e o d20 deu 1.'
    : 'A última carga se foi.';
}

/** The words of the d20 dialog of the last charge: the sentence that asks, the part the board draws in bold (what a 1 does) and
 * the rest (what the other results do). */
export interface LastChargeBody {
  readonly intro: string;
  readonly strong: string;
  readonly rest: string;
}

export function lastChargeBody(name: string, recharge: ItemRecharge): LastChargeBody {
  const noun = nounOf(name);
  const he = genderOf(name) === 'a' ? 'ela' : 'ele';
  const the = genderOf(name);
  const intro = 'O item pede um d20 agora.';
  let strong = '';
  let loss = '';
  switch (recharge.onLoss) {
    case ItemLoss.GONE:
      strong = `Com 1, ${the} ${noun} vira cinzas`;
      loss = ' e fica na ficha como destruída.';
      break;
    case ItemLoss.PLAIN_ITEM:
      strong = `Com 1, ${the} ${noun} perde a magia`;
      loss = ` e vira ${the === 'a' ? 'uma' : 'um'} ${noun} comum.`;
      break;
    case ItemLoss.PARTLY_MAGIC:
      strong = `Com 1, ${the} ${noun} perde a magia`;
      loss = ' e fica só o +2 no ataque e no dano.';
      break;
  }
  const waits = comesBack(recharge) ? 'espera o amanhecer' : 'fica, sem cargas';
  const others = recharge.regainOnTwenty
    ? `Com 2 a 19, ${he} ${waits}. Com 20, devolve ${diceText(recharge.regainOnTwenty)} cargas.`
    : strong
      ? `Com 2 ou mais, ${he} ${waits}.`
      : `Com qualquer resultado, ${he} ${waits}.`;
  return { intro, strong, rest: strong ? `${loss} ${others}` : others };
}
