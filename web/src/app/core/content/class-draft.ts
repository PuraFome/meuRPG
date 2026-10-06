import type { MessageInitShape } from '@bufbuild/protobuf';

import { Ability } from '../../../gen/meurpg/rules/v1/rules_pb';
import {
  type CastingTableDefault,
  type GetClassTableDefaultsResponse,
  type TableCasting,
  TableClassSchema,
  type TableClass,
  type TableClassLevel,
  type TableSubclass,
  TableSubclassSchema,
} from '../../../gen/meurpg/rules/v1/table_content_pb';
import type { EffectMenuVm } from './effect-draft';
import {
  ABILITY_FIELDS,
  type AbilityField,
  type FeatureDraft,
  draftToFeature,
  featureOwnPaths,
  featureToDraft,
} from './feature-draft';

/**
 * The class and subclass forms (MR-025, RN-23, ADR-0018, E10-02): what the editor holds, and the request it makes of it. The
 * 20-level table starts from `GetClassTableDefaults` (the server's rows, pasted whole) and the master edits cells; nothing here
 * works a slot, a bonus or a spell count out. The features of a class are one flat list with a level each (the screen shows
 * them as a list); the request puts each under its level, and `classFeatureBase` says where, so a refusal at
 * `table_class.levels[4].features[0].effects[0].value` lands on the right input.
 */

export const LEVEL_COUNT = 20;
/** The circles a row has columns for (1st to 9th). */
export const CIRCLES = 9;

export type ClassInit = MessageInitShape<typeof TableClassSchema>;
export type SubclassInit = MessageInitShape<typeof TableSubclassSchema>;

export type CastingKind = '' | 'full' | 'half' | 'pact' | 'third';
export type Preparation = 'known' | 'prepared';

/** One row of the table: the proficiency bonus and the casting columns (the features of a level are the draft's). */
export interface RowDraft {
  profBonus: number;
  cantrips: number;
  spells: number;
  /** Nine entries, 1st circle first. */
  slots: number[];
}

export interface CastingDraft {
  kind: CastingKind;
  ability: Ability;
  preparation: Preparation;
  /** The class whose list it casts from; empty is the class's own list. */
  listFrom: string;
  /** A formula; empty is the server's (the ability modifier plus the level, half of it for a half caster). */
  preparedMax: string;
  /** 0 is the kind's default. */
  startLevel: number;
  ritual: boolean;
}

export interface LevelFeature {
  /** A stable id the screen tracks the row by. */
  id: string;
  level: number;
  feature: FeatureDraft;
}

export type Minimums = Record<AbilityField, number>;

export interface ClassDraft {
  name: string;
  hitDie: number;
  saves: [Ability, Ability];
  skillChoose: number;
  skillFrom: string[];
  /** Armor, weapon and tool proficiency keys. */
  proficiencies: string[];
  /** The multiclass prerequisites: every ability here (a score above 0) must be met... */
  minimums: Minimums;
  /** ...or, for `anyOf`, one of them. */
  anyOf: Minimums;
  multiclassProficiencies: string[];
  multiclassSkillChoose: number;
  subclassLevel: number;
  asiLevels: number[];
  casting: CastingDraft;
  rows: RowDraft[];
  features: LevelFeature[];
}

export interface AlwaysPreparedGroup {
  level: number;
  spells: string[];
}

export interface SubclassDraft {
  name: string;
  classKey: string;
  /** The class level the choice happens at; 0 is the class's. Kept as the entry has it. */
  level: number;
  /** One paragraph per line break. */
  text: string;
  conjures: boolean;
  casting: CastingDraft;
  rows: RowDraft[];
  features: LevelFeature[];
  alwaysPrepared: AlwaysPreparedGroup[];
}

let nextFeatureId = 0;
export function newLevelFeatureId(): string {
  return `lf-${nextFeatureId++}`;
}

export function noMinimums(): Minimums {
  return { strength: 0, dexterity: 0, constitution: 0, intelligence: 0, wisdom: 0, charisma: 0 };
}

export function emptyRow(): RowDraft {
  return { profBonus: 0, cantrips: 0, spells: 0, slots: Array<number>(CIRCLES).fill(0) };
}

export function emptyCasting(kind: CastingKind = ''): CastingDraft {
  return { kind, ability: Ability.UNSPECIFIED, preparation: 'prepared', listFrom: '', preparedMax: '', startLevel: 0, ritual: false };
}

