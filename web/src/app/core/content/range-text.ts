import { feetToMeters, metersToFeet } from '../units';

/** The metres a range field shows, from the feet it stores. */
export function rangeMeters(ft: number): string {
  return ft > 0 ? String(feetToMeters(ft)).replace('.', ',') : '';
}

/** The feet of what a range field holds ("18" or "4,5" metres); 0 for an empty or unreadable field. */
export function rangeFeet(text: string): number {
  const n = Number(text.trim().replace(',', '.'));
  return Number.isFinite(n) && n > 0 ? metersToFeet(n) : 0;
}

/** Whether a range field holds something that is not a number of metres (empty is fine: it means none). */
export function unreadableRange(text: string): boolean {
  const t = text.trim();
  if (t === '') {
    return false;
  }
  const n = Number(t.replace(',', '.'));
  return !Number.isFinite(n) || n < 0;
}

/** What the editor says of a range field it cannot read. */
export const UNREADABLE_RANGE = 'Escreva só o número de metros, como 18 ou 4,5.';

/** Like rangeFeet, but unreadable text is NaN instead of 0, so a field where 0 is valid cannot save it by accident. */
export function strictRangeFeet(text: string): number {
  return unreadableRange(text) ? Number.NaN : rangeFeet(text);
}
