/** The lowest and the highest DC the server takes. */
export const DC_MIN = 1;
export const DC_MAX = 40;

/** The DC typed, or `0` (none) when it is empty, or `null` when it is not a whole number the server takes. */
export function dcOf(text: string): number | null {
  const t = text.trim();
  if (t === '') {
    return 0;
  }
  const n = Number(t);
  return Number.isInteger(n) && n >= DC_MIN && n <= DC_MAX ? n : null;
}
