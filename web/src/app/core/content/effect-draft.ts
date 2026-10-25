import type {
  EffectMenuField,
  EffectMenuList,
  EffectMenuType,
  EffectOptionSet,
  GetEffectMenuResponse,
  TableEffect,
} from '../../../gen/meurpg/rules/v1/table_content_pb';
import { feetToMeters, metersToFeet } from '../units';

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
  /** resistance: the damage type it leaves out, and when it holds ("rage"). */
  except: string;
  while: string;
  /** extra_damage: how many dice (a formula) and their sides, the damage type ("" is the weapon's), the weapon kind, how often it
   * applies, and whether a critical hit leaves the dice alone. */
  dice: string;
  diceSides: number;
  damageType: string;
  weapon: string;
  frequency: string;
  noCritDoubling: boolean;
  /** spell_modifier: the spells it fits, and the one change it makes. */
  appliesTo: string;
  spellChange: string;
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
    dice: '',
    diceSides: 0,
    damageType: '',
    weapon: '',
    frequency: '',
    noCritDoubling: false,
    appliesTo: '',
    spellChange: '',
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
    dice: e.dice,
    diceSides: e.diceSides,
    damageType: e.damageType,
    weapon: e.weapon,
    frequency: e.frequency,
    noCritDoubling: e.noCritDoubling,
    appliesTo: e.appliesTo,
    spellChange: e.spellChange,
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
  (key: 'value' | 'when' | 'resource' | 'max' | 'textPt' | 'dice'): EffectWriter =>
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
      | 'damageType'
      | 'weapon'
      | 'frequency'
      | 'appliesTo'
      | 'spellChange',
  ): EffectWriter =>
  (d) =>
    d[key] ? ({ [key]: d[key] } as EffectInit) : {};

/** A list that is sent (as a copy) when it has something in it. */
const pickedList =
  (key: 'targets' | 'from' | 'spells'): EffectWriter =>
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
  ['dice', trimmedText('dice')],
  ['dice_sides', (d) => (d.diceSides > 0 ? { diceSides: d.diceSides } : {})],
  ['damage_type', pickedText('damageType')],
  ['weapon', pickedText('weapon')],
  ['frequency', pickedText('frequency')],
  ['no_crit_doubling', (d) => (d.noCritDoubling ? { noCritDoubling: true } : {})],
  ['applies_to', pickedText('appliesTo')],
  ['spell_change', pickedText('spellChange')],
  ['ignore_cover', (d) => (d.ignoreCover ? { ignoreCover: true } : {})],
  [
    'range_multiplier',
    (d) => (d.rangeMultiplier > 0 ? { rangeMultiplier: d.rangeMultiplier } : {}),
  ],
  [
    'no_long_range_disadvantage',
    (d) => (d.noLongRangeDisadvantage ? { noLongRangeDisadvantage: true } : {}),
  ],
]);

/** The effect a request sends: `type` and the fields the menu says that type reads, and nothing else. An empty text or an empty
 * list is not sent: the server reads "not set" the same way. */
export function draftToEffect(d: EffectDraft, menu: EffectMenuVm): EffectInit {
  const out: EffectInit = { type: d.type };
  for (const f of visibleFields(menu.fieldsOf(d.type), d)) {
    Object.assign(out, EFFECT_WRITERS.get(f.name)?.(d));
  }
  return out;
}

/** What a spell_modifier's change reads besides the fields every change has: the swap lists the types it swaps among, the bonus
 * damage its amount and type, the extra dice their count. The other fields stay out of the editor (and of the request). */
const SPELL_CHANGE_FIELDS: Readonly<Record<string, readonly string[]>> = {
  damage_type_swap: ['from'],
  bonus_damage: ['value', 'damage_type'],
  extra_dice_per_slot_over: ['count'],
};
const SPELL_CHANGE_ONLY: ReadonlySet<string> = new Set(Object.values(SPELL_CHANGE_FIELDS).flat());

/** The fields of the draft's type that apply to it now: a spell_modifier shows only what its chosen change reads (none of the
 * change's own fields until one is chosen); every other type shows all of its menu fields. */
export function visibleFields(
  fields: readonly EffectMenuField[],
  d: EffectDraft,
): readonly EffectMenuField[] {
  if (d.type !== 'spell_modifier') {
    return fields;
  }
  const own = SPELL_CHANGE_FIELDS[d.spellChange] ?? [];
  return fields.filter((f) => !SPELL_CHANGE_ONLY.has(f.name) || own.includes(f.name));
}

