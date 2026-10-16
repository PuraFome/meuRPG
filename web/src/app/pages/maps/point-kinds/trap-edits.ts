import type { TrapSeverity } from '../../../../gen/meurpg/rules/v1/rules_pb';

/** One line of the SRD's severity table, as the server sends it: "revés 10 a 11 · perigosa 12 a 15 · mortal 16 a 20". */
export function severityLine(
  severities: readonly TrapSeverity[],
  range: (s: TrapSeverity) => string,
): string {
  return severities.map((s) => `${s.namePt.toLocaleLowerCase('pt-BR')} ${range(s)}`).join(' · ');
}

/** The list with the part at `i` changed, the others kept. */
export function changeAt<T>(list: readonly T[], i: number, change: Partial<T>): T[] {
  return list.map((part, n) => (n === i ? { ...part, ...change } : part));
}

/**
 * What a key does in the "Também acham com" listbox: the cursor's new place, `'pick'` (Space or Enter) or `'close'` (Escape);
 * `null` for a key the list leaves alone.
 */
export function alsoKeyMove(
  key: string,
  cursor: number,
  last: number,
): number | 'pick' | 'close' | null {
  switch (key) {
    case 'ArrowDown':
      return Math.min(last, cursor + 1);
    case 'ArrowUp':
      return Math.max(0, cursor - 1);
    case 'Home':
      return 0;
    case 'End':
      return Math.max(0, last);
    case ' ':
    case 'Enter':
      return 'pick';
    case 'Escape':
      return 'close';
    default:
      return null;
  }
}
