import type {
  ClassOptionVm,
  ExtraClassValue,
  RulesCatalogVm,
  SpellOptionVm,
  SpellPreparation,
  SubclassOptionVm,
} from './character-editor.types';

/**
 * The pure half of a character's classes in the editor (MR-025's multiclass at creation, slice
 * 10.12b): one block per class, the total level, the hit die of each level, and which spell lists
 * a sheet reads from. Nothing here is a D&D rule: every table it reads (the highest circle at each
 * level, a subclass's level, the list a class casts from) is the server's catalog, and the
 * prerequisites, the hit points and the slots of a multiclass are the server's too, shown as the
 * sheet's issues and numbers. The browser only partitions what the catalog gives.
 */

/** One class on the sheet as the blocks have it, the first included. */
export interface ClassBlock {
  readonly classKey: string;
  readonly level: number;
  readonly subclassKey: string;
  readonly customSubclassName: string;
}

export function blocksOf(first: ClassBlock, extras: readonly ExtraClassValue[]): ClassBlock[] {
  return [first, ...extras.map((c) => ({ ...c }))];
}

/** The sum of the levels (a level that is not a whole number counts as 0 until it is fixed). */
export function totalLevel(blocks: readonly ClassBlock[]): number {
  return blocks.reduce((sum, b) => sum + (Number.isInteger(b.level) && b.level > 0 ? b.level : 0), 0);
}

/** The most levels a sheet has (the SRD's 20); more is refused by the server. */
export const MAX_TOTAL_LEVEL = 20;

export function classOf(catalog: RulesCatalogVm, key: string): ClassOptionVm | undefined {
  return catalog.classes.find((c) => c.key === key);
}

export function subclassOf(catalog: RulesCatalogVm, classKey: string, subclassKey: string): SubclassOptionVm | undefined {
  return classOf(catalog, classKey)?.subclasses.find((s) => s.key === subclassKey);
}

/** The hit die of every level after the first, in the order of the blocks (every level of the first
 * class, then the next class): the order `HitPoints.rolls` follow. A class not in the catalog counts
 * as a d8 only for the row's label, the server rejects the sheet anyway. */
export function hitDiceAfterFirst(catalog: RulesCatalogVm, blocks: readonly ClassBlock[]): number[] {
  const dice: number[] = [];
  blocks.forEach((b, i) => {
    const die = classOf(catalog, b.classKey)?.hitDie ?? 0;
    const levels = Number.isInteger(b.level) && b.level > 0 ? b.level : 0;
    for (let n = 1; n <= levels; n++) {
      if (i === 0 && n === 1) {
        continue;
      }
      dice.push(die);
    }
  });
  return dice;
}

/** The spell lists one block reads from: its class when that casts, else its subclass when that is a third caster. */
export interface CasterSection {
  /** The block's index: stable for the section's id and its filters. */
  readonly index: number;
  readonly classKey: string;
  /** "Mago", or "Guerreiro" for a third caster's subclass. */
  readonly namePt: string;
  /** The subclass the list belongs to, for a third caster. */
  readonly subclassNamePt: string;
  readonly level: number;
  /** The class whose spell list this section reads. */
  readonly listClassKey: string;
  readonly preparation: SpellPreparation | null;
  /** The highest circle at this class level, `null` when the table is missing (nothing is hidden). */
  readonly maxCircle: number | null;
  readonly firstLevel: number;
}

function circleAt(table: readonly number[], level: number): number | null {
  if (table.length === 0) {
    return null;
  }
  const at = Math.min(Math.max(Math.trunc(level) || 1, 1), table.length);
  return table[at - 1];
}

