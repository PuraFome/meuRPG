import type {
  EffectMenuField,
  EffectMenuList,
  EffectMenuType,
  EffectOptionSet,
  GetEffectMenuResponse,
  TableEffect,
} from '../../../gen/meurpg/rules/v1/table_content_pb';
import { feetToMeters } from '../units';
import type { EffectDraft, EffectInit } from './effect-draft.types';
import { visibleFields } from './effect-visible';
import { EFFECT_WRITERS, rangeFeet } from './effect-writers';

export type { EffectDraft, EffectInit } from './effect-draft.types';
export { onlyProficientApplies, rangeFeet, splitTags } from './effect-writers';

/**
 * A feature's effect as the editor holds it (MR-025, ADR-0018, section 4), and the menu it is chosen from. The menu is the
 * server's (`GetEffectMenu`): the types, the fields each one reads, the closed lists with their Portuguese names. Nothing
 * here lists a target, a sense or a recharge; whatever the menu offers, the server accepts. What goes to the server is
 * only the fields the chosen type reads (a field of another type is refused as `bad_value`), so changing the type drops
 * what the old one had.
 */

let nextId = 0;

/** A new row id, for effects and features. */
export function newRowId(): string {
  return `row-${nextId++}`;
}

export function emptyEffect(type = ''): EffectDraft {
  return {
    id: newRowId(),
    type,
    target: '',
    mode: '',
    value: '',
    when: '',
    tags: '',
    proficiency: '',
    level: '',
    roll: '',
    targets: [],
    sense: '',
    rangeM: '',
    resource: '',
    max: '',
    recharge: '',
    choice: '',
    count: 0,
    from: [],
    economy: '',
    applies: '',
    ability: '',
    spells: [],
    textPt: '',
    ignoreCover: false,
    rangeMultiplier: 0,
    noLongRangeDisadvantage: false,
    except: '',
    while: '',
    cost: '',
    costMax: '',
    valuePerPoint: '',
    heal: '',
    tempHp: '',
    event: '',
    restore: '',
    healPercentMax: 0,
    onlyProficient: false,
    spell: '',
    fromLevel: '',
    uses: '',
    alsoWithSlot: false,
    ritualOnly: false,
    countsAsKnown: false,
    dice: '',
    diceSides: 0,
    damageType: '',
    weapon: '',
    frequency: '',
    noCritDoubling: false,
    appliesTo: '',
    spellChange: '',
    damageTypes: [],
    kind: '',
    conditions: [],
    againstCondition: '',
  };
}

/** A stored effect, as a draft. */
export function effectToDraft(e: TableEffect): EffectDraft {
  return {
    id: newRowId(),
    type: e.type,
    target: e.target,
    mode: e.mode,
    value: e.value,
    when: e.when,
    tags: e.tags.join(', '),
    proficiency: e.proficiency,
    level: e.level,
    roll: e.roll,
    targets: [...e.targets],
    sense: e.sense,
    rangeM: rangeMeters(e.rangeFt),
    resource: e.resource,
    max: e.max,
    recharge: e.recharge,
    choice: e.choice,
    count: e.count,
    from: [...e.from],
    economy: e.economy,
    applies: e.applies,
    ability: e.ability,
    spells: [...e.spells],
    textPt: e.textPt,
    ignoreCover: e.ignoreCover,
    rangeMultiplier: e.rangeMultiplier,
    noLongRangeDisadvantage: e.noLongRangeDisadvantage,
    except: e.except,
    while: e.while,
    cost: e.cost,
    costMax: e.costMax,
    valuePerPoint: e.valuePerPoint,
    heal: e.heal,
    tempHp: e.tempHp,
    event: e.event,
    restore: e.restore,
    healPercentMax: e.healPercentMax,
    onlyProficient: e.onlyProficient,
    spell: e.spell,
    fromLevel: e.fromLevel,
    uses: e.uses,
    alsoWithSlot: e.alsoWithSlot,
    ritualOnly: e.ritualOnly,
    countsAsKnown: e.countsAsKnown,
    dice: e.dice,
    diceSides: e.diceSides,
    damageType: e.damageType,
    weapon: e.weapon,
    frequency: e.frequency,
    noCritDoubling: e.noCritDoubling,
    appliesTo: e.appliesTo,
    spellChange: e.spellChange,
    damageTypes: [...e.damageTypes],
    kind: e.kind,
    conditions: [...e.conditions],
    againstCondition: e.againstCondition,
  };
}

