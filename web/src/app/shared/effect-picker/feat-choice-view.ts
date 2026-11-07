import type { EffectMenuField } from '../../../gen/meurpg/rules/v1/table_content_pb';
import {
  type EffectDraft,
  type EffectMenuVm,
  type FeatChoice,
  defaultChoiceName,
  formulaInsertions,
} from '../../core/content/effect-draft';
import {
  type ReadingPart,
  formulaReading,
  pointsAtChoice,
} from '../../core/content/formula-reading';
import type { SelectOption } from '../form-fields/select-field';
import {
  CHOICE_GROUP_NOTE,
  OPTION_NOTE,
  type RefOption,
  choiceSide,
  fixedGroup,
  pickedRef,
  refKind,
  refOptions,
} from './effect-picker-choices';

/** What the view reads of the picker: its inputs, as the signals themselves. */
export interface FeatChoiceHost {
  readonly effect: () => EffectDraft;
  readonly menu: () => EffectMenuVm;
  readonly choices: () => readonly FeatChoice[] | null;
  readonly effectIndex: () => number;
  readonly featOption: () => boolean;
  readonly siblings: () => readonly EffectDraft[];
}

/**
 * The feat's named choices as one effect of the feat sees them: the group above the fixed values of each field that may point
 * at one, the buttons and the reading of a formula, the name a new choice gets and the lock of "Quantos". Only an effect of the
 * feat itself has choices (`choices` set); an option of the feat has none and says so.
 */
export class FeatChoiceView {
  constructor(private readonly host: FeatChoiceHost) {}

  /** The earlier choices of the feat a field may point at: only its own kind, only from the effects before this one. */
  private refsOf(f: EffectMenuField): RefOption[] {
    const kind = refKind(f.name);
    const all = this.host.choices();
    if (!kind || all === null || this.host.featOption()) {
      return [];
    }
    const before = all.filter((c) => c.kind === kind && c.index < this.host.effectIndex());
    return refOptions(f.name, before, (c) => choiceSide(c, this.damageTypesOf(c)));
  }

  /** The damage types a choice of the feat offers, in lower case ("ácido"); none when it offers them all. */
  private damageTypesOf(c: FeatChoice): string[] {
    const from = this.host.siblings()[c.index]?.from ?? [];
    return from.map((key) => this.host.menu().nameOf('damage_types', key).toLowerCase());
  }

  /** The group of choices above the fixed values; the values alone when the field has no choice to offer. */
  withChoices<T extends SelectOption>(f: EffectMenuField, list: readonly T[]): SelectOption[] {
    const refs = this.refsOf(f);
    return refs.length === 0 ? [...list] : [...refs, ...fixedGroup(list)];
  }

  /** The line a field's value has beside it when it is one of the group's entries. */
  hint(f: EffectMenuField, value: string): string {
    return pickedRef(this.refsOf(f), value)?.hint ?? '';
  }

  /** The line at the foot of a field's list: where the group comes from, or in an option of the feat that it has none. */
  footer(f: EffectMenuField): string {
    if (this.refsOf(f).length > 0) {
      return CHOICE_GROUP_NOTE;
    }
    return this.host.featOption() && refKind(f.name) !== null ? OPTION_NOTE : '';
  }

  /** This effect among the feat's choices, once it has a name. */
  own(): FeatChoice | null {
    const e = this.host.effect();
    return e.type === 'choice' && e.name !== ''
      ? (this.host.choices()?.find((c) => c.index === this.host.effectIndex()) ?? null)
      : null;
  }

  /** A choice with a name picks one: "Quantos" shows 1 and cannot change. */
  countLocked(f: EffectMenuField): boolean {
    const e = this.host.effect();
    return f.name === 'count' && e.type === 'choice' && e.name !== '';
  }

  countNote(): string {
    return this.host.effect().choice === 'damage_type'
      ? 'Um tipo por escolha.'
      : 'Uma por escolha. Para duas, adicione outra escolha.';
  }

  /** The name a choice gets when its kind is picked: "habilidade", then "habilidade_2"; none for a kind that has no name. */
  nameFor(kind: string): string {
    const others = this.host
      .siblings()
      .filter((_, i) => i !== this.host.effectIndex())
      .map((e) => e.name);
    return defaultChoiceName(kind, others);
  }

  /** The earlier ability choices a formula may read, as buttons: "Modificador · Habilidade", "Valor · Habilidade". */
  insertions(f: EffectMenuField) {
    const all = this.host.choices();
    if (all === null || this.host.featOption() || f.kind !== 'formula') {
      return [];
    }
    return formulaInsertions(all.filter((c) => c.index < this.host.effectIndex()));
  }

  /** What a formula says in words, when it points at a choice of the feat. */
  reading(f: EffectMenuField, text: string): ReadingPart[] {
    const all = this.host.choices();
    if (all === null || (f.kind !== 'formula' && f.kind !== 'condition') || !pointsAtChoice(text)) {
      return [];
    }
    const parts = formulaReading(text, all);
    return parts.map((p, i) => ({
      ...p,
      text: i === 0 ? p.text.trimStart() : i === parts.length - 1 ? p.text.trimEnd() : p.text,
    }));
  }

  /** Under "Texto na ficha": where the number of the formula goes, when the text has the place and the formula reads a choice. */
  valueSlot(f: EffectMenuField): string {
    const e = this.host.effect();
    if (f.name !== 'text_pt' || this.host.choices() === null || !e.textPt.includes('{value}')) {
      return '';
    }
    if (!pointsAtChoice(e.value)) {
      return '';
    }
    return /\bCD\b/.test(e.textPt) ? 'é onde o número da CD entra.' : 'é onde o número entra.';
  }
}
