import { Component, ElementRef, Injector, afterNextRender, computed, effect, inject, input, output, signal, untracked } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { RouterLink } from '@angular/router';

import { Ability } from '../../../../gen/meurpg/rules/v1/rules_pb';
import { PlayersSwitch } from '../players-switch/players-switch';
import type { GetClassTableDefaultsResponse, TableEntry } from '../../../../gen/meurpg/rules/v1/table_content_pb';
import type { CatalogVm } from '../../../core/content/catalog';
import {
  type CastingDraft,
  type CastingKind,
  type ClassDraft,
  LEVEL_COUNT,
  type LevelFeature,
  type Preparation,
  castingOfKind,
  classFeatureBase,
  classFeaturePaths,
  classToDraft,
  defaultTableOf,
  draftToClass,
  emptyClass,
  featureIdOfPath,
  gridColumns,
  isGridPath,
  newLevelFeatureId,
  rowsEdited,
  rowsOfTable,
  sortedFeatures,
} from '../../../core/content/class-draft';
import { type EntryBody, TableContentClient } from '../../../core/content/content-client';
import type { EffectMenuVm } from '../../../core/content/effect-draft';
import { EntrySaver, focusField } from '../../../core/content/entry-saver';
import { ABILITY_FIELDS, type AbilityField, emptyFeature } from '../../../core/content/feature-draft';
import { CheckRow } from '../../../shared/form-fields/check-row';
import { FieldNote } from '../../../shared/form-fields/field-note';
import { NumberStepper } from '../../../shared/form-fields/number-stepper';
import { PickList } from '../../../shared/form-fields/pick-list';
import { SelectField, type SelectOption } from '../../../shared/form-fields/select-field';
import { TextField } from '../../../shared/form-fields/text-field';
import { type GridChip, type GridEdit, type GridRowVm, LevelGrid } from '../../../shared/level-grid/level-grid';
import { CastingFields } from '../class-parts/casting-fields';
import { ClassFeatures } from '../class-parts/class-features';
import { type NavSection, SectionNav } from '../class-parts/section-nav';
import { TableQuestion } from '../class-parts/table-question';
import { EditorAlerts, EditorBar } from '../editor-bar/editor-bar';
import type { EditorSaved } from '../spell-editor/spell-editor';

/** The armor, shield and weapon groups every class lists (the keys; the names are the server's). */
const ARMOR_KEYS = ['proficiency:light-armor', 'proficiency:medium-armor', 'proficiency:heavy-armor', 'proficiency:shields'] as const;
const WEAPON_KEYS = ['proficiency:simple-weapons', 'proficiency:martial-weapons'] as const;
const GROUP_KEYS: readonly string[] = [...ARMOR_KEYS, ...WEAPON_KEYS];

/** The question that comes up when a change would replace a table the master edited. */
type TableAsk = { readonly kind: 'casting'; readonly next: CastingDraft } | { readonly kind: 'restore' };

/**
 * The class editor (MR-025, RN-23, ADR-0018; E10-02 states 1 to 3): one page with sections and a list of them at the side. The
 * basics, the proficiencies, how it casts, the 20-level table (the server's defaults, edited cell by cell), the features with
 * their effects from the server's menu, the level the subclass is chosen at and the subclasses the class has. One "Salvar
 * classe" sends the whole entry; a refusal comes back on the input it names, a grid cell included. The class's switch "Disponível
 * para os jogadores" is slice 10.11c's: it goes in the "Para os jogadores" panel at the foot of the side list (see the seam in
 * the template).
 */
