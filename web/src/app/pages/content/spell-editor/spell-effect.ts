import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { CONDITIONS } from '../../../core/combat/conditions';
import {
  AURA_REFUSED,
  AURA_REFUSED_PT,
  type AuraAffectsChoice,
  DEFAULT_AURA,
  EFFECT_KINDS,
  type EffectApplyChoice,
  type EffectKind,
  type EffectRow,
  MAX_EFFECT_ITEMS,
  MAX_LABEL,
  type RollWord,
  type SpellEffectDraft,
  emptyRow,
  modifierIndexOf,
  rowToModifiers,
} from '../../../core/content/spell-effect-draft';
import { auraTag, rowTags } from '../../../core/content/spell-effect-tags';
import { latestInput } from '../../../shared/latest-input';
import { CheckGroup } from '../../../shared/form-fields/check-group';
import { FieldNote } from '../../../shared/form-fields/field-note';
import { PickList } from '../../../shared/form-fields/pick-list';
import { RadioCards, type RadioCard } from '../../../shared/form-fields/radio-cards';
import { SearchPick } from '../../../shared/form-fields/search-pick';
import { SelectField, type SelectOption } from '../../../shared/form-fields/select-field';
import { SwitchField } from '../../../shared/form-fields/switch-field';
import { TextField } from '../../../shared/form-fields/text-field';
import { Segmented, type Segment } from '../../../shared/segmented/segmented';

const AT = 'table_spell.effect';

/** What the effect block needs to know of the rest of the spell: it reads these, it never edits them. */
export interface EffectContext {
  /** The spell lasts (not instantaneous). */
  readonly lasts: boolean;
  /** "1 minuto", "até ser dissipada": the duration as the spell says it. */
  readonly durationText: string;
  readonly concentration: boolean;
  readonly hasSave: boolean;
  readonly hasAttack: boolean;
  /** "Sabedoria": the saving throw's ability. */
  readonly saveName: string;
  /** The target is "Só quem conjura". */
  readonly self: boolean;
  /** "As 3 criaturas da magia." */
  readonly targets: string;
}

const ABILITY_KEYS = ['str', 'dex', 'con', 'int', 'wis', 'cha'] as const;
const ROLL_OPTIONS: readonly { value: RollWord; label: string }[] = [
  { value: 'attack', label: 'Ataque' },
  { value: 'save', label: 'Teste de resistência' },
  { value: 'check', label: 'Teste de habilidade' },
];
const ADV_ROLL_OPTIONS: readonly { value: RollWord; label: string }[] = [
  { value: 'attack', label: 'Ataques' },
  { value: 'save', label: 'Resistências' },
  { value: 'check', label: 'Testes' },
];
const DICE = ['2', '3', '4', '6', '8', '10', '12', '20', '100'].map((n) => ({
  value: n,
  label: `d${n}`,
}));
const ALL_DAMAGE: SelectOption[] = [
  { value: 'all', label: 'Todo dano' },
  { value: 'nonmagical_bps', label: 'Dano de armas não mágicas' },
];