/** The labels of the effect fields, in our words (the menu names the fields by their proto name). */
export const EFFECT_FIELD_LABELS: Readonly<Record<string, string>> = {
  target: 'O que muda',
  mode: 'Como muda',
  value: 'Valor',
  when: 'Só quando',
  tags: 'Situação',
  proficiency: 'Proficiência em',
  level: 'Quanto',
  roll: 'Rolagem',
  targets: 'Em quê',
  sense: 'Sentido',
  range_ft: 'Alcance',
  resource: 'Nome do recurso',
  max: 'Usos (máximo)',
  recharge: 'Volta em',
  choice: 'O jogador escolhe',
  count: 'Quantos',
  from: 'Entre',
  economy: 'Tipo de ação',
  applies: 'Onde o dado entra',
  ability: 'Habilidade somada',
  spells: 'Magias',
  text_pt: 'Texto na ficha',
  except: 'Menos o dano de tipo',
  while: 'Enquanto',
  ignore_cover: 'Ignora meia cobertura e três quartos',
  range_multiplier: 'Multiplica o alcance da magia por',
  no_long_range_disadvantage: 'Sem desvantagem no alcance longo',
  dice: 'Dados (quantos)',
  dice_sides: 'Lados do dado',
  damage_type: 'Tipo de dano',
  weapon: 'Arma',
  frequency: 'Quantas vezes',
  no_crit_doubling: 'Não dobra no acerto crítico',
  applies_to: 'Vale para',
  spell_change: 'O que muda na magia',
};

/** A label that reads differently in one type: a spell_modifier's `from` lists the damage types it swaps, its `count` is dice. */
const TYPE_FIELD_LABELS: Readonly<Record<string, Readonly<Record<string, string>>>> = {
  extra_damage: { value: 'Dano fixo' },
  spell_modifier: {
    from: 'Tipos de dano que troca',
    value: 'Dano fixo somado',
    count: 'Dados a mais por nível acima',
    damage_type: 'Tipo do dano somado',
    resource: 'Gasta um uso de',
  },
};

/** The label of a field of a type: the type's own wording when it has one. */
export function effectFieldLabel(type: string, name: string): string {
  return TYPE_FIELD_LABELS[type]?.[name] ?? EFFECT_FIELD_LABELS[name] ?? name;
}

/** One line under a field, where the name alone does not say. This is screen copy; the formula functions and the tag
 * prefixes it mentions come from the server's menu (`EffectMenuVm.formulaHint`, `tagHint`). */
export const EFFECT_FIELD_HINTS: Readonly<Record<string, string>> = {
  resource: 'Só letras minúsculas sem acento, números e _ (de 1 a 40).',
  text_pt: 'O que a ficha mostra. {value} vira o valor com sinal.',
  ignore_cover: 'O alvo não ganha o bônus na CA. A cobertura total continua impedindo o ataque.',
  range_multiplier: 'Só para magias: 2 dobra o alcance, 3 triplica. Vazio, o alcance não muda.',
  no_long_range_disadvantage: 'Só para armas: até o alcance longo, a rolagem não tem desvantagem.',
  dice: 'Uma fórmula, como 1 ou prof(). Vale junto com os lados do dado.',
  dice_sides: 'De 4 a 12: d4, d6, d8, d10 ou d12.',
  no_crit_doubling: 'Por padrão os dados dobram no acerto crítico; o dano fixo nunca dobra.',
};

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

/** The feet of what a range field holds ("18" or "4,5" metres); 0 for an empty or unreadable field. */
export function rangeFeet(text: string): number {
  const n = Number(text.trim().replace(',', '.'));
  return Number.isFinite(n) && n > 0 ? metersToFeet(n) : 0;
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

/** Whether a required closed-list field of the action, maneuver, resistance and spell_modifier types is still empty. */
function choiceEmpty(d: EffectDraft, name: string): boolean {
  return (
    (name === 'economy' && d.economy === '') ||
    (name === 'applies' && d.applies === '') ||
    (name === 'except' && d.except === '') ||
    (name === 'while' && d.while === '') ||
    (name === 'applies_to' && d.appliesTo === '') ||
    (name === 'spell_change' && d.spellChange === '')
  );
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
