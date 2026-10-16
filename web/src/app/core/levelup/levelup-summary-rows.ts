import type { LevelUpMulticlassSummary } from '../../../gen/meurpg/characters/v1/characters_pb';
import type { DerivedSheet } from '../../../gen/meurpg/rules/v1/rules_pb';

/** One line of "O que muda": a label, the value before and after, and a small line under it. */
export interface ChangeRow {
  readonly key: string;
  readonly label: string;
  readonly before: string;
  readonly after: string;
  readonly sub: string;
  /** The number comes from the table's own class ("Da mesa"), not the SRD's. */
  readonly table?: boolean;
}

/** What the summary adds when a class is new to the sheet (SRD 5.1, "Multiclassing"): the multiclass summary the server sent and the names. */
export interface MulticlassContext {
  readonly newClassName: string;
  readonly summary: LevelUpMulticlassSummary | null;
  /** The features of the new class's level 1, by name. */
  readonly newFeatures: readonly string[];
}

/** What the summary needs besides the two sheets: how the hit points were decided and what was picked. */
export interface SummaryContext {
  /** The class gaining the level: a multiclass sheet lists one `spellcasting` entry per casting class, and these rows follow this one. */
  readonly classKey: string;
  /** Whose spell list the known spells are counted on, when it is not the class's own (a table class reusing a list, a third caster). */
  readonly spellListClassKey?: string;
  /** "Média 4 + Constituição +3" / "Rolado 5 + Constituição +3", or '' when it can't be told yet. */
  readonly hpSub: string;
  readonly cantrips: readonly string[];
  readonly spells: readonly string[];
  readonly prepared: readonly string[];
  /** The spells of a spellbook are "Livro de magias"; the others "Magias conhecidas". */
  readonly spellbook: boolean;
  /** How many more spells the level asks for and how many were picked: "(falta 1)". */
  readonly spellsMissing: number;
  /** The class learns spells (a book or a known list) rather than preparing from its list: it shows "Magias conhecidas"
   * or "Livro de magias"; a class that prepares shows only "Magias preparadas". Unset reads as it learns (the old way). */
  readonly learnsSpells?: boolean;
  /** The class gaining the level is the table's own (its key ends in "@mesa"): the slots, which come from its table, say so. */
  readonly table?: boolean;
  /** The features the level gives by themselves, by name ("Estilo de luta", "Conjuração"). */
  readonly newFeatures?: readonly string[];
  /** Set when the level adds a class to the sheet. */
  readonly multiclass?: MulticlassContext;
}

/** "A, B e C" in pt-BR. */
export const LIST = new Intl.ListFormat('pt-BR', { type: 'conjunction' });

/** "Novo: A" / "Novas: A e B". */
export function newNames(names: readonly string[], one: string, many: string): string {
  return names.length === 0 ? '' : `${names.length === 1 ? one : many}: ${LIST.format(names)}`;
}

/** The line for a number that moved, or `null` when the two sides are the same (nothing to show). */
export function row(
  key: string,
  label: string,
  before: string,
  after: string,
  sub = '',
): ChangeRow | null {
  return before === after ? null : { key, label, before, after, sub };
}

/** The spellcasting entry of the class gaining the level (a multiclass sheet lists one per casting class). */
export function cast(sheet: DerivedSheet, classKey: string) {
  return sheet.spellcasting.find((c) => c.classKey === classKey);
}

/** "o" or "a" for a class name, the way the sheet says "o Mago", "a Bruxa"... the SRD names are masculine but for a few. */
export function article(name: string): string {
  return /^(Bruxa|Druida)$/.test(name) ? 'a' : 'o';
}
