import {
  type CasterSection,
  type ExtraSpellList,
  type SpellSourceVm,
  allOfSection,
  cantripsOf,
  extraSpellLists,
  leveledOf,
  outsideTheLists,
  sectionName,
} from './class-blocks';
import type {
  RulesCatalogVm,
  SpellLimitsVm,
  SpellOptionVm,
  SpellPreparation,
} from './character-editor.types';

/** What the "Magias" step lists for each casting class, from the server's catalog and the picks of the form. */

/** The "search box" the catalog-backed pickers use to narrow a long list (cantrips, spells): a plain
 * case-insensitive substring match on the Portuguese name, no new dependency. */
export function filterByName<T extends { readonly namePt: string }>(
  items: readonly T[],
  query: string,
): readonly T[] {
  const q = query.trim().toLowerCase();
  return q ? items.filter((item) => item.namePt.toLowerCase().includes(q)) : items;
}

/** "Preparadas 7 de 5" for a preparing class whose number is known, and whether it is past the number. */
export function preparedCountLine(
  preparation: SpellPreparation | null,
  picked: number,
  max: number | undefined,
) {
  if (preparation === 'known' || max === undefined) {
    return { text: '', over: false };
  }
  return { text: `Preparadas ${picked} de ${max}`, over: picked > max };
}

/** What each spell list of a section takes at most: `null` where there is no number to show. `preparedMax` is the
 * number of prepared spells whether or not the class prepares, for the "Preparadas N de M" line. */
export function spellListLimits(
  server: SpellLimitsVm | undefined,
  preparation: SpellPreparation | null,
  saved: number | undefined,
) {
  const preparedMax = server && server.preparedMax > 0 ? server.preparedMax : saved;
  return {
    cantrips: server?.cantripsKnown ? server.cantripsKnown : null,
    known: preparation === 'known' && server?.spellsKnown ? server.spellsKnown : null,
    prepared: preparation !== 'known' && preparedMax ? preparedMax : null,
    preparedMax,
  };
}

/** Every cantrip and spell of the lists of the sections, once each, by circle and then by name. */
function unionOf(
  catalog: RulesCatalogVm,
  sections: readonly CasterSection[],
  pick: (section: CasterSection, spells: readonly SpellOptionVm[]) => SpellOptionVm[],
): SpellOptionVm[] {
  const seen = new Map<string, SpellOptionVm>();
  for (const section of sections) {
    for (const spell of pick(section, catalog.spells)) {
      seen.set(spell.key, spell);
    }
  }
  return [...seen.values()].sort(
    (a, b) => a.level - b.level || a.namePt.localeCompare(b.namePt, 'pt-BR'),
  );
}

/** Every cantrip of the sheet's lists, for "the chosen line" and for what a save may send. */
export function availableCantripsOf(
  catalog: RulesCatalogVm,
  sections: readonly CasterSection[],
  picked: ReadonlySet<string>,
): SpellOptionVm[] {
  return unionOf(catalog, sections, (sec, spells) =>
    cantripsOf(spells, sec, new Set(picked), true),
  );
}

/** Every spell of the sheet's lists, and the extra lists' (the patron's spells and the Magical Secrets are picks of
 * the sheet too, though no class list has them). */
export function availableSpellsOf(
  catalog: RulesCatalogVm,
  sections: readonly CasterSection[],
  extras: readonly { index: number; lists: ExtraSpellList[] }[],
): SpellOptionVm[] {
  const own = unionOf(catalog, sections, (sec, spells) =>
    allOfSection(spells, sec).filter((sp) => sp.level >= 1),
  );
  const seen = new Set(own.map((sp) => sp.key));
  const extra = extras
    .flatMap((e) => e.lists.flatMap((l) => l.spells))
    .filter((sp) => !seen.has(sp.key) && seen.add(sp.key));
  return [...own, ...extra];
}

/** For each section, the spells outside its list that `PreviewChoices` lets it pick. */
export function sectionExtrasOf(
  catalog: RulesCatalogVm,
  sections: readonly CasterSection[],
  sources: readonly SpellSourceVm[],
  picked: ReadonlySet<string>,
  master: boolean,
): { index: number; lists: ExtraSpellList[] }[] {
  if (sources.length === 0) {
    return [];
  }
  return sections.map((section) => ({
    index: section.index,
    lists: extraSpellLists(
      catalog,
      section,
      sources.find((src) => src.classKey === section.classKey),
      picked,
      master,
    ),
  }));
}

/** Only the selected keys that are still in the (class-filtered) available list. A pick the catalog does not know at
 * all is kept, never dropped quietly: the server judges it. Only a spell that is known and not on the lists any more
 * (the class changed) goes. */
