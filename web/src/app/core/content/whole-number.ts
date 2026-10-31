/** The largest and smallest whole number the server stores in a number field (int32). */
export const WHOLE_MAX = 2_147_483_647;
export const WHOLE_MIN = -2_147_483_648;

/** What a number field says when its text is no whole number the server can store. */
export const UNREADABLE_COUNT = 'Escreva um número inteiro, como 2.';
export const UNREADABLE_BONUS = 'Escreva um número inteiro, como 2 ou -1.';

/** The whole number a field's text holds: empty is 0 ("not set"), and a sign is read where `signed` allows one ("+1" is 1). Text that is
 * no whole number, or one outside the int32 range, is NaN: the mark an editor refuses to save, since a NaN goes out as `null`. */
export function readWhole(text: string, signed: boolean): number {
  const t = text.trim();
  if (t === '') {
    return 0;
  }
  if (!(signed ? /^[+-]?\d+$/ : /^\+?\d+$/).test(t)) {
    return Number.NaN;
  }
  const n = Number(t);
  // "-0" is 0, not the negative zero Number() reads
  return Number.isSafeInteger(n) && n >= WHOLE_MIN && n <= WHOLE_MAX ? n + 0 : Number.NaN;
}

/** The text a stored count shows: empty for 0 ("not set"). */
export function wholeText(n: number): string {
  return n !== 0 && !Number.isNaN(n) ? String(n) : '';
}

/** What a count field shows: the text the person typed while it still reads as the number the draft holds, else the number. */
export function shownWhole(n: number, typed: string | undefined, signed = false): string {
  return typed && Object.is(readWhole(typed, signed), n) ? typed : wholeText(n);
}

/** What the draft keeps of a typed text: the text itself when it is not how the number is written back ("+3", "007", text no number),
 * else nothing. It is never sent; a reloaded draft has none, so the field shows the stored number. */
export function keptTyped(text: string, n: number): string | undefined {
  return text === wholeText(n) ? undefined : text;
}
