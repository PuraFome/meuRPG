/** A zone as the announcer needs it: who it is and how it reads. */
export interface ZoneNote {
  readonly id: string;
  readonly label: string;
}

function list(one: string, many: string, zones: readonly ZoneNote[]): string {
  if (zones.length === 0) {
    return '';
  }
  const names = zones.map((z) => z.label).join(', ');
  return zones.length === 1 ? `${one}: ${names}.` : `${many}: ${names}.`;
}

/**
 * What the polite live region says when the zones list changes: "Zona criada: Teia · Bartolo." and "Zona encerrada: ...".
 * `before` is `null` on the first read, which says nothing (the list is not news when the screen opens).
 */
export function zoneNotice(before: readonly ZoneNote[] | null, after: readonly ZoneNote[]): string {
  if (before === null) {
    return '';
  }
  const had = new Set(before.map((z) => z.id));
  const has = new Set(after.map((z) => z.id));
  const created = list(
    'Zona criada',
    'Zonas criadas',
    after.filter((z) => !had.has(z.id)),
  );
  const ended = list(
    'Zona encerrada',
    'Zonas encerradas',
    before.filter((z) => !has.has(z.id)),
  );
  return [created, ended].filter((t) => t !== '').join(' ');
}
