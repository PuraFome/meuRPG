/**
 * The few text rules the forms check before they ask the server (MR-038, RN-27). The server stays the authority: it works out
 * every one of these again and its refusal wins. These copies only tell the master what is wrong before the call, in the field.
 */

const ACCENTS: Readonly<Record<string, string>> = { ß: 'ss', æ: 'ae', œ: 'oe', ø: 'o', ı: 'i' };

/**
 * The form two texts are compared in: lower case, no accents, only letters and digits, each run of anything else counting as
 * one space ("  A Sombra! " and "a sombra" are the same). The same rule as the server's `Fold`.
 */
export function fold(text: string): string {
  const plain = text
    .toLowerCase()
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .replace(/[ßæœøı]/g, (c) => ACCENTS[c] ?? c);
  let out = '';
  let space = true;
  for (const ch of plain) {
    if (/[\p{L}\p{N}]/u.test(ch)) {
      out += ch;
      space = false;
    } else if (!space) {
      out += ' ';
      space = true;
    }
  }
  return out.trimEnd();
}

/** Whether a message has a letter A to Z once folded: a cipher has nothing to hide otherwise. */
export function hasCipherLetter(message: string): boolean {
  return /[a-z]/.test(fold(message));
}

/** The distinct letters of a keyword, in order, once folded. */
export function keywordLetters(keyword: string): string[] {
  const seen = new Set<string>();
  for (const ch of fold(keyword)) {
    if (ch >= 'a' && ch <= 'z') {
      seen.add(ch);
    }
  }
  return [...seen];
}

/** A keyword whose letters are the alphabet's own first ones ("ABC") swaps nothing: the server refuses it. */
export function keywordSwapsNothing(letters: readonly string[]): boolean {
  return letters.every((c, i) => c === String.fromCharCode(97 + i));
}

/** The distinct letters of a ciphered text, A to Z in order: the columns of the player's decoding table. */
export function cipherColumns(ciphertext: string): string[] {
  const seen = new Set<string>();
  for (const ch of ciphertext.toUpperCase()) {
    if (ch >= 'A' && ch <= 'Z') {
      seen.add(ch);
    }
  }
  return [...seen].sort();
}
