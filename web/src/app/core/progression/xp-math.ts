import { formatInt, tight } from '../format/text';

/** How a total of XP splits among some characters (question 46: down, as in
 * the book). The server does the real division; this is the live line the
 * master reads before giving, and the same numbers the server will answer. */
export interface Split {
  /** How many share. */
  readonly count: number;
  /** What each one gets. */
  readonly each: number;
  /** What does not split and is lost. */
  readonly lost: number;
}

export function splitXp(total: number, count: number): Split {
  if (!Number.isInteger(total) || total < 0 || count < 1) {
    return { count: Math.max(0, count), each: 0, lost: 0 };
  }
  const each = Math.floor(total / count);
  return { count, each, lost: total - each * count };
}

/** "116 XP para cada": the big line of the split. */
export function eachLine(split: Split): string {
  return tight(`${formatInt(split.each)} XP para cada`);
}

/** "350 XP ÷ 3 = 116,67, arredondado para baixo. 2 XP se perdem na divisão."
 * (the combat block, which has room to write the division out), or "Divisão
 * exata, nada se perde." when it closes. */
export function divisionSentence(total: number, split: Split): string {
  const sum = `${formatInt(total)} XP ÷ ${split.count}`;
  if (split.lost === 0) {
    return tight(`${sum} = ${formatInt(split.each)}. Divisão exata, nada se perde.`);
  }
  const exact = (total / split.count).toFixed(2).replace('.', ',');
  return tight(`${sum} = ${exact}, arredondado para baixo. ${lostWords(split.lost)}`);
}

/** The short form beside "50 XP para cada" in the dialog: "150 XP ÷ 3 = 50",
 * and for gold "120 PO = 120 XP ÷ 3 = 40"; what is lost is said after it. */
export function shortDivision(total: number, split: Split, gold = 0): string {
  const lead = gold > 0 ? `${formatInt(gold)} PO = ` : '';
  const sum = `${lead}${formatInt(total)} XP ÷ ${split.count} = ${formatInt(split.each)}`;
  return tight(split.lost > 0 ? `${sum}. ${lostWords(split.lost)}` : sum);
}

/** "2 XP se perdem na divisão." */
export function lostWords(lost: number): string {
  return lost === 1 ? '1 XP se perde na divisão.' : `${formatInt(lost)} XP se perdem na divisão.`;
}

/** "2 se perdem": the short form beside "116 XP para cada" when the footer is compact. */
export function lostShort(lost: number): string {
  return lost === 1 ? '1 se perde' : `${formatInt(lost)} se perdem`;
}

/** What a screen reader hears when the number changes: "175 XP para cada." */
export function splitAnnouncement(split: Split): string {
  return split.count === 0 ? 'Ninguém marcado.' : `${eachLine(split)}.`;
}

/** The limit `progression.proto` documents for the amount and the gold. */
export const XP_MAX = 1_000_000;

/** A whole number from 1 to 1.000.000 typed as text ("1.000" is not one: the
 * field takes digits only, as a number input does). `null` for anything
 * else, including an empty text. */
export function parseAmount(text: string): number | null {
  const clean = text.trim();
  if (!/^\d{1,7}$/.test(clean)) {
    return null;
  }
  const n = Number(clean);
  return n >= 1 && n <= XP_MAX ? n : null;
}
