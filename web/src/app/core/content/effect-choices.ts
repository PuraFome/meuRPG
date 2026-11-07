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