/** The effect a request sends: `type` and the fields the menu says that type reads, and nothing else. An empty text or an empty
 * list is not sent: the server reads "not set" the same way. */
export function draftToEffect(d: EffectDraft, menu: EffectMenuVm): EffectInit {
  const out: EffectInit = { type: d.type };
  for (const f of visibleFields(menu.fieldsOf(d.type), d)) {
    Object.assign(out, EFFECT_WRITERS.get(f.name)?.(d));
  }
  return out;
}

/** A value of a closed list: the key the effect stores and the Portuguese name. */
export interface MenuOption {
  readonly key: string;
  readonly namePt: string;
  readonly hintPt: string;
}

/** The menu the pickers read, from `GetEffectMenu`. */
export class EffectMenuVm {
  readonly types: readonly EffectMenuType[];
  readonly lists: ReadonlyMap<string, readonly MenuOption[]>;
  readonly optionSets: readonly EffectOptionSet[];
  readonly classIndexes: readonly string[];
  readonly helpers: GetEffectMenuResponse['helpers'];
  readonly maxFeatures: number;
  readonly maxEffects: number;
  readonly maxTags: number;
  /** The Portuguese name of a class key, set by the page once it has the catalog. */
  classNamePt: (key: string) => string = (key) => key.replace(/^class:/, '');
  /** The feats a "feat" choice may list in `from` (the table's own, set by the page from the entries); the SRD's are not in the catalog. */
  featOptions: readonly MenuOption[] = [];

  constructor(res: GetEffectMenuResponse) {
    this.types = res.types;
    this.lists = new Map(res.lists.map((l: EffectMenuList) => [l.name, l.values]));
    this.optionSets = res.optionSets;
    this.classIndexes = res.classIndexes;
    this.helpers = res.helpers;
    this.maxFeatures = res.maxFeaturesPerClass;
    this.maxEffects = res.maxEffectsPerFeature;
    this.maxTags = res.maxTagsPerEffect;
  }

  /** "Um número ou uma fórmula com: mod("<habilidade>"), prof()…": the functions are the server's (`helpers`). */
  formulaHint(): string {
    const calls = this.helpers.map((h) => h.call);
    return calls.length > 0
      ? `Um número ou uma fórmula. Funções: ${calls.join(', ')}.`
      : 'Um número ou uma fórmula.';
  }

  /** The prefixes a tag starts with, from the menu's `tag_prefixes`: "against:… (Contra…)". */
  tagHint(): string {
    const prefixes = this.list('tag_prefixes').map((p) => `${p.key}…`);
    return prefixes.length > 0
      ? `Separe por vírgula, começando por ${prefixes.join(' ou ')}. O app só lembra; o mestre decide.`
      : 'Separe por vírgula. O app só lembra; o mestre decide.';
  }

  /** The types an editor offers: the ones only a feat has (`feat_only`) are left out unless it is the feat editor. */
  typesFor(featEditor: boolean): readonly EffectMenuType[] {
    return featEditor ? this.types : this.types.filter((t) => !t.featOnly);
  }

  typeOf(type: string): EffectMenuType | undefined {
    return this.types.find((t) => t.type === type);
  }

  fieldsOf(type: string): readonly EffectMenuField[] {
    return this.typeOf(type)?.fields ?? [];
  }

  /** The closed list a field points at. */
  list(name: string): readonly MenuOption[] {
    return this.lists.get(name) ?? [];
  }