/**
 * "Deixa um efeito nos alvos" of the spell editor (#579, canvas E boards 579-01 to 579-04): the stored `TableSpell.effect`. Whom it stays on (or, as
 * an aura, how far from the caster and for whom), the conditions, what changes (seven kinds, one row each), and who sees it and what the
 * other players read. The duration and the concentration are the spell's own, and the block only shows them. The block holds no state: it
 * reads the draft and sends a whole new one, and it knows the rest of the spell only through `context`.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-spell-effect',
  imports: [
    CheckGroup,
    FieldNote,
    MatIconModule,
    PickList,
    RadioCards,
    SearchPick,
    SelectField,
    Segmented,
    SwitchField,
    TextField,
  ],
  templateUrl: './spell-effect.html',
  styleUrl: './spell-effect.scss',
})
export class SpellEffect {
  readonly effect = input.required<SpellEffectDraft>();
  readonly context = input.required<EffectContext>();
  readonly skills = input<readonly SelectOption[]>([]);
  readonly damageTypes = input<readonly SelectOption[]>([]);
  /** The six abilities' names in the sheet's order (str, dex, con, int, wis, cha). */
  readonly abilityNames = input<readonly string[]>([]);
  /** The Portuguese name of a damage-type key. */
  readonly nameOf = input<(key: string) => string>((k) => k);
  /** The messages of one path: the server's refusals and the form's own. */
  readonly issuesOf = input<(path: string) => readonly string[]>(() => []);
  readonly effectChange = output<SpellEffectDraft>();
  private readonly draft = latestInput(this.effect);

  protected readonly at = AT;
  protected readonly maxItems = MAX_EFFECT_ITEMS;
  protected readonly maxLabel = MAX_LABEL;
  protected readonly auraRefusedLine = AURA_REFUSED_PT;
  protected readonly conditionOptions = CONDITIONS.map((c) => ({
    value: c.key,
    label: c.name,
    hint: '',
    group: '',
  }));
  protected readonly rollOptions = ROLL_OPTIONS;
  protected readonly advRollOptions = ADV_ROLL_OPTIONS;
  protected readonly dice = DICE;
  protected readonly signs: SelectOption[] = [
    { value: '1', label: 'Soma' },
    { value: '-1', label: 'Tira' },
  ];
  protected readonly fixedOrDie: Segment<'fixed' | 'die'>[] = [
    { value: 'fixed', label: 'Número fixo' },
    { value: 'die', label: 'Um dado' },
  ];
  protected readonly resistances: Segment<'resist' | 'immune' | 'vulnerable'>[] = [
    { value: 'resist', label: 'Resistência' },
    { value: 'immune', label: 'Imunidade' },
    { value: 'vulnerable', label: 'Vulnerabilidade' },
  ];
  protected readonly speedModes: Segment<'add' | 'multiply'>[] = [
    { value: 'add', label: 'Soma metros' },
    { value: 'multiply', label: 'Multiplica' },
  ];
  protected readonly advantages: Segment<'adv' | 'dis'>[] = [
    { value: 'adv', label: 'Vantagem' },
    { value: 'dis', label: 'Desvantagem' },
  ];
  protected readonly affectsOptions: Segment<AuraAffectsChoice>[] = [
    { value: 'allies', label: 'Aliados de quem conjura' },
    { value: 'enemies', label: 'Inimigos' },
    { value: 'self', label: 'Só quem conjura' },
  ];

  protected readonly abilityOptions = computed(() =>
    ABILITY_KEYS.map((k, i) => ({ value: k as string, label: this.abilityNames()[i] ?? k })),
  );
  protected readonly damageOptions = computed<SelectOption[]>(() => [
    ...ALL_DAMAGE,
    ...this.damageTypes(),
  ]);

  protected readonly kindOptions = computed<SelectOption[]>(() => {
    const aura = this.effect().aura !== null;
    return EFFECT_KINDS.map((k) => {
      const refused = aura && AURA_REFUSED.includes(k.value);
      return {
        value: k.value as string,
        label: k.label,
        disabled: refused,
        side: refused ? 'não vale numa aura' : k.hint,
      };
    });
  });

  /** "Em quem fica": every target, the ones that fail the save, the ones the attack hits; the last two wait for a save or an attack on the spell. */
  protected readonly appliesCards = computed<RadioCard[]>(() => {
    const c = this.context();
    return [
      { value: 'all', label: 'Todos os alvos', hint: c.targets },
      {
        value: 'failed_save',
        label: 'Quem falhar no teste',
        hint: c.hasSave ? `De ${c.saveName}, contra a sua CD.` : 'Esta magia não tem teste.',
        disabled: !c.hasSave,
      },
      {
        value: 'hit',
        label: 'Quem o ataque acertar',
        hint: c.hasAttack ? 'O ataque da magia.' : 'Esta magia não tem ataque.',
        disabled: !c.hasAttack,
      },
    ];
  });
  protected readonly visibilityCards: RadioCard[] = [
    { value: 'public', label: 'Todos na mesa', hint: 'Os outros jogadores leem só o rótulo.' },
    { value: 'owner', label: 'Só o dono do alvo e você', hint: 'Os outros não leem nada.' },
  ];
  protected readonly chips = computed(() =>
    this.effect().conditions.map((k) => ({
      value: k,
      label: CONDITIONS.find((c) => c.key === k)?.name ?? k,
      hint: '',
      tag: '',
    })),
  );
  protected readonly auraLine = computed(() => {
    const a = this.effect().aura;
    return a ? auraTag(a) : '';
  });

  /** The whole spell's lock line while the block is off and cannot be turned on. */
  protected readonly locked = computed(() => !this.effect().on && !this.context().lasts);
  protected readonly tagsOf = (r: EffectRow): string[] => rowTags(r, this.nameOf());
  protected readonly labelCount = computed(
    () => `${[...this.effect().label].length} de ${MAX_LABEL}`,
  );

  protected path(field: string): string {
    return field ? `${AT}.${field}` : AT;
  }

  protected issues(field: string): readonly string[] {
    return this.issuesOf()(this.path(field));
  }

  /** The messages of one field of one row: its modifier or, for a row of several modifiers, any of them. */
  protected rowIssues(i: number, field: string): readonly string[] {
    const rows = this.effect().rows;
    const first = modifierIndexOf(rows, i);
    const count = Math.max(1, rowToModifiers(rows[i]).length);
    const out: string[] = [];
    for (let n = first; n < first + count; n++) {
      out.push(...this.issues(`modifiers[${n}].${field}`));
      if (field === 'kind') {
        out.push(...this.issues(`modifiers[${n}]`));
      }
    }
    return [...new Set(out)];
  }

  /** The path of a field of row `i`: its first modifier, or for the abilities of an advantage row the first one that has them (the attack's has none). */
  protected rowPath(i: number, field: string): string {
    const rows = this.effect().rows;
    const skip =
      field === 'abilities' && rows[i].kind === 'advantage' && rows[i].rolls.includes('attack')
        ? 1
        : 0;
    return this.path(`modifiers[${modifierIndexOf(rows, i) + skip}].${field}`);
  }

  protected send(next: SpellEffectDraft): void {
    this.draft.sent(next);
    this.effectChange.emit(next);
  }

  protected patch(p: Partial<SpellEffectDraft>): void {
    this.send({ ...this.draft.current(), ...p });
  }

  protected patchRow(id: number, p: Partial<EffectRow>): void {
    const d = this.draft.current();
    this.send({ ...d, rows: d.rows.map((r) => (r.id === id ? { ...r, ...p } : r)) });
  }

  protected setKind(id: number, kind: string): void {
    const d = this.draft.current();
    this.send({
      ...d,
      rows: d.rows.map((r) => (r.id === id ? { ...emptyRow(kind as EffectKind), id } : r)),
    });
  }

  protected addRow(): void {
    const d = this.draft.current();
    this.send({ ...d, rows: [...d.rows, emptyRow()] });
  }

  protected removeRow(id: number): void {
    const d = this.draft.current();
    this.send({ ...d, rows: d.rows.filter((r) => r.id !== id) });
  }

  protected turnOn(on: boolean): void {
    this.patch({ on });
  }

  protected setAura(on: boolean): void {
    this.patch(on ? { aura: DEFAULT_AURA, applies: 'all', conditions: [] } : { aura: null });
  }

  protected patchAura(p: Partial<NonNullable<SpellEffectDraft['aura']>>): void {
    const a = this.draft.current().aura;
    if (a) {
      this.patch({ aura: { ...a, ...p } });
    }
  }

  protected addCondition(k: string): void {
    const d = this.draft.current();
    if (!d.conditions.includes(k)) {
      this.patch({ conditions: [...d.conditions, k] });
    }
  }

  protected removeCondition(k: string): void {
    this.patch({ conditions: this.draft.current().conditions.filter((c) => c !== k) });
  }

  protected applies(v: string): void {
    this.patch({ applies: v as EffectApplyChoice });
  }
}
