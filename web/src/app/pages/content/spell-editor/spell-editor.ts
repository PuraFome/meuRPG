import { Component, ElementRef, Injector, afterNextRender, computed, effect, inject, input, output, signal, untracked } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { AffectedCharacter, TableEntry } from '../../../../gen/meurpg/rules/v1/table_content_pb';
import { type CatalogVm } from '../../../core/content/catalog';
import { type EntryBody, TableContentClient } from '../../../core/content/content-client';
import { EntrySaver, focusField } from '../../../core/content/entry-saver';
import { navOfKind } from '../../../core/content/content-kinds';
import { TableContentKind } from '../../../../gen/meurpg/rules/v1/table_content_pb';
import {
  ATTACK_OPTIONS,
  DURATION_OPTIONS,
  DURATION_UNITS,
  MECHANIC_OPTIONS,
  RANGE_OPTIONS,
  SAVE_SUCCESS_OPTIONS,
  SHAPE_OPTIONS,
  type SpellDraft,
  TARGET_OPTIONS,
  TIME_OPTIONS,
  circleLabel,
  draftToSpell,
  emptySpell,
  moreHint,
  moreLabel,
  sizeLabel,
  spellFieldPaths,
  spellToDraft,
  withTarget,
} from '../../../core/content/spell-draft';
import { CheckRow } from '../../../shared/form-fields/check-row';
import { SelectField, type SelectOption } from '../../../shared/form-fields/select-field';
import { SwitchField } from '../../../shared/form-fields/switch-field';
import { TextField } from '../../../shared/form-fields/text-field';
import { Segmented } from '../../../shared/segmented/segmented';
import { EntryRead } from '../entry-read/entry-read';
import { previewRead } from '../../../core/content/preview';
import { EditorAlerts, EditorBar } from '../editor-bar/editor-bar';
import { PlayersSwitch } from '../players-switch/players-switch';

export interface EditorSaved {
  readonly entry: TableEntry;
  readonly affected: readonly AffectedCharacter[];
}

/**
 * The spell editor (MR-025, RN-23; E10-01 states 4, 4b and 5): every field of a `TableSpell`, one page, one "Salvar magia".
 * The "Alvo" group comes before the mechanic (one creature, several, an area with its shape and size, only the caster), the
 * range offers "Pessoal" and "Toque" beside a distance, and the mechanic is optional ("Só texto" is always valid). The
 * preview "Como os jogadores veem" is written from the form with the same formatters as an SRD spell's "?". What the
 * server refuses comes back on the field it names. Distances are typed in metres, in steps of 1,5 m.
 */
