import type { EffectMenuField } from '../../../gen/meurpg/rules/v1/table_content_pb';
import type { EffectDraft } from './effect-draft.types';
import { visibleFields } from './effect-visible';

/**
 * The named choices of a feat (RN-23, MR-025): a `choice` effect of kind ability or damage type with a `name`. Later effects
 * of the same feat write `@name` for the pick. The rules are the server's (`rules/picks.go`, `rules/feat_choices.go`).
 */

export type ChoiceKind = 'ability' | 'damage_type';

export interface FeatChoice {
  kind: ChoiceKind;
  /** The master's name, the part after the "@". */
  name: string;
  /** The place of this choice among the feat's choices of its kind, from 1. */
  number: number;
  /** How many choices of its kind the feat has. */
  ofKind: number;
  /** "Habilidade" or "Tipo de dano", numbered "(1ª)" or "(1º)" only when the feat has two or more of the kind. */
  label: string;
  /** Where the choice is in the feat's effects. */
  index: number;
}

export interface FormulaInsertion {
  /** `mod("@name")` or `score("@name")`. */
  text: string;
  /** "Modificador · Habilidade (1ª)". */
  label: string;
}

export interface MissingChoice {
  /** The effect that holds the reference. */
  index: number;
  /** The field's path, "table_feat.effects[2].proficiency". */
  path: string;
  /** The name the field points at, with no "@". */
  name: string;
}

const KIND_TITLE: Readonly<Record<ChoiceKind, { title: string; ordinal: string }>> = {
  ability: { title: 'Habilidade', ordinal: 'ª' },
  damage_type: { title: 'Tipo de dano', ordinal: 'º' },
};

function kindOf(e: EffectDraft): ChoiceKind | null {
  const named = e.type === 'choice' && e.name !== '';
  return named && (e.choice === 'ability' || e.choice === 'damage_type') ? e.choice : null;
}

/** The feat's named choices in the order of its effects, numbered by kind as the server numbers the level-up questions. */
export function featChoices(effects: readonly EffectDraft[]): FeatChoice[] {
  const found: { kind: ChoiceKind; name: string; index: number }[] = [];
  effects.forEach((e, index) => {
    const kind = kindOf(e);
    if (kind) {
      found.push({ kind, name: e.name, index });
    }
  });
  return found.map((c, at) => {
    const ofKind = found.filter((o) => o.kind === c.kind).length;
    const number = found.slice(0, at + 1).filter((o) => o.kind === c.kind).length;
    const { title, ordinal } = KIND_TITLE[c.kind];
    return { ...c, number, ofKind, label: ofKind > 1 ? `${title} (${number}${ordinal})` : title };
  });
}

/** The choices of `kind` that come before the effect at `index`: the only ones its field may point at. */
export function choicesBefore(
  effects: readonly EffectDraft[],
  index: number,
  kind: ChoiceKind,
): FeatChoice[] {
  return featChoices(effects).filter((c) => c.kind === kind && c.index < index);
}

const DEFAULT_NAME: Readonly<Record<ChoiceKind, string>> = {
  ability: 'habilidade',
  damage_type: 'tipo_de_dano',
};

/** The name the editor gives a new choice of `kind` ("habilidade", then "habilidade_2"), so the master never types one; '' for a kind that has no name. */
export function defaultChoiceName(kind: string, taken: readonly string[]): string {
  const base = DEFAULT_NAME[kind as ChoiceKind];
  if (!base) {
    return '';
  }
  let name = base;
  for (let n = 2; taken.includes(name); n++) {
    name = `${base}_${n}`;
  }
  return name;
}

/** What a field writes to point at a choice. */
export function choiceRef(choice: Pick<FeatChoice, 'name'>): string {
  return `@${choice.name}`;
}

/** What a formula may read of a choice: only an ability's modifier or score (the server's `mod` and `score`). */
export function formulaInsertions(choices: readonly FeatChoice[]): FormulaInsertion[] {
  return choices
    .filter((c) => c.kind === 'ability')
    .flatMap((c) => [
      { text: `mod("${choiceRef(c)}")`, label: `Modificador · ${c.label}` },
      { text: `score("${choiceRef(c)}")`, label: `Valor · ${c.label}` },
    ]);
}

