import type {
  EffectMenuField,
  EffectMenuList,
  EffectMenuType,
  EffectOptionSet,
  GetEffectMenuResponse,
  TableEffect,
} from '../../../gen/meurpg/rules/v1/table_content_pb';
import { rangeFeet, rangeMeters } from './range-text';

export {
  rangeFeet,
  rangeMeters,
  strictRangeFeet,
  unreadableRange,
  UNREADABLE_RANGE,
} from './range-text';

/**
 * A feature's effect as the editor holds it (MR-025, ADR-0018, section 4), and the menu it is chosen from. The menu is the
 * server's (`GetEffectMenu`): the types, the fields each one reads, the closed lists with their Portuguese names. Nothing
 * here lists a target, a sense or a recharge; whatever the menu offers, the server accepts. What goes to the server is
 * only the fields the chosen type reads (a field of another type is refused as `bad_value`), so changing the type drops
 * what the old one had.
 */

/** The fields of `TableEffect`, as plain values: strings for text, arrays for lists, numbers for counts and ranges. */
export interface EffectDraft {
  /** A stable id the editor tracks the row by (never sent): focus and "Mais opções" follow a moved item. */
  id: string;
  type: string;
  target: string;
  mode: string;
  value: string;
  when: string;
  /** Written as "against:magic, about:items": split into tags when sent. */
  tags: string;
  proficiency: string;
  level: string;
  roll: string;
  targets: string[];
  sense: string;
  /** The range as typed, in metres (the server stores feet): kept as text so the field never rewrites what is being typed. */
  rangeM: string;
  resource: string;
  max: string;
  recharge: string;
  choice: string;
  count: number;
  from: string[];
  economy: string;
  /** superiority_die: where the die goes, and the ability whose modifier a reduction adds. ranged_attack: "spell" or "weapon". */
  applies: string;
  ability: string;
  spells: string[];
  textPt: string;
  /** ranged_attack's switches: past half and three-quarters cover, the spell range times 2 or 3 (0 is none), no disadvantage at long range. */
  ignoreCover: boolean;
  rangeMultiplier: number;
  noLongRangeDisadvantage: boolean;
  /** resistance: the damage type it leaves out, and when it holds ("rage" or a state of the entry). The other effects that
   * hold only while a state is on (roll_mode, extra_damage, weapon_rule, pseudo_weapon) carry the same `while`: optional there. */
  except: string;
  while: string;
  /** state: its name in the entry, the formula of its duration in rounds (empty: until it ends), and how else it ends. */
  stateName: string;
  durationRounds: string;
  endsOn: string[];
  /** weapon_rule: which weapons (and the name of "named"), another ability for the attack and for the damage, bonuses (signed),
   * and a melee reach in metres as typed. pseudo_weapon: its kind in `weapon`, `attackName`, `damageDice`, `damageType`, the
   * ability in `ability`, the reach in `reachM` and the range in `rangeM`. */
  weapon: string;
  weaponName: string;
  attackAbility: string;
  damageAbility: string;
  attackBonus: number;
  damageBonus: number;
  reachM: string;
  attackName: string;
  damageDice: string;
  damageType: string;
  /** proficiency, level "expertise" on a skill: doubles it only where the character is already proficient. */
  onlyProficient: boolean;
}

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
    stateName: '',
    durationRounds: '',
    endsOn: [],
    weapon: '',
    weaponName: '',
    attackAbility: '',
    damageAbility: '',
    attackBonus: 0,
    damageBonus: 0,
    reachM: '',
    attackName: '',
    damageDice: '',
    damageType: '',
    onlyProficient: false,
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
    stateName: e.stateName,
    durationRounds: e.durationRounds,
    endsOn: [...e.endsOn],
    weapon: e.weapon,
    weaponName: e.weaponName,
    attackAbility: e.attackAbility,
    damageAbility: e.damageAbility,
    attackBonus: e.attackBonus,
    damageBonus: e.damageBonus,
    reachM: rangeMeters(e.reachFt),
    attackName: e.attackName,
    damageDice: e.damageDice,
    damageType: e.damageType,
    onlyProficient: e.onlyProficient,
  };
}

/** The fields a request carries for one effect: the type, and only what that type reads. */
export type EffectInit = Partial<Omit<TableEffect, '$typeName' | '$unknown'>>;

/** The tags of "against:magic, about:items": trimmed, empty ones dropped. */
export function splitTags(text: string): string[] {
  return text
    .split(',')
    .map((t) => t.trim())
    .filter((t) => t !== '');
}

type EffectWriter = (d: EffectDraft) => EffectInit;

/** A text the server reads trimmed; an empty one is not sent. */
const trimmedText =
  (
    key:
      | 'value'
      | 'when'
      | 'resource'
      | 'max'
      | 'textPt'
      | 'stateName'
      | 'durationRounds'
      | 'weaponName'
      | 'attackName'
      | 'damageDice',
  ): EffectWriter =>
  (d) => {
    const text = d[key].trim();
    return text ? ({ [key]: text } as EffectInit) : {};
  };