// ---------------------------------------------------------------------------------------------------------------------
// The server's defaults.

/** The default table of one way of casting: `kind` '' is a class that does not cast. */
export function tableFor(defaults: GetClassTableDefaultsResponse, kind: CastingKind, preparation: Preparation): CastingTableDefault | undefined {
  return defaults.tables.find((t) => t.kind === kind && (kind === '' || t.preparation === preparation));
}

function rowOf(r: Pick<TableClassLevel, 'cantripsKnown' | 'spellsKnown' | 'slots'> & { profBonus?: number }): RowDraft {
  const slots = Array<number>(CIRCLES).fill(0);
  r.slots.forEach((n, i) => {
    if (i < CIRCLES) slots[i] = n;
  });
  return { profBonus: r.profBonus ?? 0, cantrips: r.cantripsKnown, spells: r.spellsKnown, slots };
}

/** The 20 rows of a default table, as drafts: the server's numbers, copied. */
export function rowsOfTable(table: CastingTableDefault | undefined, defaults: GetClassTableDefaultsResponse): RowDraft[] {
  if (table && table.rows.length === LEVEL_COUNT) {
    return table.rows.map(rowOf);
  }
  return Array.from({ length: LEVEL_COUNT }, (_, i) => ({ ...emptyRow(), profBonus: defaults.profBonus[i] ?? 0 }));
}

/** The table of the casting the draft has now. */
export function defaultTableOf(c: CastingDraft, defaults: GetClassTableDefaultsResponse): CastingTableDefault | undefined {
  return tableFor(defaults, c.kind, c.preparation);
}

/** Whether two tables are the same. A third caster's table (a subclass's) has no proficiency bonus of its own: the server stores
 * none, so it is left out of the comparison (`bonus` false). */
export function sameRows(a: readonly RowDraft[], b: readonly RowDraft[], bonus = true): boolean {
  return (
    a.length === b.length &&
    a.every((r, i) => {
      const o = b[i];
      return (!bonus || r.profBonus === o.profBonus) && r.cantrips === o.cantrips && r.spells === o.spells && r.slots.every((n, k) => n === o.slots[k]);
    })
  );
}

/** Whether the master changed the table from the default of the casting it has now (asked about before it is replaced). A third
 * caster compares the casting columns only: the rows of a stored subclass carry no bonus. */
export function rowsEdited(rows: readonly RowDraft[], c: CastingDraft, defaults: GetClassTableDefaultsResponse): boolean {
  return !sameRows(rows, rowsOfTable(defaultTableOf(c, defaults), defaults), c.kind !== 'third');
}

/** The casting a kind starts with: its own default preparation, the table's start level, nothing else chosen yet. */
export function castingOfKind(kind: CastingKind, previous: CastingDraft, defaults: GetClassTableDefaultsResponse): CastingDraft {
  if (kind === '') {
    return emptyCasting('');
  }
  const preparation = tableFor(defaults, kind, previous.preparation) ? previous.preparation : 'prepared';
  const table = tableFor(defaults, kind, preparation);
  return { ...previous, kind, preparation, startLevel: table?.startLevel ?? 0, preparedMax: preparation === 'prepared' ? previous.preparedMax : '' };
}

export function emptyClass(defaults: GetClassTableDefaultsResponse): ClassDraft {
  return {
    name: '',
    hitDie: 8,
    saves: [Ability.UNSPECIFIED, Ability.UNSPECIFIED],
    skillChoose: 2,
    skillFrom: [],
    proficiencies: [],
    minimums: noMinimums(),
    anyOf: noMinimums(),
    multiclassProficiencies: [],
    multiclassSkillChoose: 0,
    subclassLevel: defaults.subclassLevel,
    asiLevels: [...defaults.asiLevels],
    casting: emptyCasting(''),
    rows: rowsOfTable(tableFor(defaults, '', 'prepared'), defaults),
    features: [],
  };
}

// ---------------------------------------------------------------------------------------------------------------------
// From the entry and back.

/** A stored casting as a draft. A start level of 0 means "the kind's own" to the server; the draft holds the table's number. */
function castingToDraft(c: TableCasting | undefined, defaults: GetClassTableDefaultsResponse): CastingDraft {
  if (!c || c.kind === '') {
    return emptyCasting('');
  }
  const preparation = c.preparation === 'known' ? 'known' : 'prepared';
  return {
    kind: c.kind as CastingKind,
    ability: c.ability,
    preparation,
    listFrom: c.listFrom,
    preparedMax: c.preparedMax,
    startLevel: c.startLevel > 0 ? c.startLevel : (tableFor(defaults, c.kind as CastingKind, preparation)?.startLevel ?? 1),
    ritual: c.ritual,
  };
}

