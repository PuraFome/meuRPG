import {
  Component,
  ElementRef,
  Injector,
  computed,
  effect,
  inject,
  input,
  output,
  signal,
  untracked,
} from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { WeaponCategory } from '../../../../gen/meurpg/rules/v1/rules_pb';
import type {
  TableContentViolation,
  TableEntry,
} from '../../../../gen/meurpg/rules/v1/table_content_pb';
import { type CatalogVm } from '../../../core/content/catalog';
import { type EntryBody, TableContentClient } from '../../../core/content/content-client';
import type { EffectMenuVm } from '../../../core/content/effect-draft';
import { EntrySaver, focusAfterRender } from '../../../core/content/entry-saver';
import { previewRead } from '../../../core/content/preview';
import {
  type WeaponDraft,
  VERSATILE,
  WEAPON_NAME_MAX,
  draftToWeapon,
  emptyWeapon,
  lockedProperties,
  propertyClash,
  unreadableWeaponPaths,
  weaponToDraft,
} from '../../../core/content/weapon-draft';
import { COINS } from '../../../core/content/weapon-text';
import { PickList } from '../../../shared/form-fields/pick-list';
import { RadioCards, type RadioCard } from '../../../shared/form-fields/radio-cards';
import { SelectField, type SelectOption } from '../../../shared/form-fields/select-field';
import { TextField } from '../../../shared/form-fields/text-field';
import { EditorAlerts, EditorBar } from '../editor-bar/editor-bar';
import { EntryRead } from '../entry-read/entry-read';
import { PlayersSwitch } from '../players-switch/players-switch';
import type { EditorSaved } from '../spell-editor/spell-editor';

const NOT_A_NUMBER = 'Escreva um número, como 2 ou 1,5.';

const CATEGORIES: readonly RadioCard[] = [
  { value: 'simple', label: 'Simples' },
  { value: 'martial', label: 'Marcial' },
];
const USES: readonly RadioCard[] = [
  { value: 'melee', label: 'Corpo a corpo' },
  { value: 'ranged', label: 'À distância' },
];

/**
 * The weapon editor (MR-025, RN-23, #580 S2; boards 580-10 and 580-11): a name, the category and use, the damage, the
 * properties of the SRD's list (the ones the use cannot carry are shown shut), the range in meters, the weight in
 * kilograms and the price in a coin. "Salvar arma" sends the whole entry; the server decides whether the properties go
 * together, and the form says so as soon as they do not.
 */