/** The casting sections of a sheet, in block order: a class that casts, or a subclass that casts from its level on. */
export function casterSections(catalog: RulesCatalogVm, blocks: readonly ClassBlock[]): CasterSection[] {
  const out: CasterSection[] = [];
  blocks.forEach((b, index) => {
    const cls = classOf(catalog, b.classKey);
    if (!cls) {
      return;
    }
    if (cls.isCaster) {
      out.push({
        index,
        classKey: cls.key,
        namePt: cls.namePt,
        subclassNamePt: '',
        level: b.level,
        listClassKey: cls.spellListClassKey || cls.key,
        preparation: cls.preparation,
        maxCircle: circleAt(cls.maxSpellLevelByLevel, b.level),
        firstLevel: cls.spellcastingFirstLevel,
      });
      return;
    }
    const sub = b.subclassKey ? cls.subclasses.find((s) => s.key === b.subclassKey) : undefined;
    if (sub?.casting && b.level >= sub.casting.firstLevel) {
      out.push({
        index,
        classKey: cls.key,
        namePt: cls.namePt,
        subclassNamePt: sub.namePt,
        level: b.level,
        listClassKey: sub.casting.listClassKey,
        preparation: sub.casting.preparation,
        maxCircle: circleAt(sub.casting.maxSpellLevelByLevel, b.level),
        firstLevel: sub.casting.firstLevel,
      });
    }
  });
  return out;
}

/** "Mago", or "Guerreiro (Lâmina de Tinta)" for a third caster. */
export function sectionName(s: CasterSection): string {
  return s.subclassNamePt ? `${s.namePt} (${s.subclassNamePt})` : s.namePt;
}

/** The cantrips of a section's list, sorted by Portuguese name. */
export function cantripsOf(spells: readonly SpellOptionVm[], s: CasterSection): SpellOptionVm[] {
  return sorted(spells.filter((sp) => sp.level === 0 && sp.classKeys.includes(s.listClassKey)));
}

/** The leveled spells of a section's list: circle first, then name. `selected` stays listed above the
 * limit (the level was lowered) so it can be unchecked instead of vanishing. */
export function leveledOf(spells: readonly SpellOptionVm[], s: CasterSection, selected: ReadonlySet<string> = new Set()): SpellOptionVm[] {
  return sorted(
    spells.filter(
      (sp) =>
        sp.level >= 1 &&
        sp.classKeys.includes(s.listClassKey) &&
        (s.maxCircle === null || sp.level <= s.maxCircle || selected.has(sp.key)),
    ),
  );
}

/** Every spell of the lists above, whatever the circle: what a save may send. */
export function allOfSection(spells: readonly SpellOptionVm[], s: CasterSection): SpellOptionVm[] {
  return sorted(spells.filter((sp) => sp.classKeys.includes(s.listClassKey)));
}

function sorted(spells: SpellOptionVm[]): SpellOptionVm[] {
  return spells.sort((a, b) => a.level - b.level || a.namePt.localeCompare(b.namePt, 'pt-BR'));
}

/** A spell the search finds that no list of the sheet has, and the classes that do have it. */
export interface OutsideSpell {
  readonly spell: SpellOptionVm;
  /** "Bardo, Druida e Patrulheiro". */
  readonly classes: string;
}

const LIST = new Intl.ListFormat('pt-BR', { type: 'conjunction' });

/** What a search finds outside the sheet's lists: the spells that match `query` (a cantrip or a leveled
 * one, as asked) that none of the sections lists, with the classes that do (at most 6 rows). */
export function outsideTheLists(
  catalog: RulesCatalogVm,
  sections: readonly CasterSection[],
  query: string,
  cantrips: boolean,
): OutsideSpell[] {
  const q = query.trim().toLowerCase();
  if (q === '' || sections.length === 0) {
    return [];
  }
  const lists = new Set(sections.map((s) => s.listClassKey));
  const names = new Map(catalog.classes.map((c) => [c.key, c.namePt]));
  return catalog.spells
    .filter((sp) => (cantrips ? sp.level === 0 : sp.level >= 1) && sp.namePt.toLowerCase().includes(q) && !sp.classKeys.some((k) => lists.has(k)))
    .slice(0, 6)
    .map((spell) => ({ spell, classes: LIST.format(spell.classKeys.map((k) => names.get(k) ?? k)) }));
}