function castingInit(c: CastingDraft): MessageInitShape<typeof TableClassSchema>['casting'] {
  return {
    kind: c.kind,
    ability: c.ability,
    preparation: c.preparation,
    listFrom: c.listFrom,
    preparedMax: c.preparation === 'prepared' ? c.preparedMax.trim() : '',
    startLevel: c.startLevel,
    ritual: c.ritual,
  };
}

function minimumsOf(m: TableClass['minimums']): Minimums {
  const out = noMinimums();
  for (const a of ABILITY_FIELDS) out[a] = m?.[a] ?? 0;
  return out;
}

function hasMinimums(m: Minimums): boolean {
  return ABILITY_FIELDS.some((a) => m[a] > 0);
}

export function classToDraft(c: TableClass, defaults: GetClassTableDefaultsResponse): ClassDraft {
  const rows = Array.from({ length: LEVEL_COUNT }, (_, i) => (c.levels[i] ? rowOf(c.levels[i]) : emptyRow()));
  const features: LevelFeature[] = [];
  c.levels.forEach((lv, i) => lv.features.forEach((f) => features.push({ id: newLevelFeatureId(), level: i + 1, feature: featureToDraft(f) })));
  return {
    name: c.namePt,
    hitDie: c.hitDie,
    saves: [c.savingThrows[0] ?? Ability.UNSPECIFIED, c.savingThrows[1] ?? Ability.UNSPECIFIED],
    skillChoose: c.skillChoose,
    skillFrom: [...c.skillFrom],
    proficiencies: [...c.proficiencies],
    minimums: minimumsOf(c.minimums),
    anyOf: minimumsOf(c.anyOf),
    multiclassProficiencies: [...c.multiclassProficiencies],
    multiclassSkillChoose: c.multiclassSkillChoose,
    subclassLevel: c.subclassLevel || defaults.subclassLevel,
    asiLevels: c.asiLevels.length > 0 ? [...c.asiLevels] : [...defaults.asiLevels],
    casting: castingToDraft(c.casting, defaults),
    rows,
    features,
  };
}

function minimumsInit(m: Minimums): Partial<Record<AbilityField, number>> | undefined {
  return hasMinimums(m) ? { ...m } : undefined;
}

/** The slots of a row: nine entries, or none when every one is zero (the server reads both the same). */
function slotsOf(r: RowDraft): number[] {
  return r.slots.some((n) => n !== 0) ? [...r.slots] : [];
}

function featuresAt(features: readonly LevelFeature[], level: number, menu: EffectMenuVm): ReturnType<typeof draftToFeature>[] {
  return features.filter((f) => f.level === level).map((f) => draftToFeature(f.feature, menu));
}

export function draftToClass(d: ClassDraft, menu: EffectMenuVm): ClassInit {
  return {
    namePt: d.name.trim(),
    hitDie: d.hitDie,
    savingThrows: d.saves.filter((a) => a !== Ability.UNSPECIFIED),
    skillChoose: d.skillChoose,
    skillFrom: [...d.skillFrom],
    proficiencies: [...d.proficiencies],
    minimums: minimumsInit(d.minimums),
    anyOf: minimumsInit(d.anyOf),
    multiclassProficiencies: [...d.multiclassProficiencies],
    multiclassSkillChoose: d.multiclassSkillChoose,
    subclassLevel: d.subclassLevel,
    asiLevels: [...d.asiLevels].sort((a, b) => a - b),
    casting: d.casting.kind === '' ? undefined : castingInit(d.casting),
    levels: d.rows.map((r, i) => ({
      profBonus: r.profBonus,
      cantripsKnown: r.cantrips,
      spellsKnown: r.spells,
      slots: slotsOf(r),
      features: featuresAt(d.features, i + 1, menu),
    })),
  };
}

// ---------------------------------------------------------------------------------------------------------------------
// Where a feature goes in the request: its path.

/** "table_class.levels[4].features[1]": the level's place in the request and the feature's place among its level's. */
export function classFeatureBase(features: readonly LevelFeature[], index: number): string {
  const f = features[index];
  const k = features.slice(0, index).filter((x) => x.level === f.level).length;
  return `table_class.levels[${f.level - 1}].features[${k}]`;
}

