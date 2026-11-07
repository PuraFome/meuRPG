import {
  type TableEntry,
  type TableFeature,
  TableContentKind,
} from '../../../gen/meurpg/rules/v1/table_content_pb';

/**
 * The class and subclass features of the table that have a list of options of their own: the only ones a table option of the set
 * "table_feature" can join (the server's `offered_by`). Read from the entries the page already holds.
 */
export interface OfferingFeature {
  readonly key: string;
  readonly name: string;
  readonly level: number;
  /** The names of the options the feature lists itself. */
  readonly options: readonly string[];
  /** The owner entry: a subclass or a class of the table. */
  readonly ownerKey: string;
  readonly owner: string;
  readonly ownerKind: 'subclass' | 'class';
  /** The class a subclass belongs to, by name ('' for a class). */
  readonly classKey: string;
}

export function offeringFeatures(entries: readonly TableEntry[]): OfferingFeature[] {
  return entries.filter((e) => !e.archived).flatMap(featuresOf);
}

type Row = { features: readonly TableFeature[] };

function featuresOf(e: TableEntry): OfferingFeature[] {
  const b = e.body;
  if (e.kind === TableContentKind.CLASS && b.case === 'tableClass') {
    // A class's rows are the 20 levels in order.
    return rowsOf(e, b.value.levels, 'class', '', (_, i) => i + 1);
  }
  if (e.kind === TableContentKind.SUBCLASS && b.case === 'tableSubclass') {
    return rowsOf(e, b.value.levels, 'subclass', b.value.classKey, (l) => l.level);
  }
  return [];
}

function rowsOf<L extends Row>(
  e: TableEntry,
  levels: readonly L[],
  kind: 'subclass' | 'class',
  classKey: string,
  levelOf: (row: L, index: number) => number,
): OfferingFeature[] {
  return levels.flatMap((row, i) =>
    row.features
      .filter((f) => f.options.length > 0 && f.key !== '')
      .map((f) => feature(f, levelOf(row, i), e, kind, classKey)),
  );
}

function feature(
  f: { key: string; namePt: string; options: readonly { namePt: string }[] },
  level: number,
  owner: TableEntry,
  ownerKind: 'subclass' | 'class',
  classKey: string,
): OfferingFeature {
  return {
    key: f.key,
    name: f.namePt,
    level,
    options: f.options.map((o) => o.namePt),
    ownerKey: owner.key,
    owner: owner.namePt,
    ownerKind,
    classKey,
  };
}
