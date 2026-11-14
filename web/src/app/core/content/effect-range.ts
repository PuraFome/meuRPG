import { feetToMeters, metersToFeet } from '../units';
import { WHOLE_MAX } from './whole-number';

/** The metres a range field shows, from the feet it stores. */
export function rangeMeters(ft: number): string {
  return ft > 0 ? String(feetToMeters(ft)).replace('.', ',') : '';
}

/** Whether a range field holds something that is not a number of metres the server can store (empty is fine: it means none). The text is
 * a plain decimal ("18", "4,5"), and its feet are a whole number within int32. */
export function unreadableRange(text: string): boolean {
  const t = text.trim();
  if (t === '') {
    return false;
  }
  if (!/^\d+([.,]\d+)?$/.test(t)) {
    return true;
  }
  const ft = metersToFeet(Number(t.replace(',', '.')));
  return !Number.isSafeInteger(ft) || ft > WHOLE_MAX;
}