/** The inputs of every feature of a class, by path (the known set the violations are placed against). */
export function classFeaturePaths(features: readonly LevelFeature[], menu: EffectMenuVm): string[] {
  return features.flatMap((f, i) => featureOwnPaths(classFeatureBase(features, i), f.feature, menu));
}

/** The features in the order of their level, keeping the order inside a level (a stable sort; what the request sends). */
export function sortedFeatures(features: readonly LevelFeature[]): LevelFeature[] {
  return features.map((f, i) => ({ f, i })).sort((a, b) => a.f.level - b.f.level || a.i - b.i).map((x) => x.f);
}

/** The levels the request has a row for: all 20 for a class; for a subclass, the ones that cast or have a feature, ascending. */
export function subclassLevels(d: SubclassDraft): number[] {
  const start = d.casting.startLevel > 0 ? d.casting.startLevel : 1;
  const out: number[] = [];
  for (let level = 1; level <= LEVEL_COUNT; level++) {
    if ((d.conjures && level >= start) || d.features.some((f) => f.level === level)) {
      out.push(level);
    }
  }
  return out;
}

export function subclassFeatureBase(d: SubclassDraft, index: number): string {
  const f = d.features[index];
  const levels = subclassLevels(d);
  const k = d.features.slice(0, index).filter((x) => x.level === f.level).length;
  return `table_subclass.levels[${levels.indexOf(f.level)}].features[${k}]`;
}

export function subclassFeaturePaths(d: SubclassDraft, menu: EffectMenuVm): string[] {
  return d.features.flatMap((f, i) => featureOwnPaths(subclassFeatureBase(d, i), f.feature, menu));
}

/** "table_subclass.levels[2]": the row of a class level of a third caster, or '' when the request has none for it. */
export function subclassRowBase(d: SubclassDraft, level: number): string {
  const i = subclassLevels(d).indexOf(level);
  return i < 0 ? '' : `table_subclass.levels[${i}]`;
}

export function emptySubclass(classKey: string): SubclassDraft {
  return { name: '', classKey, level: 0, text: '', conjures: false, casting: emptyCasting(''), rows: Array.from({ length: LEVEL_COUNT }, emptyRow), features: [], alwaysPrepared: [] };
}

export function subclassToDraft(s: TableSubclass, defaults: GetClassTableDefaultsResponse): SubclassDraft {
  const rows = Array.from({ length: LEVEL_COUNT }, emptyRow);
  const features: LevelFeature[] = [];
  for (const lv of s.levels) {
    if (lv.level >= 1 && lv.level <= LEVEL_COUNT) {
      rows[lv.level - 1] = rowOf(lv);
    }
    lv.features.forEach((f) => features.push({ id: newLevelFeatureId(), level: lv.level, feature: featureToDraft(f) }));
  }
  const groups = new Map<number, string[]>();
  for (const a of s.alwaysPrepared) {
    groups.set(a.classLevel, [...(groups.get(a.classLevel) ?? []), a.spellKey]);
  }
  return {
    name: s.namePt,
    classKey: s.classKey,
    level: s.level,
    text: s.descPt.join('\n\n'),
    conjures: s.casting !== undefined && s.casting.kind !== '',
    casting: castingToDraft(s.casting, defaults),
    rows,
    features,
    alwaysPrepared: [...groups].sort((a, b) => a[0] - b[0]).map(([level, spells]) => ({ level, spells })),
  };
}

/** The paragraphs of a text: blank lines split them. */
function paragraphsOf(text: string): string[] {
  return text.split(/\n\s*\n/).map((p) => p.trim()).filter((p) => p !== '');
}

export function draftToSubclass(d: SubclassDraft, menu: EffectMenuVm): SubclassInit {
  return {
    namePt: d.name.trim(),
    classKey: d.classKey,
    level: d.level,
    descPt: paragraphsOf(d.text),
    casting: d.conjures ? castingInit(d.casting) : undefined,
    levels: subclassLevels(d).map((level) => {
      const r = d.rows[level - 1];
      const cast = d.conjures && level >= (d.casting.startLevel > 0 ? d.casting.startLevel : 1);
      return {
        level,
        features: featuresAt(d.features, level, menu),
        cantripsKnown: cast ? r.cantrips : 0,
        spellsKnown: cast ? r.spells : 0,
        slots: cast ? slotsOf(r) : [],
      };
    }),
    alwaysPrepared: flattenAlwaysPrepared(d.alwaysPrepared),
  };
}