  /** What a `choice` effect's `from` may hold, by the kind of choice: skills, languages, tools, an option set's options or a
   * spell list's class. Empty for a kind with no list ("any of the kind"). */
  fromOptions(choice: string): readonly MenuOption[] {
    switch (choice) {
      case 'skill':
      case 'expertise':
        return this.list('skills');
      case 'language':
        return this.list('languages');
      case 'tool':
        return this.list('tools');
      case 'cantrip':
      case 'spell':
        return this.classIndexes.map((i) => ({
          key: `class:${i}`,
          namePt: this.classNamePt(`class:${i}`),
          hintPt: '',
        }));
      case 'feat':
        return this.featOptions;
      case 'feature':
        return this.optionSets.map((s) => ({ key: s.key, namePt: s.namePt, hintPt: '' }));
      default:
        return [];
    }
  }

  /** The Portuguese name of a value of a list (the key when the menu does not know it). */
  nameOf(list: string, key: string): string {
    return this.list(list).find((v) => v.key === key)?.namePt ?? key;
  }
}

/** The metres a range field shows, from the feet it stores. */
export function rangeMeters(ft: number): string {
  return ft > 0 ? String(feetToMeters(ft)).replace('.', ',') : '';
}

/** Whether a range field holds something that is not a number of metres (empty is fine: it means none). */
export function unreadableRange(text: string): boolean {
  const t = text.trim();
  if (t === '') {
    return false;
  }
  const n = Number(t.replace(',', '.'));
  return !Number.isFinite(n) || n < 0;
}

/** What the editor says of a range field it cannot read. */
export const UNREADABLE_RANGE = 'Escreva só o número de metros, como 18 ou 4,5.';

/** Like rangeFeet, but unreadable text is NaN instead of 0, so a field where 0 is valid cannot save it by accident. */
export function strictRangeFeet(text: string): number {
  return unreadableRange(text) ? Number.NaN : rangeFeet(text);
}

/** The required closed-list fields of the action, maneuver, resistance, spell and condition types, by name: whether each is still empty. */
const CHOICE_EMPTY: Readonly<Record<string, (d: EffectDraft) => boolean>> = {
  event: (d) => d.event === '',
  economy: (d) => d.economy === '',
  applies: (d) => d.applies === '',
  except: (d) => d.except === '',
  while: (d) => d.while === '',
  spell: (d) => d.spell === '',
  ability: (d) => d.ability === '',
  conditions: (d) => d.conditions.length === 0,
  applies_to: (d) => d.appliesTo === '',
  spell_change: (d) => d.spellChange === '',
};

/** Whether a required closed-list field is still empty. */
function choiceEmpty(d: EffectDraft, name: string): boolean {
  return CHOICE_EMPTY[name]?.(d) ?? false;
}

/** A required field of the type that still has nothing: the editor says so before the server does. */
export function missingRequired(d: EffectDraft, menu: EffectMenuVm): string[] {
  const out: string[] = [];
  for (const f of visibleFields(menu.fieldsOf(d.type), d)) {
    if (!f.required) {
      continue;
    }
    const empty = ((): boolean => {
      switch (f.name) {
        case 'target':
          return d.target === '';
        case 'mode':
          return d.mode === '';
        case 'value':
          return d.value.trim() === '';
        case 'proficiency':
          return d.proficiency === '';
        case 'roll':
          return d.roll === '';
        case 'targets':
          return d.targets.length === 0;
        case 'sense':
          return d.sense === '';
        case 'range_ft':
          return rangeFeet(d.rangeM) <= 0;
        case 'resource':
          return d.resource.trim() === '';
        case 'max':
          return d.max.trim() === '';
        case 'recharge':
          return d.recharge === '';
        case 'choice':
          return d.choice === '';
        case 'count':
          return d.count <= 0;
        default:
          return choiceEmpty(d, f.name);
      }
    })();
    if (empty) {
      out.push(f.name);
    }
  }
  return out;
}
