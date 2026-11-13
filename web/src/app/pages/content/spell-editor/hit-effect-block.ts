import { ChangeDetectionStrategy, Component, computed, input, output, signal } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { CONDITIONS } from '../../../core/combat/conditions';
import {
  HIT_CAP_MAX,
  HIT_DICE_FACES,
  HIT_DICE_MAX,
  HIT_MARKED,
  HIT_NEXT,
  HIT_PER_SLOT_MAX,
  INSTANT_LOCK_PT,
  NEEDS_CONCENTRATION_PT,
  type HitDraft,
  type HitLasts,
  type HitTriggerKey,
  hitAvailable,
  markedLock,
  hitMovedNote,
  needsConcentration,
} from '../../../core/content/hit-effect-draft';
import { FieldNote } from '../../../shared/form-fields/field-note';
import { SelectField, type SelectOption } from '../../../shared/form-fields/select-field';
import { SwitchField } from '../../../shared/form-fields/switch-field';
import { TextField } from '../../../shared/form-fields/text-field';
import { latestInput } from '../../../shared/latest-input';

let nextId = 0;

interface TriggerCard {
  readonly value: HitTriggerKey;
  readonly label: string;
  readonly hint: string;
}

const TRIGGER_CARDS: readonly TriggerCard[] = [
  { value: 'next_hit', label: 'Próximo golpe com arma', hint: 'Acerta uma vez e acaba.' },
  { value: 'next_melee_hit', label: 'Próximo golpe corpo a corpo', hint: 'À distância não gasta.' },
  { value: 'each_hit', label: 'Cada golpe enquanto durar', hint: 'Todo golpe com arma.' },
  { value: HIT_MARKED, label: 'Cada golpe no alvo marcado', hint: 'Só no alvo da magia.' },
];

export const HIT_HOW_PT =
  'O efeito fica em quem conjura e espera o golpe que acerta. Num crítico, os dados dobram. Com teste de resistência, o alvo testa depois do dano, contra a CD de magia de quem conjura. Quando a concentração (ou a duração) acaba, o efeito acaba, com ou sem golpe.';