@Component({
  selector: 'app-spell-editor',
  imports: [CheckRow, EditorAlerts, EditorBar, PlayersSwitch, EntryRead, MatButtonModule, MatIconModule, Segmented, SelectField, SwitchField, TextField],
  templateUrl: './spell-editor.html',
  styleUrl: './spell-editor.scss',
})
export class SpellEditor {
  private readonly client = inject(TableContentClient);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);

  readonly campaignId = input.required<string>();
  /** The entry being edited; `null` for "Nova magia". */
  readonly entry = input<TableEntry | null>(null);
  readonly catalog = input.required<CatalogVm>();
  /** The entries of the table, to name the one a refusal is about. */
  readonly entries = input<readonly TableEntry[]>([]);
  /** Why "Salvar" is off while the archive question is open ("" when it is not). */
  readonly saveBlocked = input('');

  readonly saved = output<EditorSaved>();
  /** The entry's switch "Disponível para os jogadores" was turned (it saves at once, apart from the form). */
  readonly switched = output<TableEntry>();
  readonly reload = output<void>();
  readonly cancelled = output<void>();

  protected readonly draft = signal<SpellDraft>(emptySpell());
  protected readonly saver = new EntrySaver(
    {
      aOne: 'uma magia',
      nameOf: (key) => this.entries().find((e) => e.key === key)?.namePt ?? '',
    },
    'a magia',
  );
  protected readonly dirty = signal(false);

  protected readonly timeOptions = TIME_OPTIONS;
  protected readonly rangeOptions: SelectOption[] = [...RANGE_OPTIONS];
  protected readonly durationOptions: SelectOption[] = [...DURATION_OPTIONS];
  protected readonly durationUnits: SelectOption[] = [...DURATION_UNITS];
  protected readonly targetOptions = TARGET_OPTIONS;
  protected readonly mechanicOptions = MECHANIC_OPTIONS;
  protected readonly shapeOptions: SelectOption[] = [...SHAPE_OPTIONS];
  protected readonly attackOptions: SelectOption[] = [...ATTACK_OPTIONS];
  protected readonly successOptions: SelectOption[] = [...SAVE_SUCCESS_OPTIONS];
  /** The abilities and the damage types are the server's (`Content`), named as it names them. */
  protected readonly abilityOptions = computed<SelectOption<number>[]>(() => this.catalog().abilities.map((a) => ({ value: a.ability as number, label: a.name })));
  protected readonly damageTypes = computed<SelectOption[]>(() => [...this.catalog().damageTypes]);
  protected readonly timeSelect: SelectOption[] = TIME_OPTIONS.map((t) => ({ value: t.value, label: t.label }));

  protected readonly levels = computed<SelectOption<number>[]>(() => {
    // A spell never crosses between truque and leveled: sheets keep them in different lists (the server says `immutable`).
    const e = this.entry();
    const original = e?.body.case === 'tableSpell' ? e.body.value.level : null;
    return Array.from({ length: 10 }, (_, level) => ({
      value: level,
      label: circleLabel(level),
      disabled: original !== null && (original === 0) !== (level === 0),
    }));
  });
  /** What a player reads, written by the same function as the player's page, from the entry the form would send. */
  protected readonly preview = computed(() => previewRead({ case: 'tableSpell', value: draftToSpell(this.draft()) }, this.catalog().nameOf));
  protected readonly previewName = computed(() => this.draft().name.trim() || 'Nova magia');
  protected readonly sizeText = computed(() => sizeLabel(this.draft().shape));
  protected readonly moreText = computed(() => moreLabel(this.draft().level));
  protected readonly moreNote = computed(() => moreHint(this.draft().level));
  protected readonly schools = computed<SelectOption[]>(() => [...this.catalog().schools]);
  protected readonly classes = computed(() => this.catalog().castingClasses);
  /** "Mais criaturas por nível de espaço" stays folded until asked, or until the spell has a number there. */
  protected readonly morePerCircle = signal(false);
  protected readonly nav = navOfKind(TableContentKind.SPELL);

  constructor() {
    // The form starts from the entry; a reload (a new revision) starts it again.
    // Keyed on the entry's key and revision, never on the object: archiving or unarchiving hands the page a new object with the
    // same revision, and the master's unsaved draft must stay.
    const source = computed(() => {
      const e = this.entry();
      return e ? `${e.key}@${e.revision}` : 'new';
    });
    effect(() => {
      source();
      untracked(() => {
        const e = this.entry();
        this.draft.set(e?.body.case === 'tableSpell' ? spellToDraft(e.body.value) : emptySpell());
        this.morePerCircle.set(false);
        this.dirty.set(false);
        this.saver.clear();
      });
    });
  }

  protected patch(p: Partial<SpellDraft>): void {
    this.draft.update((d) => ({ ...d, ...p }));
    this.dirty.set(true);
  }

  protected setTarget(value: string): void {
    this.draft.update((d) => withTarget(d, value as SpellDraft['target']));
    this.dirty.set(true);
  }

  protected toggleClass(key: string, on: boolean): void {
    const d = this.draft();
    this.patch({ classKeys: on ? [...d.classKeys, key] : d.classKeys.filter((k) => k !== key) });
  }

  protected setCount(text: string, field: 'count' | 'perSlot' | 'timeAmount' | 'durationAmount'): void {
    const n = Number(text.trim());
    this.patch({ [field]: Number.isInteger(n) && n >= 0 ? n : 0 } as Partial<SpellDraft>);
  }

  protected numberText(n: number): string {
    return n > 0 ? String(n) : '';
  }

  protected readonly known = (path: string): boolean => spellFieldPaths(this.draft()).has(path);

  protected async save(): Promise<void> {
    if (this.saver.saving() || this.saveBlocked()) {
      return;
    }
    const body: EntryBody = { case: 'tableSpell', value: draftToSpell(this.draft()) };
    const res = await this.saver.run(() => this.client.save(this.campaignId(), this.entry(), body), this.known);
    if (res) {
      this.dirty.set(false);
      this.saved.emit(res);
      return;
    }
    afterNextRender(
      () => {
        const first = this.saver.placement().fields[0];
        if (first) {
          focusField(this.host.nativeElement, first);
        } else {
          this.host.nativeElement.querySelector<HTMLElement>('[role="alert"]')?.scrollIntoView({ block: 'center' });
        }
      },
      { injector: this.injector },
    );
  }
}