/** The fields that hold a reference to a choice (`pickFields` of the server), by the proto name; the formulas take abilities only. */
const REFERENCE_FIELDS: readonly { name: string; key: keyof EffectDraft }[] = [
  { name: 'proficiency', key: 'proficiency' },
  { name: 'target', key: 'target' },
  { name: 'targets', key: 'targets' },
  { name: 'ability', key: 'ability' },
  { name: 'attack_ability', key: 'attackAbility' },
  { name: 'damage_ability', key: 'damageAbility' },
  { name: 'damage_type', key: 'damageType' },
  { name: 'damage_types', key: 'damageTypes' },
  { name: 'except', key: 'except' },
  { name: 'value', key: 'value' },
  { name: 'when', key: 'when' },
  { name: 'max', key: 'max' },
  { name: 'dice', key: 'dice' },
  { name: 'duration_rounds', key: 'durationRounds' },
  { name: 'from_level', key: 'fromLevel' },
  { name: 'cost', key: 'cost' },
  { name: 'cost_max', key: 'costMax' },
  { name: 'temp_hp', key: 'tempHp' },
  { name: 'value_per_point', key: 'valuePerPoint' },
];

/** The names a text points at with `@name`, once each; the table suffix of a key ("feature:x@mesa") is no reference. */
function referencesIn(text: string): string[] {
  const names = [...text.matchAll(/@([a-z0-9_]+)/g)].map((m) => m[1]).filter((n) => n !== 'mesa');
  return [...new Set(names)];
}

function textsOf(e: EffectDraft, key: keyof EffectDraft): string[] {
  const v = e[key];
  if (typeof v === 'string') {
    return [v];
  }
  return Array.isArray(v) ? v.filter((s): s is string => typeof s === 'string') : [];
}

/**
 * Every field that points at a choice no earlier effect declares, with the name it lacks. A choice that was removed, renamed or
 * moved below its field leaves such a field; nothing is re-pointed, and the save waits until the master fixes each one. With
 * `visible` given, only the fields the effect's type reads count (a field the type does not read is not sent).
 */
export function missingChoices(
  effects: readonly EffectDraft[],
  visible?: { fieldsOf(type: string): readonly EffectMenuField[] },
  base = 'table_feat.effects',
): MissingChoice[] {
  const out: MissingChoice[] = [];
  const declared = new Set<string>();
  effects.forEach((e, index) => {
    const shown = visible
      ? new Set(visibleFields(visible.fieldsOf(e.type), e).map((f) => f.name))
      : null;
    for (const f of REFERENCE_FIELDS) {
      if (shown && !shown.has(f.name)) {
        continue;
      }
      const names = textsOf(e, f.key).flatMap(referencesIn);
      for (const name of new Set(names)) {
        if (!declared.has(name)) {
          out.push({ index, path: `${base}[${index}].${f.name}`, name });
        }
      }
    }
    if (kindOf(e)) {
      declared.add(e.name);
    }
  });
  return out;
}

/** The fields of an effect that point at a name, by the proto name, with the draft's key. */
function referencesOfField(e: EffectDraft, field: string): string[] {
  const own = REFERENCE_FIELDS.find((f) => f.name === field);
  return own ? [...new Set(textsOf(e, own.key).flatMap(referencesIn))] : [];
}

/**
 * Why a field's pick cannot be read: `removed` when no effect of the feat declares the name any more, `later` when the choice
 * is declared at this effect or below it. Null when every name it holds is a choice of an earlier effect (or it holds none).
 */
export function referenceProblem(
  e: EffectDraft,
  field: string,
  choices: readonly FeatChoice[],
  index: number,
): 'removed' | 'later' | null {
  let later = false;
  for (const name of referencesOfField(e, field)) {
    const known = choices.filter((c) => c.name === name);
    if (known.some((c) => c.index < index)) {
      continue;
    }
    if (known.length === 0) {
      return 'removed';
    }
    later = true;
  }
  return later ? 'later' : null;
}

export interface ChoiceUse {
  choice: FeatChoice;
  /** How many later effects have a field that points at the choice. */
  effects: number;
}

/** Each named choice of the feat with the number of effects that use it, in the order of the feat's effects. */
export function choiceUses(effects: readonly EffectDraft[]): ChoiceUse[] {
  return featChoices(effects).map((choice) => {
    const used = effects.filter(
      (e, index) =>
        index > choice.index &&
        REFERENCE_FIELDS.some((f) => referencesOfField(e, f.name).includes(choice.name)),
    );
    return { choice, effects: used.length };
  });
}