/** A choice that is sent when something is picked. */
const pickedText =
  (
    key:
      | 'target'
      | 'mode'
      | 'proficiency'
      | 'level'
      | 'roll'
      | 'sense'
      | 'recharge'
      | 'choice'
      | 'economy'
      | 'applies'
      | 'ability'
      | 'except'
      | 'while'
      | 'weapon'
      | 'attackAbility'
      | 'damageAbility'
      | 'damageType',
  ): EffectWriter =>
  (d) =>
    d[key] ? ({ [key]: d[key] } as EffectInit) : {};

/** A list that is sent (as a copy) when it has something in it. */
const pickedList =
  (key: 'targets' | 'from' | 'spells' | 'endsOn'): EffectWriter =>
  (d) =>
    d[key].length > 0 ? ({ [key]: [...d[key]] } as EffectInit) : {};

/** How each field the menu names goes into the request, by the field's proto name. ranged_attack's switches and its multiplier
 * are only sent when on (or set). A name the table does not know writes nothing. */
const EFFECT_WRITERS: ReadonlyMap<string, EffectWriter> = new Map<string, EffectWriter>([
  ['target', pickedText('target')],
  ['mode', pickedText('mode')],
  ['value', trimmedText('value')],
  ['when', trimmedText('when')],
  [
    'tags',
    (d) => {
      const tags = splitTags(d.tags);
      return tags.length > 0 ? { tags } : {};
    },
  ],
  ['proficiency', pickedText('proficiency')],
  ['level', pickedText('level')],
  ['roll', pickedText('roll')],
  ['targets', pickedList('targets')],
  ['sense', pickedText('sense')],
  ['range_ft', (d) => (rangeFeet(d.rangeM) > 0 ? { rangeFt: rangeFeet(d.rangeM) } : {})],
  ['resource', trimmedText('resource')],
  ['max', trimmedText('max')],
  ['recharge', pickedText('recharge')],
  ['choice', pickedText('choice')],
  ['count', (d) => (d.count > 0 ? { count: d.count } : {})],
  ['from', pickedList('from')],
  ['economy', pickedText('economy')],
  ['applies', pickedText('applies')],
  ['ability', pickedText('ability')],
  ['spells', pickedList('spells')],
  ['text_pt', trimmedText('textPt')],
  ['except', pickedText('except')],
  ['while', pickedText('while')],
  ['state_name', trimmedText('stateName')],
  ['duration_rounds', trimmedText('durationRounds')],
  ['ends_on', pickedList('endsOn')],
  ['weapon', pickedText('weapon')],
  ['weapon_name', trimmedText('weaponName')],
  ['attack_ability', pickedText('attackAbility')],
  ['damage_ability', pickedText('damageAbility')],
  ['attack_bonus', (d) => (d.attackBonus !== 0 ? { attackBonus: d.attackBonus } : {})],
  ['damage_bonus', (d) => (d.damageBonus !== 0 ? { damageBonus: d.damageBonus } : {})],
  ['reach_ft', (d) => (rangeFeet(d.reachM) > 0 ? { reachFt: rangeFeet(d.reachM) } : {})],
  ['attack_name', trimmedText('attackName')],
  ['damage_dice', trimmedText('damageDice')],
  ['damage_type', pickedText('damageType')],
  ['ignore_cover', (d) => (d.ignoreCover ? { ignoreCover: true } : {})],
  [
    'only_proficient',
    (d) => (onlyProficientApplies(d) && d.onlyProficient ? { onlyProficient: true } : {}),
  ],
  [
    'range_multiplier',
    (d) => (d.rangeMultiplier > 0 ? { rangeMultiplier: d.rangeMultiplier } : {}),
  ],
  [
    'no_long_range_disadvantage',
    (d) => (d.noLongRangeDisadvantage ? { noLongRangeDisadvantage: true } : {}),
  ],
]);

/** Whether "Só onde já tem proficiência" means something for the effect: it doubles a skill, so it goes with the level
 * "Especialização" on a skill ("skill:*" is every skill). The editor shows it only then, and the request sends it only then. */
export function onlyProficientApplies(d: Pick<EffectDraft, 'level' | 'proficiency'>): boolean {
  return d.level === 'expertise' && d.proficiency.startsWith('skill:');
}

/** The effect a request sends: `type` and the fields the menu says that type reads, and nothing else. An empty text or an empty
 * list is not sent: the server reads "not set" the same way. */
export function draftToEffect(d: EffectDraft, menu: EffectMenuVm): EffectInit {
  const out: EffectInit = { type: d.type };
  for (const f of menu.fieldsOf(d.type)) {
    Object.assign(out, EFFECT_WRITERS.get(f.name)?.(d));
  }
  return out;
}

export { EFFECT_FIELD_HINTS, EFFECT_FIELD_LABELS } from './effect-labels';

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
    // active("nome") reads a state and is only for a condition ("Só quando"): a number never has it.
    const calls = this.helpers.map((h) => h.call).filter((c) => !c.startsWith('active('));
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
