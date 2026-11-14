import {
  ChangeDetectionStrategy,
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

import type {
  TableContentViolation,
  TableEntry,
} from '../../../../gen/meurpg/rules/v1/table_content_pb';
import { create } from '@bufbuild/protobuf';
import { FeatPrerequisiteSchema } from '../../../../gen/meurpg/rules/v1/rules_pb';
import { type CatalogVm } from '../../../core/content/catalog';
import { type EntryBody, TableContentClient } from '../../../core/content/content-client';
import { EntrySaver, focusAfterRender } from '../../../core/content/entry-saver';
import {
  type EffectDraft,
  type EffectMenuVm,
  choiceUses,
  featChoices,
  missingChoices,
} from '../../../core/content/effect-draft';
import { FEAT_OPTION_SETS, optionChoiceIndex } from '../../../core/content/feat-options';
import {
  type FeatDraft,
  type Scores,
  draftToFeat,
  emptyFeat,
  featPaths,
  featToDraft,
  unreadableFeatPaths,
} from '../../../core/content/feat-draft';
import {
  LACK_GROUPS,
  lackChips,
  lackOptions,
  pickableLacks,
  pickableRaces,
  raceChips,
} from '../../../core/content/feat-lookup';
import {
  ABILITY_FIELDS,
  type AbilityField,
  type FeatureDraft,
  SIZE_OPTIONS,
} from '../../../core/content/feature-draft';
import { askedRaces, prerequisiteParts } from '../../../core/content/feat-text';
import { previewRead } from '../../../core/content/preview';
import { pactChoices, requirableOptions } from '../../../core/content/requirable-options';
import { CheckRow } from '../../../shared/form-fields/check-row';
import { FeatureEditor } from '../../../shared/feature-editor/feature-editor';
import { PickList } from '../../../shared/form-fields/pick-list';
import { SearchPick } from '../../../shared/form-fields/search-pick';
import { SelectField, type SelectOption } from '../../../shared/form-fields/select-field';
import { TextField } from '../../../shared/form-fields/text-field';
import { EditorAlerts, EditorBar } from '../editor-bar/editor-bar';
import { EntryRead } from '../entry-read/entry-read';
import { FeatChoices } from './feat-choices/feat-choices';
import { FeatOptions } from './feat-options/feat-options';
import { OptionSource } from './option-source/option-source';
import { PlayersSwitch } from '../players-switch/players-switch';
import type { EditorSaved } from '../spell-editor/spell-editor';

const RACES = 'table_feat.prerequisite.races';
const LACKS = 'table_feat.prerequisite.lacks_feature_key';

/** The sizes a feat may ask for, as the server writes them. */
const FEAT_SIZES: SelectOption[] = [{ value: '', label: 'Qualquer' }, ...SIZE_OPTIONS];

type AskKind = 'class' | 'pact' | 'spell' | 'options';

interface Ask {
  kind: AskKind;
  label: string;
  icon: string;
  /** The first path is the one that takes the focus. */
  paths: readonly string[];
}

const ASK = 'table_feat.prerequisite';
const ASKS: readonly Ask[] = [
  {
    kind: 'class',
    label: 'Nível de classe',
    icon: 'shield',
    paths: [`${ASK}.class_level.class_key`, `${ASK}.class_level.level`, `${ASK}.class_level`],
  },
  { kind: 'pact', label: 'Dádiva do pacto', icon: 'bookmark', paths: [`${ASK}.requires_pact`] },
  {
    kind: 'spell',
    label: 'Magia que conhece',
    icon: 'auto_fix_high',
    paths: [`${ASK}.requires_spell_key`],
  },
  {
    kind: 'options',
    label: 'Outras opções',
    icon: 'format_list_bulleted',
    paths: [`${ASK}.requires_option_keys`],
  },
];
const ASK_PATH = /\.(class_level|requires_)/;
const NO_ASKS: Partial<FeatDraft> = {
  classKey: '',
  classLevel: '',
  requiresPact: '',
  requiresSpell: '',
  requiresOptions: [],
};
const CLEARED: Record<AskKind, Partial<FeatDraft>> = {
  class: { classKey: '', classLevel: '' },
  pact: { requiresPact: '' },
  spell: { requiresSpell: '' },
  options: { requiresOptions: [] },
};

function askFilled(kind: AskKind, d: FeatDraft): boolean {
  switch (kind) {
    case 'class':
      return d.classKey !== '' || d.classLevel !== '';
    case 'pact':
      return d.requiresPact !== '';
    case 'spell':
      return d.requiresSpell !== '';
    default:
      return d.requiresOptions.length > 0;
  }
}

/**
 * The feat editor: a name, a text, what the feat asks of a character, and its effects.
 * The prerequisite has ability scores, a proficiency, spellcasting, races, a size, a feature to lack and a level.
 * The effects come from the server's menu, with the types only a feat has.
 * A feat's choice of options takes them from the feat's own option panel, a whole SRD set or a list of SRD options.
 * "Salvar talento" sends the whole entry.
 * Whether a character meets the prerequisite is the server's to say.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-feat-editor',
  imports: [
    CheckRow,
    EditorAlerts,
    EditorBar,
    EntryRead,
    FeatChoices,
    FeatOptions,
    FeatureEditor,
    MatIconModule,
    OptionSource,
    PickList,
    PlayersSwitch,
    SearchPick,
    SelectField,
    TextField,
  ],
  templateUrl: './feat-editor.html',
  styleUrl: '../editor.scss',
})
export class FeatEditor {
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
  /** The entry's switch "Disponível para os jogadores" was turned (it saves at once, apart from the form). */
  readonly switched = output<TableEntry>();
  readonly reload = output<void>();
  readonly cancelled = output<void>();

  protected readonly draft = signal<FeatDraft>(emptyFeat());
  protected readonly saver = new EntrySaver(
    {
      aOne: 'um talento',
      nameOf: (key) => this.entries().find((e) => e.key === key)?.namePt ?? '',
    },
    'o talento',
  );
  protected readonly abilities = computed(() => this.catalog().abilities);
  protected readonly spellOptions = computed<SelectOption[]>(() => [...this.catalog().spells]);
  protected readonly proficiencyOptions = computed<SelectOption[]>(() => [
    { value: '', label: 'Nenhuma' },
    ...this.catalog().proficiencies,
  ]);
  protected readonly sizes = FEAT_SIZES;
  protected readonly classOptions = computed<SelectOption[]>(() => [
    { value: '', label: 'Nenhuma' },
    ...this.catalog().classes,
  ]);
  protected readonly requireSpellOptions = computed<SelectOption[]>(() => [
    { value: '', label: 'Nenhuma' },
    ...this.catalog().spells,
  ]);
  protected readonly pactOptions = computed<SelectOption[]>(() =>
    pactChoices(this.menu(), this.entries()),
  );
  protected readonly requirable = computed<SelectOption[]>(() =>
    requirableOptions(this.menu(), this.entries()),
  );
  /** The kinds of the "Adicionar pré-requisito" menu the master opened and has not filled yet. */
  private readonly opened = signal<readonly AskKind[]>([]);
  protected readonly menuOpen = signal(false);
  /** The rows on the list: the kinds with a value, a refusal, or that the master just added. */
  protected readonly askRows = computed(() => {
    const d = this.draft();
    return ASKS.filter(
      (a) =>
        this.opened().includes(a.kind) ||
        askFilled(a.kind, d) ||
        a.paths.some((p) => this.saver.issues(p).length > 0),
    );
  });
  protected readonly askMissing = computed(() =>
    ASKS.filter((a) => !this.askRows().some((r) => r.kind === a.kind)),
  );
  /** The block of the older fields opens by itself when a refusal lands inside it. */
  protected readonly oldOpen = signal(false);
  protected readonly oldRefused = computed(() =>
    [...this.refused()].some((p) => p.startsWith('table_feat.prerequisite') && !p.match(ASK_PATH)),
  );
  protected readonly oldNote = computed(() => {
    const parts = prerequisiteParts(
      create(
        FeatPrerequisiteSchema,
        draftToFeat({ ...this.draft(), ...NO_ASKS }, this.menu()).prerequisite,
      ),
      this.nameOf(),
    );
    return parts.length === 0
      ? 'Os campos de hoje · nada pedido'
      : `Os campos de hoje · ${parts.join(', ')}`;
  });
  /** The place of the feat's choice of options among its effects, or -1. */
  protected readonly choiceAt = computed(() => optionChoiceIndex(this.draft().feature.effects));
  protected readonly choice = computed<EffectDraft | null>(
    () => this.draft().feature.effects[this.choiceAt()] ?? null,
  );
  /** The feat's named choices: the fields of the effects after one may point at it. */
  protected readonly choices = computed(() => featChoices(this.draft().feature.effects));
  protected readonly uses = computed(() => choiceUses(this.draft().feature.effects, this.menu()));
  /** The fields that point at a choice that was removed or moved below them: nothing is re-pointed, and the save waits. */
  private readonly missing = computed(() =>
    missingChoices(this.draft().feature.effects, this.menu()),
  );
  /** Why "Salvar talento" waits: the page's reason, else a field whose choice is gone. */
  protected readonly blockReason = computed(() => {
    const n = this.missing().length;
    if (this.saveBlocked() || n === 0) {
      return this.saveBlocked();
    }
    const who = n === 1 ? 'Um campo usava' : `${n} campos usavam`;
    return `${who} uma escolha que foi removida ou ficou abaixo dele. Escolha outro valor.`;
  });
  /** The set the choice offers whole, by its name, or empty. */
  protected readonly setName = computed(() => {
    const set = this.choice()?.set ?? '';
    return set === '' ? '' : this.menu().nameOf(FEAT_OPTION_SETS, set);
  });
  /** The paths the last refusal landed on: a folded option with one inside opens. */
  protected readonly refused = computed(() => this.saver.placement().fields);
  protected readonly lackGroups = LACK_GROUPS;
  private readonly lackAll = computed(() =>
    lackOptions(this.menu().list('lackable_features'), this.entries(), this.catalog().nameOf),
  );
  /** The name of any key the preview or a chip writes: the SRD's features are in the menu, not in the catalog. */
  private readonly nameOf = computed(() => {
    const srd = new Map(this.lackAll().map((o) => [o.value, o.label]));
    const catalog = this.catalog();
    return (key: string) => srd.get(key) ?? catalog.nameOf(key);
  });
  /** The races asked: a feat's single race_key stands in the list as one more chip. */
  private readonly asked = computed(() => askedRaces(this.draft().raceKey, this.draft().races));
  /** The key of each race the server refused, with the path of its refusal. */
  private readonly refusedRaces = computed(() => {
    const d = this.draft();
    const out = new Map<string, string>();
    const at = (key: string, path: string) => {
      if (key !== '' && this.saver.issues(path).length > 0) {
        out.set(key, path);
      }
    };
    at(d.raceKey, 'table_feat.prerequisite.race_key');
    d.races.forEach((key, i) => at(key, `${RACES}[${i}]`));
    return out;
  });
  protected readonly raceChipList = computed(() =>
    raceChips(this.asked(), this.entries(), this.nameOf(), this.refusedRaces()),
  );
  protected readonly raceSearch = computed(() =>
    pickableRaces(this.catalog().raceChoices, this.asked()),
  );
  /** Every refusal about the races, each text once: of the list, of one race and of the single race. */
  protected readonly raceIssues = computed(() => {
    const paths = [
      RACES,
      'table_feat.prerequisite.race_key',
      ...this.draft().races.map((_, i) => `${RACES}[${i}]`),
    ];
    return [...new Set(paths.flatMap((p) => this.saver.issues(p)))];
  });
  protected readonly lackSearch = computed(() =>
    pickableLacks(this.lackAll(), this.entry()?.key ?? '', this.draft().lacksFeatureKey),
  );
  protected readonly lackChipList = computed(() =>
    lackChips(
      this.draft().lacksFeatureKey,
      this.lackAll(),
      this.nameOf(),
      this.saver.issues(LACKS).length > 0,
    ),
  );
  /** What a player reads, written by the same function as the player's page. */
  protected readonly preview = computed(() =>
    previewRead(
      { case: 'tableFeat', value: draftToFeat(this.draft(), this.menu()) },
      this.nameOf(),
    ),
  );

  constructor() {
    // Keyed on the entry's key and revision, never on the object: archiving hands the page a new object with the same revision,
    // and the master's unsaved draft must stay.
    const source = computed(() => {
      const e = this.entry();
      return e ? `${e.key}@${e.revision}` : 'new';
    });
    effect(() => {
      source();
      untracked(() => {
        const e = this.entry();
        this.draft.set(e?.body.case === 'tableFeat' ? featToDraft(e.body.value) : emptyFeat());
        this.saver.clear();
        this.opened.set([]);
        this.menuOpen.set(false);
        this.seedLeftOut();
      });
    });
  }

  /** The rules refuse a field of this entry, so the campaign is built without it: the field says why, and the focus goes to it. */
  private seedLeftOut(): void {
    if (this.saver.seedLeftOut(this.leftOut(), this.known)) {
      this.saver.focusRefusal(this.host.nativeElement, this.injector);
    }
  }

  protected addAsk(kind: AskKind): void {
    this.opened.update((o) => [...o, kind]);
    this.menuOpen.set(false);
    focusAfterRender(
      this.host.nativeElement,
      this.injector,
      ASKS.find((a) => a.kind === kind)?.paths[0] ?? '',
    );
  }

  protected clearAsk(kind: AskKind): void {
    this.opened.update((o) => o.filter((k) => k !== kind));
    this.patch(CLEARED[kind]);
  }

  protected patch(p: Partial<FeatDraft>): void {
    this.draft.update((d) => ({ ...d, ...p }));
  }

  protected setFeature(feature: FeatureDraft): void {
    this.patch({ feature });
  }

  protected setOptions(options: FeatureDraft[]): void {
    this.patch({ options });
  }

  /** The feat's choice of options after the cards changed it. */
  protected setChoice(effect: EffectDraft): void {
    const feature = this.draft().feature;
    const effects = feature.effects.map((e, i) => (i === this.choiceAt() ? effect : e));
    this.patch({ feature: { ...feature, effects } });
  }

  /** A race is added to the list; a feat's single race_key moves into it, so the server never gets both. */
  protected addRace(key: string): void {
    const d = this.draft();
    this.patch({ raceKey: '', races: [...askedRaces(d.raceKey, d.races), key] });
  }

  protected removeRace(key: string): void {
    const d = this.draft();
    this.patch(d.raceKey === key ? { raceKey: '' } : { races: d.races.filter((r) => r !== key) });
  }

  protected setScore(which: 'minimums' | 'anyOf', field: AbilityField, text: string): void {
    const scores: Scores = { ...this.draft()[which], [field]: text };
    this.patch({ [which]: scores });
  }

  protected readonly fields = ABILITY_FIELDS;
  protected readonly issuesOf = (path: string): readonly string[] => this.saver.issues(path);

  private readonly known = (path: string): boolean =>
    featPaths(this.draft(), this.menu()).includes(path);

  protected async save(): Promise<void> {
    if (this.saver.saving() || this.blockReason()) {
      return;
    }
    if (
      this.saver.blockOn(unreadableFeatPaths(this.draft(), this.menu()), (path) =>
        focusAfterRender(this.host.nativeElement, this.injector, path),
      )
    ) {
      return;
    }
    const body: EntryBody = {
      case: 'tableFeat',
      value: draftToFeat(this.draft(), this.menu()),
    };
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
