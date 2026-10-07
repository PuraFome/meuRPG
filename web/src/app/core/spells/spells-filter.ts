import type { MessageInitShape } from '@bufbuild/protobuf';

import type { ListSpellsRequestSchema } from '../../../gen/meurpg/rules/v1/rules_pb';

/**
 * What the "Magias" page asks the server (MR-045, E10-11). The page filters nothing itself: every one of
 * these goes into `ListSpells`, which finds, filters, sorts and counts.
 */
export interface SpellFilter {
  /** Part of the name, in Portuguese or the SRD's English. */
  readonly query: string;
  /** A class key; empty for any. */
  readonly classKey: string;
  /** The circles, 0 (truque) to 9; empty for all. */
  readonly levels: readonly number[];
  /** A school key; empty for any. */
  readonly schoolKey: string;
  /** "Só as que posso aprender", for the player's own character. */
  readonly onlyMine: boolean;
}

export const NO_FILTER: SpellFilter = {
  query: '',
  classKey: '',
  levels: [],
  schoolKey: '',
  onlyMine: false,
};

/** How many rows a page asks for (the most the server gives); the SRD's 319 spells and the table's up to 300 come in two pages, "Mostrar mais" asks for the next. */
export const PAGE_SIZE = 400;

/** The eight schools of the SRD, in the order the book lists them. */
export const SCHOOLS: readonly { readonly key: string; readonly label: string }[] = [
  { key: 'school:abjuration', label: 'Abjuração' },
  { key: 'school:conjuration', label: 'Conjuração' },
  { key: 'school:divination', label: 'Adivinhação' },
  { key: 'school:enchantment', label: 'Encantamento' },
  { key: 'school:evocation', label: 'Evocação' },
  { key: 'school:illusion', label: 'Ilusão' },
  { key: 'school:necromancy', label: 'Necromancia' },
  { key: 'school:transmutation', label: 'Transmutação' },
];

/** The circle chips: "Truque", "1º"... "9º". */
export const LEVEL_CHIPS: readonly { readonly level: number; readonly label: string }[] =
  Array.from({ length: 10 }, (_, level) => ({
    level,
    label: level === 0 ? 'Truque' : `${level}º`,
  }));

/** The `ListSpells` request for a filter and a page. `characterId` goes only with "Só as que posso aprender". */
export function toListRequest(
  campaignId: string,
  filter: SpellFilter,
  characterId: string | null,
  pageToken = '',
): MessageInitShape<typeof ListSpellsRequestSchema> {
  return {
    campaignId,
    query: filter.query.trim(),
    classKey: filter.classKey,
    levels: [...filter.levels],
    schoolKeys: filter.schoolKey ? [filter.schoolKey] : [],
    characterId: filter.onlyMine && characterId ? characterId : '',
    pageSize: PAGE_SIZE,
    pageToken,
  };
}

/** How many filters beyond the name are on: the number in "Filtros (2)". */
export function activeFilters(filter: SpellFilter): number {
  return (
    (filter.classKey ? 1 : 0) +
    (filter.levels.length > 0 ? 1 : 0) +
    (filter.schoolKey ? 1 : 0) +
    (filter.onlyMine ? 1 : 0)
  );
}

/** Whether anything narrows the list, the name included. */
export function isFiltered(filter: SpellFilter): boolean {
  return filter.query.trim() !== '' || activeFilters(filter) > 0;
}

/** The circles toggled: one more, or one less, kept in order. */
export function toggleLevel(levels: readonly number[], level: number): number[] {
  return levels.includes(level)
    ? levels.filter((l) => l !== level)
    : [...levels, level].sort((a, b) => a - b);
}

export interface FilterChip {
  readonly id: 'class' | 'level' | 'school' | 'mine';
  readonly label: string;
}

/** The chips under the phone's search, one per filter that is on (each one is taken off with its ✕). */
export function filterChips(filter: SpellFilter, className: (key: string) => string): FilterChip[] {
  const chips: FilterChip[] = [];
  // A link with a class shows its chip once the class names are known (never an empty one).
  if (filter.classKey && className(filter.classKey) !== '') {
    chips.push({ id: 'class', label: className(filter.classKey) });
  }
  if (filter.levels.length > 0) {
    chips.push({
      id: 'level',
      label: filter.levels.map((l) => (l === 0 ? 'Truque' : `${l}º nível`)).join(', '),
    });
  }
  if (filter.schoolKey) {
    chips.push({
      id: 'school',
      label: SCHOOLS.find((s) => s.key === filter.schoolKey)?.label ?? '',
    });
  }
  if (filter.onlyMine) {
    chips.push({ id: 'mine', label: 'Só as que posso aprender' });
  }
  return chips;
}