export function keepAvailable(
  selected: ReadonlySet<string>,
  available: readonly { readonly key: string }[],
  catalogSpells: readonly { readonly key: string }[],
): string[] {
  const availableKeys = new Set(available.map((item) => item.key));
  const known = new Set(catalogSpells.map((sp) => sp.key));
  return Array.from(selected).filter((key) => availableKeys.has(key) || !known.has(key));
}

/** What a section's pickers read: the draft's picks, the search of each list, and the server's numbers. */
export interface SectionViewInput {
  readonly catalog: RulesCatalogVm;
  readonly sections: readonly CasterSection[];
  readonly section: CasterSection;
  readonly cantrips: ReadonlySet<string>;
  readonly known: ReadonlySet<string>;
  readonly prepared: ReadonlySet<string>;
  readonly master: boolean;
  /** The search of each list, by `<section>:<list>`: "0:cantrips", "1:known", "1:prepared". */
  readonly filters: Readonly<Record<string, string>>;
  readonly extras: readonly { index: number; lists: ExtraSpellList[] }[];
  /** The draft's numbers when the server has answered. */
  readonly serverLimits: SpellLimitsVm | undefined;
  /** Else the saved sheet's prepared number (an edit). */
  readonly savedPreparedMax: number | undefined;
}

/** One list of a section as the picker draws it: all of it, what the search leaves, the search, and what the search
 * finds outside the sheet's lists. */
function listView(
  i: SectionViewInput,
  list: 'cantrips' | 'known' | 'prepared',
  all: SpellOptionVm[],
  show: boolean,
) {
  const query = i.filters[`${i.section.index}:${list}`] ?? '';
  return {
    all,
    shown: filterByName(all, query),
    filter: query,
    show,
    outside: outsideTheLists(i.catalog, i.sections, query, list === 'cantrips'),
  };
}

/** What each section's pickers list now: the cantrips, the spells up to the circle the class level reaches (plus any
 * picked above it, so it can be unchecked), each narrowed by its own search. */
export function buildSectionView(i: SectionViewInput) {
  const { section, catalog } = i;
  const cantrips = cantripsOf(catalog.spells, section, i.cantrips, i.master);
  // An always-prepared spell is never a pick: it is in its locked row, free, and never in these lists.
  const alwaysKeys = new Set(section.alwaysPrepared);
  const known = leveledOf(catalog.spells, section, i.known, i.master).filter(
    (sp) => !alwaysKeys.has(sp.key),
  );
  const prepared = leveledOf(catalog.spells, section, i.prepared, i.master).filter(
    (sp) => !alwaysKeys.has(sp.key),
  );
  // A wizard prepares only from the spellbook (SRD 5.1): its list is the book's spells (and what is already
  // prepared, so a wrong pick can still be unchecked).
  const preparable =
    section.preparation === 'spellbook'
      ? prepared.filter((sp) => i.known.has(sp.key) || i.prepared.has(sp.key))
      : prepared;
  // The subclass's always-prepared spells, shown in this class's section, checked and locked, never in the count.
  const byKey = new Map(catalog.spells.map((sp) => [sp.key, sp]));
  const always = section.alwaysPrepared.flatMap((k) => (byKey.has(k) ? [byKey.get(k)!] : []));
  const limits = spellListLimits(i.serverLimits, section.preparation, i.savedPreparedMax);
  const picked = prepared.filter((sp) => i.prepared.has(sp.key) && !alwaysKeys.has(sp.key)).length;
  const preparation = section.preparation;
  const noLeveledYet = section.maxCircle === 0 && i.known.size === 0 && i.prepared.size === 0;
  return {
    section,
    id: String(section.index),
    extras: i.extras.find((e) => e.index === section.index)?.lists ?? [],
    /** The extra lists add to the known spells of a "known" or "spellbook" class, else to the prepared ones. */
    extrasToKnown: section.preparation !== 'prepared',
    cantrips: listView(
      i,
      'cantrips',
      cantrips,
      // A cantrip already on the sheet keeps the picker, so it can still be unchecked.
      cantrips.length > 0 || (i.sections.length === 1 && i.cantrips.size > 0),
    ),
    known: listView(
      i,
      'known',
      known,
      !noLeveledYet && (preparation === 'known' || preparation === 'spellbook'),
    ),
    prepared: listView(
      i,
      'prepared',
      preparable,
      !noLeveledYet && (preparation === 'prepared' || preparation === 'spellbook'),
    ),
    always,
    // The most each list takes (0 or none: no limit to show).
    cantripsLimit: limits.cantrips,
    knownLimit: limits.known,
    preparedLimit: limits.prepared,
    alwaysSourcePt: section.alwaysSourcePt,
    // On an edit the server says how many the class prepares (its own number, from the saved sheet).
    preparedCount: preparedCountLine(section.preparation, picked, limits.preparedMax),
    noLeveledYet,
    castingStarts: `O ${sectionName(section)} conjura magias a partir do nível ${section.firstLevel}.`,
  };
}
