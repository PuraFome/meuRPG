import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  computed,
  inject,
  input,
  output,
  signal,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatMenuModule } from '@angular/material/menu';

import {
  type DamageModDraft,
  MAX_DAMAGE_MODS,
  MOD_KINDS,
  QUALIFIERS,
  emptyDamageMod,
  kindLabel,
  modSub,
  modTitle,
  qualifierWhy,
} from '../../../core/content/creature-damage-mods';
import { focusAfterRender } from '../../../core/content/entry-saver';
import { FieldNote } from '../../../shared/form-fields/field-note';
import { Segmented } from '../../../shared/segmented/segmented';
import { PickList } from '../../../shared/form-fields/pick-list';
import { type RadioCard, RadioCards } from '../../../shared/form-fields/radio-cards';
import type { SelectOption } from '../../../shared/form-fields/select-field';

const P = 'table_creature.damage_mods';

const CARDS: readonly RadioCard[] = QUALIFIERS.map((q) => ({
  value: q.value,
  label: q.label,
  hint: q.hint,
}));
/** The same cards with "Vale contra" off: only "Todo dano desses tipos" is left. */
const CARDS_OFF: readonly RadioCard[] = CARDS.map((c) => ({ ...c, disabled: c.value !== '' }));

/**
 * "Resistências, imunidades e vulnerabilidades" of the table creature (#585): up to 12 rows, each a kind, the damage types it names
 * and a "Vale contra" qualifier. They add to the base's lists and never take one away. A row is one line that opens in place; the
 * qualifier is off for a vulnerability and for a damage type that is not bludgeoning, piercing or slashing, with the reason under it.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-creature-damage-mods',
  imports: [
    FieldNote,
    MatButtonModule,
    MatIconModule,
    MatMenuModule,
    PickList,
    RadioCards,
    Segmented,
  ],
  templateUrl: './damage-mods.html',
  styleUrls: ['../editor.scss', './creature-rows.scss', './damage-mods.scss'],
})
export class CreatureDamageMods {
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);

  readonly mods = input.required<readonly DamageModDraft[]>();
  readonly damageTypes = input.required<readonly SelectOption[]>();
  readonly nameOf = input.required<(key: string) => string>();
  /** What the base's stat block already says, as `baseModsText` writes it. */
  readonly baseText = input('');
  readonly baseName = input('');
  /** The refusals of a path, and whether any landed at or under it. */
  readonly issues = input.required<(path: string) => readonly string[]>();
  readonly refused = input.required<(path: string) => boolean>();
  readonly modsChange = output<DamageModDraft[]>();

  protected readonly path = P;
  protected readonly kinds = MOD_KINDS;
  protected readonly max = MAX_DAMAGE_MODS;
  protected readonly full = computed(() => this.mods().length >= MAX_DAMAGE_MODS);
  protected readonly status = signal('');
  private readonly open = signal<ReadonlySet<string>>(new Set());

  /** The live region says a repeated message again: the text alternates by a no-break space, so it always changes. */
  private say(text: string): void {
    this.status.update((s) => (s === text ? `${text}\u00a0` : text));
  }

  protected rowPath(i: number): string {
    return `${P}[${i}]`;
  }

  protected isOpen(id: string, i: number): boolean {
    return this.open().has(id) || this.refused()(this.rowPath(i));
  }

  protected toggle(id: string): void {
    this.open.update((s) => {
      const next = new Set(s);
      if (!next.delete(id)) {
        next.add(id);
      }
      return next;
    });
  }

  protected title(m: DamageModDraft): string {
    return modTitle(m, this.nameOf());
  }

  protected sub(m: DamageModDraft): string {
    return modSub(m);
  }

  protected kindLabel(m: DamageModDraft): string {
    return kindLabel(m.kind);
  }

  protected why(m: DamageModDraft): string {
    return qualifierWhy(m.kind, m.types);
  }

  protected cards(m: DamageModDraft): readonly RadioCard[] {
    return this.why(m) === '' ? CARDS : CARDS_OFF;
  }

  private change(id: string, p: Partial<DamageModDraft>): void {
    this.modsChange.emit(this.mods().map((m) => (m.id === id ? { ...m, ...p } : m)));
  }

  /** A kind or a type that takes "Vale contra" away from the row takes the qualifier with it, so the save is not refused for it. */
  private withQualifier(m: DamageModDraft, p: Partial<DamageModDraft>): Partial<DamageModDraft> {
    const next = { ...m, ...p };
    return qualifierWhy(next.kind, next.types) === '' ? p : { ...p, qualifier: '' };
  }

  protected setKind(m: DamageModDraft, kind: string): void {
    this.change(m.id, this.withQualifier(m, { kind }));
  }

  protected addType(m: DamageModDraft, key: string): void {
    this.change(m.id, this.withQualifier(m, { types: [...m.types, key] }));
  }

  /** One chip off, by its place: a row that holds a type twice keeps the other until it is taken off too. */
  protected removeTypeAt(m: DamageModDraft, at: number): void {
    this.change(m.id, { types: m.types.filter((_, i) => i !== at) });
  }

  protected setQualifier(m: DamageModDraft, qualifier: string): void {
    this.change(m.id, { qualifier });
  }

  protected add(): void {
    if (this.full()) {
      return;
    }
    const m = emptyDamageMod();
    const at = this.mods().length;
    this.modsChange.emit([...this.mods(), m]);
    this.open.update((s) => new Set(s).add(m.id));
    this.say('Linha adicionada.');
    focusAfterRender(this.host.nativeElement, this.injector, `${this.rowPath(at)}.kind`);
  }

  protected remove(id: string): void {
    this.modsChange.emit(this.mods().filter((m) => m.id !== id));
    this.say('Linha removida.');
    afterNextRender(
      () => {
        this.host.nativeElement.querySelector<HTMLElement>('.add button')?.focus();
      },
      { injector: this.injector },
    );
  }
}