@Component({
  selector: 'app-class-editor',
  imports: [
    CastingFields,
    CheckRow,
    ClassFeatures,
    EditorAlerts,
    EditorBar,
    FieldNote,
    LevelGrid,
    MatButtonModule,
    MatIconModule,
    NumberStepper,
    PickList,
    PlayersSwitch,
    RouterLink,
    SectionNav,
    SelectField,
    TableQuestion,
    TextField,
  ],
  templateUrl: './class-editor.html',
  styleUrl: './class-editor.scss',
})
export class ClassEditor {
  private readonly client = inject(TableContentClient);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);

  readonly campaignId = input.required<string>();
  readonly entry = input<TableEntry | null>(null);
  readonly menu = input.required<EffectMenuVm>();
  readonly catalog = input.required<CatalogVm>();
  readonly defaults = input.required<GetClassTableDefaultsResponse>();
  readonly entries = input<readonly TableEntry[]>([]);
  readonly saveBlocked = input('');

  readonly saved = output<EditorSaved>();
  /** The entry's switch "Disponível para os jogadores" was turned (it saves at once, apart from the form). */
  readonly switched = output<TableEntry>();
  readonly reload = output<void>();
  readonly cancelled = output<void>();

  protected readonly draft = signal<ClassDraft>(emptyClass(this.emptyDefaults()));
  protected readonly openFeature = signal('');
  protected readonly question = signal<TableAsk | null>(null);
  protected readonly allCircles = signal(false);
  protected readonly status = signal('');

  protected readonly saver = new EntrySaver(
    ((editor: ClassEditor) => ({
      aOne: 'uma classe',
      nameOf: (key: string) => editor.entries().find((e) => e.key === key)?.namePt ?? '',
      get maxFeatures() {
        return editor.menu().maxFeatures;
      },
      get featureCount() {
        return editor.draft().features.length;
      },
      // "Too many features" is the panel head's, with the counter, and not one feature row's.
      redirect: (field: string, reason: string) => (reason === 'limit' && /^table_class\.levels\[\d+\]\.features\[\d+\]$/.test(field) ? 'table_class.features' : field),
    }))(this),
    'a classe',
  );

  private static readonly SECTIONS = [
    { id: 'basics', label: 'Básico' },
    { id: 'profs', label: 'Proficiências' },
    { id: 'casting', label: 'Conjuração' },
    { id: 'table', label: 'Tabela dos 20 níveis' },
    { id: 'features', label: 'Características' },
    { id: 'subclasses', label: 'Subclasse' },
  ] as const;

  /** The list of sections, each with whether a refusal is inside it. */
  protected readonly sections = computed<NavSection[]>(() => ClassEditor.SECTIONS.map((s) => ({ ...s, issue: this.sectionIssues(s.id) })));

  protected readonly abilities = computed(() => this.catalog().abilities);
  protected readonly abilityOptions = computed<SelectOption<number>[]>(() => this.abilities().map((a) => ({ value: a.ability as number, label: a.name })));
  protected readonly hitDice: SelectOption<number>[] = [6, 8, 10, 12].map((n) => ({ value: n, label: `d${n}` }));
  protected readonly levelOptions: SelectOption<number>[] = Array.from({ length: LEVEL_COUNT }, (_, i) => ({ value: i + 1, label: String(i + 1) }));
  protected readonly levelNumbers = Array.from({ length: LEVEL_COUNT }, (_, i) => i + 1);
  protected readonly asiOptions: SelectOption[] = this.levelOptions.map((o) => ({ value: String(o.value), label: o.label }));
  protected readonly skills = computed(() => this.catalog().skills);
  protected readonly spellOptions = computed<SelectOption[]>(() => [...this.catalog().spells]);

  protected readonly armor = computed(() => this.groupOptions(ARMOR_KEYS));
  protected readonly weapons = computed(() => this.groupOptions(WEAPON_KEYS));
  /** The proficiencies that are not one of the six groups: single weapons, armors and tools, from the menu. */
  protected readonly otherProficiencies = computed<SelectOption[]>(() =>
    this.menu()
      .list('proficiency_targets')
      .filter((p) => p.key.startsWith('proficiency:') && !GROUP_KEYS.includes(p.key))
      .map((p) => ({ value: p.key, label: p.namePt })),
  );

  protected readonly casting = computed(() => this.draft().casting);
  protected readonly columns = computed(() => gridColumns(this.casting(), this.draft().rows, this.defaults(), this.allCircles()));
  protected readonly canAllCircles = computed(() => this.casting().kind !== '' && this.columns().circles < 9);
  protected readonly isNew = computed(() => this.entry() === null);
  protected readonly selfKey = computed(() => this.entry()?.key ?? '');
  protected readonly subclasses = computed(() => {
    const key = this.selfKey();
    return key ? this.entries().filter((e) => e.body.case === 'tableSubclass' && e.body.value.classKey === key) : [];
  });

  protected readonly gridRows = computed<GridRowVm[]>(() => {
    const d = this.draft();
    return d.rows.map((row, i) => {
      const level = i + 1;
      const chips: GridChip[] = [];
      if (d.asiLevels.includes(level)) chips.push({ id: `asi-${level}`, label: 'Aumento de atributo', locked: true });
      if (d.subclassLevel === level) chips.push({ id: `sub-${level}`, label: 'Escolha de subclasse', locked: true });
      for (const f of d.features.filter((x) => x.level === level)) {
        chips.push({ id: f.id, label: f.feature.name.trim() || 'Sem nome', locked: false });
      }
      return { level, base: `table_class.levels[${i}]`, row, chips };
    });
  });

  /** "Preenchida com a tabela do SRD de Paladino": says where the numbers come from, in the server's words. */
  protected readonly tableNote = computed(() => {
    const t = defaultTableOf(this.casting(), this.defaults());
    const ref = t?.referenceClassKey ? this.catalog().nameOf(t.referenceClassKey) : '';
    const kind = this.casting().kind;
    if (kind === '') {
      return 'Uma classe sem conjuração só tem o bônus de proficiência e as características em cada nível. “Restaurar o padrão” volta ao bônus do SRD.';
    }
    const from = ref ? `Preenchida com a tabela do ${ref} do SRD.` : 'Preenchida com o padrão de um terço.';
    const prep = this.casting().preparation === 'prepared' ? ' As magias são preparadas: o número vem da habilidade e do nível, não da tabela.' : ' As magias são conhecidas: a coluna “Magias” diz quantas.';
    return `${from} Edite o que quiser; “Restaurar o padrão” volta a ela.${prep}`;
  });

  protected readonly questionText = computed(() => {
    const q = this.question();
    if (!q) return '';
    const t = q.kind === 'restore' ? defaultTableOf(this.casting(), this.defaults()) : defaultTableOf(q.next, this.defaults());
    const ref = t?.referenceClassKey ? ` do ${this.catalog().nameOf(t.referenceClassKey)}` : '';
    return q.kind === 'restore'
      ? `Voltar a tabela ao padrão${ref}? O que você editou nela se perde.`
      : `Você editou a tabela dos níveis. Refazê-la com o padrão${ref} para esta conjuração? O que você editou nela se perde.`;
  });

  constructor() {
    // Keyed on the entry's key and revision, never on the object: archiving hands the page a new object with the same revision,
    // and the master's unsaved draft must stay. A new class waits for the defaults.
    const source = computed(() => {
      const e = this.entry();
      return e ? `${e.key}@${e.revision}` : 'new';
    });
    effect(() => {
      source();
      const defaults = this.defaults();
      untracked(() => this.start(defaults));
    });
  }

  private emptyDefaults(): GetClassTableDefaultsResponse {
    return { profBonus: [], asiLevels: [], subclassLevel: 3, tables: [] } as unknown as GetClassTableDefaultsResponse;
  }

  private start(defaults: GetClassTableDefaultsResponse): void {
    const e = this.entry();
    this.draft.set(e?.body.case === 'tableClass' ? classToDraft(e.body.value, defaults) : emptyClass(defaults));
    this.openFeature.set('');
    this.question.set(null);
    this.saver.clear();
  }

  private groupOptions(keys: readonly string[]): SelectOption[] {
    const list = this.menu().list('proficiency_targets');
    return keys.map((k) => ({ value: k, label: list.find((p) => p.key === k)?.namePt ?? this.catalog().nameOf(k) }));
  }

  protected patch(p: Partial<ClassDraft>): void {
    this.draft.update((d) => ({ ...d, ...p }));
  }

  protected readonly issuesOf = (path: string): readonly string[] => this.saver.issues(path);

  /** Whether a section has a refusal in it. */
  private sectionIssues(id: string): boolean {
    const fields = this.saver.placement().fields;
    const match = (f: string): boolean => {
      const p = f.replace('table_class.', '');
      switch (id) {
        case 'basics':
          return /^(name_pt|hit_die|saving_throws|skill_)/.test(p);
        case 'profs':
          return /^(proficiencies|multiclass_|minimums|any_of)/.test(p);
        case 'casting':
          return p.startsWith('casting');
        case 'table':
          return /^levels(\[\d+\](\.(prof_bonus|cantrips_known|spells_known|slots.*))?)?$/.test(p) || p.startsWith('asi_levels');
        case 'features':
          return /^levels\[\d+\]\.features/.test(p) || p === 'features';
        default:
          return p.startsWith('subclass_level');
      }
    };
    return fields.some(match);
  }

  private scrollTo(id: string): void {
    const el = this.host.nativeElement.querySelector<HTMLElement>(`#sec-${id}`);
    el?.scrollIntoView?.({ block: 'start', behavior: 'auto' });
  }

  // ---- The basics.

  protected setSave(i: 0 | 1, ability: number): void {
    const saves: [Ability, Ability] = [...this.draft().saves];
    saves[i] = ability as Ability;
    this.patch({ saves });
  }

  protected toggleSkill(key: string): void {
    const d = this.draft();
    this.patch({ skillFrom: d.skillFrom.includes(key) ? d.skillFrom.filter((k) => k !== key) : [...d.skillFrom, key] });
  }

  protected setSkillChoose(n: number): void {
    this.patch({ skillChoose: Math.max(0, n) });
  }

  // ---- The proficiencies.

  protected toggleProficiency(key: string, on: boolean): void {
    const d = this.draft();
    this.patch({ proficiencies: on ? [...d.proficiencies.filter((k) => k !== key), key] : d.proficiencies.filter((k) => k !== key) });
  }

  protected has(key: string): boolean {
    return this.draft().proficiencies.includes(key);
  }

  protected othersOf(list: readonly string[]): string[] {
    return list.filter((k) => !GROUP_KEYS.includes(k));
  }

  protected addOther(key: string): void {
    this.patch({ proficiencies: [...this.draft().proficiencies, key] });
  }

  protected removeOther(key: string): void {
    this.patch({ proficiencies: this.draft().proficiencies.filter((k) => k !== key) });
  }

  protected addMulticlassProficiency(key: string): void {
    this.patch({ multiclassProficiencies: [...this.draft().multiclassProficiencies, key] });
  }

  protected removeMulticlassProficiency(key: string): void {
    this.patch({ multiclassProficiencies: this.draft().multiclassProficiencies.filter((k) => k !== key) });
  }

  /** The two multiclass groups: "todas" (`minimums`) and "pelo menos uma" (`any_of`). */
  protected readonly requirementGroups = [
    { field: 'minimums' as const, label: 'Quem entra na classe precisa de todas estas habilidades', add: 'Adicionar habilidade' },
    { field: 'anyOf' as const, label: 'Ou de pelo menos uma destas', add: 'Adicionar habilidade' },
  ];

  protected requirementPath(field: 'minimums' | 'anyOf', ability?: AbilityField): string {
    return `table_class.${field === 'anyOf' ? 'any_of' : 'minimums'}${ability ? '.' + ability : ''}`;
  }

  protected requirementOptions(field: 'minimums' | 'anyOf'): SelectOption[] {
    return this.abilities().map((a) => ({ value: a.field, label: a.name }));
  }

  protected requirementValues(field: 'minimums' | 'anyOf'): AbilityField[] {
    const m = this.draft()[field];
    return ABILITY_FIELDS.filter((a) => m[a] > 0);
  }

  protected abilityName(field: AbilityField): string {
    return this.abilities().find((a) => a.field === field)?.name ?? field;
  }

  protected addRequirement(field: 'minimums' | 'anyOf', ability: string): void {
    const d = this.draft();
    this.patch({ [field]: { ...d[field], [ability]: 13 } });
  }

  protected removeRequirement(field: 'minimums' | 'anyOf', ability: string): void {
    const d = this.draft();
    this.patch({ [field]: { ...d[field], [ability]: 0 } });
  }

  /** Another ability for a row: the minimum moves with it (a row never takes an ability another row has). */
  protected moveRequirement(field: 'minimums' | 'anyOf', from: AbilityField, to: string): void {
    const d = this.draft();
    if (to === from || d[field][to as AbilityField] > 0) return;
    this.patch({ [field]: { ...d[field], [from]: 0, [to]: d[field][from] } });
  }

  protected freeRequirementOptions(field: 'minimums' | 'anyOf'): SelectOption[] {
    const m = this.draft()[field];
    return this.requirementOptions(field).filter((o) => !(m[o.value as AbilityField] > 0));
  }

  protected setRequirement(field: 'minimums' | 'anyOf', ability: AbilityField, text: string): void {
    const n = Number(text.replace(/\D/g, ''));
    const d = this.draft();
    // A field emptied or at 0 keeps its place at 1 until it is taken off: 0 would remove the line while it is typed.
    this.patch({ [field]: { ...d[field], [ability]: Number.isFinite(n) && n > 0 ? n : 1 } });
  }

  // ---- The casting and the table.

  protected setKind(kind: CastingKind): void {
    this.changeCasting(castingOfKind(kind, this.casting(), this.defaults()), kind !== this.casting().kind);
  }

  protected setPreparation(preparation: Preparation): void {
    if (preparation === this.casting().preparation) return;
    this.changeCasting(castingOfKind(this.casting().kind, { ...this.casting(), preparation }, this.defaults()), true);
  }

  protected patchCasting(p: Partial<CastingDraft>): void {
    this.patch({ casting: { ...this.casting(), ...p } });
  }

  /** A new way of casting has a new default table: it replaces the table, unless the master edited it; then it asks first. */
  private changeCasting(next: CastingDraft, tableChanges: boolean): void {
    const d = this.draft();
    if (tableChanges && rowsEdited(d.rows, d.casting, this.defaults())) {
      this.question.set({ kind: 'casting', next });
      return;
    }
    this.patch({ casting: next, rows: tableChanges ? rowsOfTable(defaultTableOf(next, this.defaults()), this.defaults()) : d.rows });
  }

  protected answerQuestion(replace: boolean): void {
    const q = this.question();
    this.question.set(null);
    if (!q) return;
    if (q.kind === 'restore') {
      if (replace) {
        this.patch({ rows: rowsOfTable(defaultTableOf(this.casting(), this.defaults()), this.defaults()) });
        this.status.set('A tabela voltou ao padrão.');
      }
      return;
    }
    this.patch({ casting: q.next, rows: replace ? rowsOfTable(defaultTableOf(q.next, this.defaults()), this.defaults()) : this.draft().rows });
    this.status.set(replace ? 'A tabela foi refeita com o padrão.' : 'A conjuração mudou; a tabela ficou como estava.');
  }

  protected restore(): void {
    if (rowsEdited(this.draft().rows, this.casting(), this.defaults())) {
      this.question.set({ kind: 'restore' });
      return;
    }
    this.status.set('A tabela já está no padrão.');
  }

  protected editCell(e: GridEdit): void {
    const rows = this.draft().rows.map((r, i) => {
      if (i !== e.level - 1) return r;
      switch (e.field) {
        case 'profBonus':
          return { ...r, profBonus: e.value };
        case 'cantrips':
          return { ...r, cantrips: e.value };
        case 'spells':
          return { ...r, spells: e.value };
        default:
          return { ...r, slots: r.slots.map((n, k) => (k === e.slot ? e.value : n)) };
      }
    });
    this.patch({ rows });
  }

  protected setAsi(levels: string[]): void {
    this.patch({ asiLevels: levels.map(Number).sort((a, b) => a - b) });
  }

  protected asiValues(): string[] {
    return this.draft().asiLevels.map(String);
  }

  // ---- The features.

  protected setFeatures(features: LevelFeature[]): void {
    this.patch({ features });
  }

  protected readonly featureBase = (i: number): string => classFeatureBase(this.draft().features, i);

  protected openFromGrid(id: string): void {
    this.openFeature.set(id);
    this.scrollTo('features');
  }

  protected addFromGrid(level: number): void {
    const id = newLevelFeatureId();
    this.patch({ features: sortedFeatures([...this.draft().features, { id, level, feature: emptyFeature() }]) });
    this.openFeature.set(id);
    this.scrollTo('features');
  }

  // ---- Saving.

  protected subclassLink(e: TableEntry): string[] {
    return ['/campaigns', this.campaignId(), 'content', 'entries', e.key];
  }

  protected newSubclassLink(): string[] {
    return ['/campaigns', this.campaignId(), 'content', 'new', 'subclass'];
  }

  private readonly known = (path: string): boolean => {
    const p = 'table_class';
    const d = this.draft();
    const fixed = [
      `${p}.name_pt`, `${p}.hit_die`, `${p}.saving_throws`, `${p}.saving_throws[0]`, `${p}.saving_throws[1]`, `${p}.skill_choose`, `${p}.skill_from`,
      `${p}.proficiencies`, `${p}.multiclass_proficiencies`, `${p}.multiclass_skill_choose`, `${p}.minimums`, `${p}.any_of`, `${p}.subclass_level`,
      `${p}.asi_levels`, `${p}.casting.kind`, `${p}.casting.ability`, `${p}.casting.list_from`, `${p}.casting.preparation`, `${p}.casting.start_level`,
      `${p}.casting.prepared_max`, `${p}.casting`, `${p}.features`,
    ];
    if (fixed.includes(path)) return true;
    if (ABILITY_FIELDS.some((a) => (path === `${p}.minimums.${a}` ? d.minimums[a] > 0 : path === `${p}.any_of.${a}` ? d.anyOf[a] > 0 : false))) return true;
    if (isGridPath(path, `${p}.levels`, this.columns(), true)) return true;
    return classFeaturePaths(d.features, this.menu()).includes(path);
  };

  protected async save(): Promise<void> {
    if (this.saver.saving() || this.saveBlocked()) {
      return;
    }
    const body: EntryBody = { case: 'tableClass', value: draftToClass(this.draft(), this.menu()) };
    const res = await this.saver.run(() => this.client.save(this.campaignId(), this.entry(), body), this.known);
    if (res) {
      this.saved.emit(res);
      return;
    }
    const first = this.saver.placement().fields[0];
    // The refused feature is opened, so its input exists when the focus goes there.
    const feature = first ? featureIdOfPath(this.draft().features, (k) => classFeatureBase(this.draft().features, k), first) : '';
    if (feature) {
      this.openFeature.set(feature);
    }
    afterNextRender(
      () => {
        if (first) {
          focusField(this.host.nativeElement, first);
        }
      },
      { injector: this.injector },
    );
  }
}
