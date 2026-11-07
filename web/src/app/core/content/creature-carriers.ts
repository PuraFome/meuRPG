import {
  TableContentKind,
  type TableEntry,
  type TableFeature,
} from '../../../gen/meurpg/rules/v1/table_content_pb';

/** One entry of the table that brings the creature to a character (#578, "Quem traz esta criatura"). */
export interface Carrier {
  /** The entry that carries it: its page. */
  readonly key: string;
  readonly kind: TableContentKind;
  readonly name: string;
  /** "Guardião do Vale, nível 3", "magia da mesa, 3º círculo". */
  readonly detail: string;
}

type Summon = { readonly creatureKeys: readonly string[] } | undefined;
type Found = readonly { readonly name: string; readonly detail: string }[];

/** The features of a list that bring the creature, each with the place it comes at. */
function featuresThatBring(
  rows: readonly { readonly features: readonly TableFeature[] }[],
  levelOf: (index: number) => number,
  from: string,
  has: (s: Summon) => boolean,
): Found {
  return rows.flatMap((row, i) =>
    row.features
      .filter((f) => has(f.summon))
      .map((f) => ({ name: f.namePt, detail: `${from}, nível ${levelOf(i)}` })),
  );
}

/** What one entry brings: the spell that summons it, the features (of a class, a subclass, a race or a background) whose summon lists it. */
function foundIn(e: TableEntry, has: (s: Summon) => boolean): Found {
  const b = e.body;
  switch (b.case) {
    case 'tableSpell':
      return has(b.value.summon)
        ? [{ name: e.namePt, detail: `magia da mesa, ${b.value.level}º círculo` }]
        : [];
    case 'tableClassFeature':
      return has(b.value.summon)
        ? [{ name: e.namePt, detail: `característica da classe, nível ${b.value.level}` }]
        : [];
    case 'tableClass':
      return featuresThatBring(b.value.levels, (i) => i + 1, e.namePt, has);
    case 'tableSubclass':
      return featuresThatBring(b.value.levels, (i) => b.value.levels[i].level, e.namePt, has);
    case 'tableRace':
    case 'tableSubrace':
      return b.value.traits
        .filter((f) => has(f.summon))
        .map((f) => ({ name: f.namePt, detail: `traço de ${e.namePt}` }));
    case 'tableBackground':
      return has(b.value.feature?.summon)
        ? [{ name: b.value.feature?.namePt || e.namePt, detail: `antecedente ${e.namePt}` }]
        : [];
    default:
      return [];
  }
}

/** The entries that bring `creatureKey`: the spells that summon it, and the class features, subclass features, traits and
 * background features whose summon lists it. The server's data, read from the entries the editor already has. */
export function carriersOf(
  entries: readonly TableEntry[],
  creatureKey: string,
): readonly Carrier[] {
  if (creatureKey === '') {
    return [];
  }
  const has = (s: Summon) => s?.creatureKeys.includes(creatureKey) ?? false;
  return entries.flatMap((e) =>
    foundIn(e, has).map((f) => ({ key: e.key, kind: e.kind, name: f.name, detail: f.detail })),
  );
}

/** The icon of a carrier: a wand for a spell, a book for everything else. */
export function carrierIcon(c: Carrier): string {
  return c.kind === TableContentKind.SPELL ? 'auto_fix_high' : 'menu_book';
}