/**
 * "Efeito no próximo golpe" of the spell editor (#577): the extra damage the caster's weapon hits carry while the spell lasts,
 * and an optional saving throw that puts a condition on the target. The block edits a `HitDraft` and nothing else: the spell's
 * target, its duration and its concentration come in as they are, and the refusals come by path from the page (the server's, and
 * the form's own). It is off by default; a spell that does not last cannot keep it, and says why.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-hit-effect-block',
  imports: [FieldNote, MatIconModule, SelectField, SwitchField, TextField],
  templateUrl: './hit-effect-block.html',
  styleUrl: './hit-effect-block.scss',
})
export class HitEffectBlock {
  readonly hit = input.required<HitDraft>();
  /** The spell's target ("creature", "creatures", "area", "self") and its extra targets per level: the marked target needs one creature. */
  readonly target = input.required<string>();
  readonly perSlot = input(0);
  /** The spell's level: a cantrip sends no extra targets per level. */
  readonly level = input(1);
  /** The lock moved the trigger to the next hit on the last change: the block says so, politely. */
  readonly moved = input(false);
  /** The spell lasts ("timed", "until_dispelled") or not ("instant"). */
  readonly duration = input.required<string>();
  /** The duration in words for the chip ("1 minuto"). */
  readonly durationText = input('');
  readonly concentration = input(false);
  /** The abilities by their wire key ("con"), and the damage types by their SRD key, as the catalog names them. */
  readonly abilities = input<readonly SelectOption[]>([]);
  readonly damageTypes = input<readonly SelectOption[]>([]);
  /** The path of one of the modifier's fields in the request, where a refusal lands. */
  readonly pathOf = input<(field: string) => string>((f) => f);
  /** The messages of one input by its path: the server's refusals and the form's own. */
  readonly issuesOf = input<(path: string) => readonly string[]>(() => []);
  readonly hitChange = output<HitDraft>();

  private readonly latest = latestInput(this.hit);
  private readonly id = nextId++;
  protected readonly reasonId = `hit-reason-${this.id}`;
  protected readonly howId = `hit-how-${this.id}`;
  protected readonly whenId = `hit-when-${this.id}`;
  protected readonly instantLock = INSTANT_LOCK_PT;
  protected readonly concentrationNote = NEEDS_CONCENTRATION_PT;
  protected readonly how = HIT_HOW_PT;
  protected readonly diceMax = HIT_DICE_MAX;
  protected readonly perSlotMax = HIT_PER_SLOT_MAX;
  protected readonly capMax = HIT_CAP_MAX;

  protected readonly available = computed(() => hitAvailable({ duration: this.duration() }));
  protected readonly on = computed(() => this.hit().enabled && this.available());
  protected readonly hasSave = computed(() => this.hit().saveAbility !== '');
  protected readonly lock = computed(() =>
    markedLock({ target: this.target(), perSlot: this.perSlot(), level: this.level() }),
  );
  protected readonly movedText = computed(() =>
    this.moved() && this.lock() !== '' && this.hit().trigger === HIT_NEXT
      ? hitMovedNote(this.lock())
      : '',
  );
  protected readonly cards = computed(() =>
    TRIGGER_CARDS.map((c) => ({
      ...c,
      checked: this.hit().trigger === c.value,
      locked: c.value === HIT_MARKED && this.lock() !== '',
      hint: c.value === HIT_MARKED && this.lock() !== '' ? this.lock() : c.hint,
    })),
  );
  /** Where the effect lands, said under the switch: it is the caster's, never the targets'. */
  protected readonly whereText = computed(() => {
    switch (this.target()) {
      case 'area':
        return 'em quem conjura, não na área';
      case 'creatures':
        return 'em quem conjura, não nos alvos';
      default:
        return 'em quem conjura';
    }
  });
  protected readonly needsConcentration = computed(
    () => needsConcentration(this.hit().trigger) && !this.concentration(),
  );
  protected readonly dice: readonly SelectOption<number>[] = HIT_DICE_FACES.map((n) => ({
    value: n,
    label: `d${n}`,
  }));
  protected readonly damageOptions = computed<SelectOption[]>(() => [
    { value: '', label: 'O da arma', side: 'cortante, perfurante ou concussão, como a arma' },
    ...this.damageTypes(),
  ]);
  protected readonly abilityOptions = computed<SelectOption[]>(() => [
    { value: '', label: 'Nenhum teste' },
    ...this.abilities(),
  ]);
  protected readonly conditionOptions: readonly SelectOption[] = CONDITIONS.map((c) => ({
    value: c.key,
    label: c.name,
  }));
  protected readonly lastsOptions: readonly SelectOption[] = [
    { value: 'next_turn', label: 'Até o fim do próximo turno do alvo' },
    {
      value: 'spell_end',
      label: 'Até o fim da magia',
      side: 'acaba junto com a concentração ou a duração',
    },
  ];

  protected readonly open = signal(false);

  protected issues(field: string): readonly string[] {
    return this.issuesOf()(this.pathOf()(field));
  }

  protected path(field: string): string {
    return this.pathOf()(field);
  }

  protected patch(p: Partial<HitDraft>): void {
    const next = { ...this.latest.current(), ...p };
    this.latest.sent(next);
    this.hitChange.emit(next);
  }

  protected setSave(ability: string): void {
    this.patch(
      ability === ''
        ? { saveAbility: '', condition: '', extraConditions: [], conditionLasts: 'next_turn' }
        : { saveAbility: ability },
    );
  }

  protected setCondition(key: string): void {
    this.patch({
      condition: key,
      extraConditions: this.latest.current().extraConditions.filter((k) => k !== key),
    });
  }

  protected setLasts(value: string): void {
    this.patch({ conditionLasts: value as HitLasts });
  }

  protected setOpen(): void {
    this.open.update((o) => !o);
  }
}