@Component({
  selector: 'app-weapon-editor',
  imports: [
    EditorAlerts,
    EditorBar,
    PlayersSwitch,
    EntryRead,
    MatIconModule,
    PickList,
    RadioCards,
    SelectField,
    TextField,
  ],
  templateUrl: './weapon-editor.html',
  styleUrl: '../editor.scss',
})
export class WeaponEditor {
  private readonly client = inject(TableContentClient);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);

  readonly campaignId = input.required<string>();
  readonly entry = input<TableEntry | null>(null);
  readonly menu = input.required<EffectMenuVm>();
  readonly catalog = input.required<CatalogVm>();
  readonly entries = input<readonly TableEntry[]>([]);
  readonly saveBlocked = input('');
  /** The rules refuse this entry on read (it is left out of the campaign): the violation, to put on its field. */
  readonly leftOut = input<TableContentViolation | null>(null);

  readonly saved = output<EditorSaved>();
  readonly switched = output<TableEntry>();
  readonly reload = output<void>();
  readonly cancelled = output<void>();

  protected readonly nameMax = WEAPON_NAME_MAX;
  protected readonly martial = WeaponCategory.MARTIAL;
  protected readonly categories = CATEGORIES;
  protected readonly uses = USES;
  protected readonly coins = COINS.map((c) => ({ value: c.code, label: c.label }));
  protected readonly draft = signal<WeaponDraft>(emptyWeapon());
  protected readonly saver = new EntrySaver(
    {
      aOne: 'uma arma',
      nameOf: (key) => this.entries().find((e) => e.key === key)?.namePt ?? '',
    },
    'a arma',
  );

  protected readonly damageTypeOptions = computed<SelectOption[]>(() => [
    ...this.catalog().damageTypes,
  ]);
  protected readonly versatile = computed(() => this.draft().properties.includes(VERSATILE));

  /** The SRD's properties, the ones this use cannot carry shut, with the reason. */
  protected readonly propertyOptions = computed<SelectOption[]>(() => {
    const locked = lockedProperties(this.draft().ranged);
    const why = this.draft().ranged ? 'só corpo a corpo' : 'só à distância';
    return this.menu()
      .list('weapon_properties')
      .map((p) =>
        locked.includes(p.key)
          ? { value: p.key, label: `${p.namePt} (${why})`, disabled: true }
          : { value: p.key, label: p.namePt },
      );
  });
  protected readonly clash = computed(() =>
    propertyClash(this.draft().ranged, this.draft().properties),
  );
  /** "No livro: 4 lb" under the weight in kilograms. */
  protected readonly inBook = computed(() => {
    const kg = Number(this.draft().weightKg.replace(',', '.'));
    return Number.isFinite(kg) && kg > 0 ? `No livro: ${Math.round(kg * 2 * 10) / 10} lb` : '';
  });
  protected readonly rangeShown = computed(
    () => this.draft().ranged || this.draft().properties.includes('weapon-property:thrown'),
  );
  protected readonly preview = computed(() =>
    previewRead({ case: 'tableWeapon', value: draftToWeapon(this.draft()) }, this.catalog().nameOf),
  );

  constructor() {
    const source = computed(() => {
      const e = this.entry();
      return e ? `${e.key}@${e.revision}` : 'new';
    });
    effect(() => {
      source();
      untracked(() => {
        const e = this.entry();
        this.draft.set(
          e?.body.case === 'tableWeapon' ? weaponToDraft(e.body.value) : emptyWeapon(),
        );
        this.saver.clear();
        if (this.saver.seedLeftOut(this.leftOut(), this.known)) {
          this.saver.focusRefusal(this.host.nativeElement, this.injector);
        }
      });
    });
  }

  protected patch(p: Partial<WeaponDraft>): void {
    this.draft.update((d) => ({ ...d, ...p }));
  }

  protected setCategory(v: string): void {
    this.patch({ category: v === 'martial' ? WeaponCategory.MARTIAL : WeaponCategory.SIMPLE });
  }

  protected setUse(v: string): void {
    this.patch({ ranged: v === 'ranged' });
  }

  private readonly unreadable = computed<readonly string[]>(() =>
    unreadableWeaponPaths(this.draft()),
  );

  protected readonly issuesOf = (path: string): readonly string[] => [
    ...this.saver.issues(path),
    ...(this.unreadable().includes(path) ? [NOT_A_NUMBER] : []),
    ...(path === 'table_weapon.properties' && this.clash() ? [this.clash()] : []),
  ];

  private readonly known = (path: string): boolean => {
    const p = 'table_weapon';
    const fixed = [
      'name_pt',
      'desc_pt',
      'category',
      'damage',
      'damage_type',
      'properties',
      'normal_range_ft',
      'long_range_ft',
      'weight_oz',
      'price_cp',
    ].map((f) => `${p}.${f}`);
    return (
      fixed.includes(path) ||
      (this.versatile() && path === `${p}.two_handed_damage`) ||
      /^table_weapon\.properties\[\d+\]$/.test(path)
    );
  };

  protected async save(): Promise<void> {
    if (this.saver.saving() || this.saveBlocked()) {
      return;
    }
    if (
      this.saver.blockOn([...this.unreadable()], (path) =>
        focusAfterRender(this.host.nativeElement, this.injector, path),
      )
    ) {
      return;
    }
    const body: EntryBody = { case: 'tableWeapon', value: draftToWeapon(this.draft()) };
    const res = await this.saver.run(
      () => this.client.save(this.campaignId(), this.entry(), body, this.saver.keyFor(body)),
      this.known,
    );
    if (res) {
      this.saved.emit(res);
      return;
    }
    this.saver.focusRefusal(this.host.nativeElement, this.injector);
  }
}