/** The request's list: a class level and a spell each, in the order of the groups. */
export function flattenAlwaysPrepared(groups: readonly AlwaysPreparedGroup[]): { classLevel: number; spellKey: string }[] {
  return groups.flatMap((g) => g.spells.map((spellKey) => ({ classLevel: g.level, spellKey })));
}

// ---------------------------------------------------------------------------------------------------------------------
// The 20-level table, and what it says in words.

/** The columns of the grid for a casting: which exist and how many circles. The numbers in them are the server's. */
export interface GridColumns {
  readonly casts: boolean;
  readonly cantrips: boolean;
  /** "Magias": only for a class that knows its spells. */
  readonly spells: boolean;
  /** How many circles have a column (the highest circle the default table, or the rows, use; never fewer than `min`). */
  readonly circles: number;
}

/** The columns of the table of a casting. The circles shown are the highest one any row of the default or of the draft has
 * a slot at, so a half caster shows 5 and a full caster 9; `all` shows the nine (to give a half caster a 6th circle). */
export function gridColumns(c: CastingDraft, rows: readonly RowDraft[], defaults: GetClassTableDefaultsResponse, all: boolean): GridColumns {
  if (c.kind === '') {
    return { casts: false, cantrips: false, spells: false, circles: 0 };
  }
  const table = defaultTableOf(c, defaults);
  const highest = (rs: readonly RowDraft[]): number => rs.reduce((m, r) => Math.max(m, r.slots.reduce((h, n, i) => (n > 0 ? i + 1 : h), 0)), 0);
  const fromDefault = table ? highest(table.rows.map(rowOf)) : 0;
  const circles = all ? CIRCLES : Math.max(fromDefault, highest(rows), 1);
  return { casts: true, cantrips: true, spells: c.preparation === 'known', circles };
}

/** The ordinal of a circle: "1º". */
export function circleLabel(i: number): string {
  return `${i + 1}º`;
}

/** "4 de 1º, 2 de 2º": the slots of a row in words; '' when there are none. */
export function slotsText(r: RowDraft, pact: boolean): string {
  const parts = r.slots.map((n, i) => (n > 0 ? (pact ? `${n} de ${circleLabel(i)} (pacto)` : `${n} de ${circleLabel(i)}`) : '')).filter((p) => p !== '');
  return parts.join(', ');
}

// ---------------------------------------------------------------------------------------------------------------------
// Placing a refusal: the same two questions for the class and the subclass.

/** Whether `path` is the input of a cell of the grid at `prefix` ("table_class.levels"), a row of it or the grid itself: the cells
 * of the columns drawn (`cols`), and the row and its slots as a whole (their message is shown with the level). */
export function isGridPath(path: string, prefix: string, cols: GridColumns, bonus: boolean): boolean {
  if (path === prefix) return true;
  const m = new RegExp('^' + prefix.replace(/\./g, '\\.') + '\\[(\\d+)\\](?:\\.(prof_bonus|cantrips_known|spells_known|slots)|\\.slots\\[(\\d)\\])?$').exec(path);
  if (!m) return false;
  if (m[2] === undefined && m[3] === undefined) return true;
  switch (m[2]) {
    case 'prof_bonus':
      return bonus;
    case 'cantrips_known':
      return cols.cantrips;
    case 'spells_known':
      return cols.spells;
    case 'slots':
      return true;
    default:
      return Number(m[3]) < cols.circles;
  }
}

/** The feature a refused path is in (the id of the row to open), or '' when the path is not in one. */
export function featureIdOfPath(features: readonly LevelFeature[], baseOf: (index: number) => string, path: string): string {
  const i = features.findIndex((_, k) => {
    const base = baseOf(k);
    return path === base || path.startsWith(base + '.') || path.startsWith(base + '[');
  });
  return i < 0 ? '' : features[i].id;
}

/** The refusals of one always-prepared group: the request lists the spells flat (`always_prepared[3].spell_key`), the screen by
 * class level; a path in the flat list belongs to the group the index falls in. Returns the group's level, or -1. */
export function groupOfAlwaysPrepared(groups: readonly AlwaysPreparedGroup[], path: string): number {
  const m = /^table_subclass\.always_prepared\[(\d+)\]/.exec(path);
  if (!m) return -1;
  let n = Number(m[1]);
  for (const g of groups) {
    if (n < g.spells.length) return g.level;
    n -= g.spells.length;
  }
  return -1;
}
