import {
  Component,
  DestroyRef,
  ElementRef,
  Injector,
  ViewChild,
  afterNextRender,
  computed,
  effect,
  inject,
  signal,
  untracked,
} from '@angular/core';
import { takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
import { FormBuilder, ReactiveFormsModule } from '@angular/forms';
import { CdkStep } from '@angular/cdk/stepper';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatDialog } from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatRadioModule } from '@angular/material/radio';
import { MatSelectModule } from '@angular/material/select';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';

import { contentRef, characterBlockedMessage } from '../../core/characters/character-errors';
import { ContentWatcher } from '../../core/content/content-watcher';
import { LiveSessionSourceLive } from '../../core/play/live-session-source.live';
import { offControlOf } from './catalog-changes';
import { spellLevelLabel } from '../../core/characters/character-labels';
import { isFullSheetKind } from '../../core/characters/characters.types';
import { formatXp } from '../../core/format/text';
import {
  applySelection,
  missingSentence,
  pendingLabels,
  withChoiceText,
} from '../../shared/choice-groups/choice-picks';
import {
  ChoiceGroups,
  type ChoiceSelection,
  type ChoiceTextEdit,
} from '../../shared/choice-groups/choice-groups';
import { FictionNotice } from '../../shared/fiction-notice/fiction-notice';
import { TableMark } from '../../shared/table-mark/table-mark';
import { AbilityFields } from './ability-fields/ability-fields';
import { AbilityScores } from './ability-scores/ability-scores';
import { TableAbilityScores } from './table-ability-scores/table-ability-scores';
import {
  ALIGNMENT_LABELS,
  AbilityMethodKey,
  AbilityTableVm,
  AlignmentKey,
  CharacterEditorSource,
  CharacterForEdit,
  CharacterFormValue,
  ExtraClassValue,
  RulesCatalogVm,
  SpellOptionVm,
} from './character-editor.types';
import { ClassBlockFields } from './class-block/class-block';
import { GrantedSpells } from './granted-spells/granted-spells';
import {
  ClassBlock,
  MAX_TOTAL_LEVEL,
  type CasterSection,
  blocksOf,
  casterSections,
  hitDiceAfterFirst,
  offered,
  unlisted,
  sectionName,
  totalLevel,
} from './class-blocks';
import type { SpellDetailsData } from '../../shared/spell-details/spell-details';
import { openSpellDetails } from '../../shared/spell-details/open-spell-details';
import type { SpellDetailsVm } from '../../shared/spell-details/spell-details.types';
import {
  catalogNameOf,
  chosenSkillsOf,
  choiceBonusesHintOf,
  choicesSubtitleOf,
  finalConstitutionOf,
  grantedSkillsOf,
  sectionHeadingOf,
  skillNoteOf,
  subclassHintOf,
  subclassWaits,
} from './catalog-views';
import { CUTTING_WORDS_OPTIONS } from './editor-options';
import { EditorLoader } from './editor-loader';
import {
  type PageState,
  type ReadyState,
  actionsNoteOf,
  cancelLinkOf,
  createLabelOf,
  leadOf,
  pageTitleOf,
  previewInputOf,
  startingXp,
  tableHitPointsMethod,
} from './editor-state';
import { EditorStepper } from './editor-stepper/editor-stepper';
import { HitPointsRolls } from './hit-points-rolls/hit-points-rolls';
import { ServerHitPoints } from './hit-points-server';
import { ServerChoices } from './server-choices';
import {
  EDITOR_STEP_LABELS,
  EditorField,
  EditorStepKey,
  countLabel,
  describeBonusesInUse,
  describeInvalidFields,
} from './editor-labels';
import {
  buildFullValue,
  choicesAfterAnswer,
  classProblemsOf,
  invalidFullFieldsOf,
  rollProblemsOf,
  serverDraftNeeded,
} from './full-draft';
import { createFullForm, fullFormValueOf } from './full-form';
import {
  basicFormToValue,
  createBasicForm,
  invalidBasicFields,
  patchBasicForm,
} from './npc-short-form/basic-form';
import { SaveFlow } from './save-flow';
import {
  CUSTOM_BACKGROUND_PROFICIENCIES,
  CUSTOM_BACKGROUND_SKILLS,
  toggledCapped,
  toggledExpertise,
  toggledSkill,
} from './skill-picks';
import {
  availableCantripsOf,
  availableSpellsOf,
  buildSectionView,
  filterByName,
  keepAvailable,
  sectionExtrasOf,
} from './spell-views';
import { SpellPicks } from './spell-picks';
import { DefeatXp } from './defeat-xp/defeat-xp';
import { FeatsField } from './feats-field/feats-field';
import { PortraitField } from './portrait-field/portrait-field';
import { NpcShortForm } from './npc-short-form/npc-short-form';
import { SkillPicker } from './skill-picker/skill-picker';
import { SpellPicker } from './spell-picker/spell-picker';

/** The server cannot derive a draft while the page has no lists to read it by. */
function pageNotReady(): Promise<never> {
  return Promise.reject(new Error('the page is not ready'));
}

/**
 * The character editor (MR-003, MR-005, MR-006): a stepper for a full
 * sheet (player, enemy, boss) — Básico, Habilidades, Perícias, Magias (only
 * for a caster class), Equipamento, `EditorStepper` over the CDK stepper —
 * or a single short form for a basic sheet (minion, story, `NpcShortForm`),
 * routed from three places (plan §5):
 *
 * - `/campaigns/:id/characters/new` — a player creates their character.
 * - `/campaigns/:id/npcs/new/:kind` — a master creates an NPC.
 * - `/campaigns/:id/characters/:characterId/edit` — either edits.
 *
 * The submit action sits outside the stepper, right under the open step,
 * so the whole form can be saved from any step. A submit with an invalid
 * field lists what to fix (`invalidSummary`), marks the steps that have
 * one, and opens the first of them. RN-01 is enforced on the server: this
 * page opens an existing character only when `Character.can_edit` says the
 * caller may save it (otherwise it shows why, as `character-sheet` does),
 * and still renders whatever `describeCharacterError` maps a
 * `failed_precondition` / `SHEET_LOCKED` response to, for a sheet locked
 * while the form was open. No D&D rule runs here: the page never shows a
 * modifier, CA or PV it computed itself.
 *
 * A custom background's two granted skills (`CustomBackground.skill_keys`)
 * are collected separately from the player's own skill proficiencies —
 * `customBackgroundSkills`, capped at 2 (`toggleCustomBackgroundSkill`) —
 * and sent only once exactly two are chosen; the server's advisory check
 * (`DerivedSheet.issues`) covers 0 or 1. Expertise (`expertiseSkillKeys`)
 * is a further, separate selection: always a subset of the proficient
 * skills (`toggleExpertise`, disabled on a skill that is not proficient).
 * `feature_choice_keys` has no field at all: see
 * `character-editor.types.ts`'s doc comment on `CharacterFormValue` for
 * why — everything else `FullSheet` has, this form now covers.
 *
 * **No data loss on save** (integrator fix, phase 2b): on an edit,
 * `CharacterEditorSourceLive.updateCharacter` starts from the `FullSheet`
 * it loaded and overwrites only the fields this form actually edits —
 * `coins` and `feature_choice_keys` (which this form has no UI for) and
 * any class beyond the first (multiclassing; this form only ever edits
 * one) survive a save unchanged instead of being silently wiped. See
 * `character-editor-source.live.spec.ts`'s round-trip test.
 *
 * **Never a typed content key** (integrator fix): race, subrace, class,
 * subclass, background, skills, armor, weapons, cantrips and spells are
 * all picked from `RulesCatalogVm` (`ContentService.ListContent`) —
 * `namePt` shown, `key` sent — never typed as free text, since a typed
 * name almost never matches the real key and the server rejects it with
 * `invalid_argument`. Cantrips and the known/prepared spell lists are
 * filterable checkbox pickers (`availableCantrips`/`availableSpells`,
 * `filteredCantrips`/`filteredSpellsKnown`/`filteredSpellsPrepared`),
 * narrowed to the chosen class's spell list; armor is a select (with "Sem
 * armadura"); weapons is a `<mat-select multiple>`. Only genuinely free
 * text stays free text — see `CharacterFormValue`'s doc comment.
 */
@Component({
  selector: 'app-character-editor',
  imports: [
    ChoiceGroups,
    ClassBlockFields,
    GrantedSpells,
    TableMark,
    AbilityFields,
    AbilityScores,
    TableAbilityScores,
    CdkStep,
    EditorStepper,
    FictionNotice,
    HitPointsRolls,
    MatButtonModule,
    MatCheckboxModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatProgressSpinnerModule,
    MatRadioModule,
    MatSelectModule,
    DefeatXp,
    FeatsField,
    NpcShortForm,
    PortraitField,
    ReactiveFormsModule,
    RouterLink,
    SkillPicker,
    SpellPicker,
  ],
  providers: [ContentWatcher, LiveSessionSourceLive],
  templateUrl: './character-editor.html',
  styleUrl: './character-editor.scss',
})
export class CharacterEditor {
  private readonly source = inject(CharacterEditorSource);
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly destroyRef = inject(DestroyRef);
  private readonly fb = inject(FormBuilder);
  private readonly dialog = inject(MatDialog);
  private readonly bottomSheet = inject(MatBottomSheet);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);
  // The session's stream says when the table's content changes (10.1d, RN-23): one stream, one read per hint, with the debounce.
  private readonly watcher = inject(ContentWatcher);
  /** The spell descriptions already fetched, by spell key, for the life of
   * the page (a second "?" on the same spell is instant; nothing is stored
   * in the browser). A failed fetch is dropped so "Tentar de novo" asks again. */
  private readonly spellDetails = new Map<string, Promise<SpellDetailsVm>>();

  @ViewChild(EditorStepper) private stepper?: EditorStepper;
  @ViewChild(ChoiceGroups) private choiceGroups?: ChoiceGroups;

  protected readonly state = signal<PageState>({ status: 'loading', title: 'Ficha' });
  /** The lists of the catalog while the page is open on a sheet, else `null`. */
  private readonly readyCatalog = computed(() => {
    const s = this.state();
    return s.status === 'ready' ? s.catalog : null;
  });

  /** The save: the request, the navigation after it and what a refusal does (`SaveFlow`). */
  private readonly saving = new SaveFlow({
    source: this.source,
    navigate: (commands) => this.router.navigate(commands),
    full: () => this.buildFullValue(),
    basic: () => basicFormToValue(this.basicForm),
    name: (isBasic) =>
      isBasic ? this.basicForm.getRawValue().name : this.fullForm.getRawValue().name,
    abilityMethod: () => (this.abilityTable() ? this.abilityMethod() : null),
    stillHere: () => this.loader.stillHere(),
    nameOfKey: (key) => this.nameOfKey(key),
    markSwitchedOff: (key) => this.markSwitchedOff(key),
    refreshCatalog: () => this.refreshCatalog(),
  });
  /** The class block a refusal points at (`full.classes[i]`), so the block says so. */
  protected readonly serverClassProblem = this.saving.classProblem;
  protected readonly saveState = this.saving.state;
  protected readonly selectedSkills = signal<ReadonlySet<string>>(new Set());
  /** The custom background's two granted skills (`CustomBackground.skills`,
   * integrator fix, phase 2) — a separate, capped-at-2 selection from
   * `selectedSkills`, only shown when `background === 'custom'`. */
  protected readonly customBackgroundSkills = signal<ReadonlySet<string>>(new Set());
  /** The "Outro" background's two tools or languages (content keys), capped at 2 like its skills. */
  protected readonly customBackgroundProficiencies = signal<readonly string[]>([]);
  /** The classes after the first (multiclass at creation): one block each, see `ClassBlockFields`. */
  protected readonly extraClasses = signal<readonly ExtraClassValue[]>([]);
  /** Expertise doubles a skill's proficiency bonus — always a subset of
   * `selectedSkills` (integrator fix, phase 2b). */
  protected readonly expertiseSkills = signal<ReadonlySet<string>>(new Set());
  /** One roll per level after the first, index 0 = level 2. Only sent when
   * `hitPointsMethod` is "rolled" (integrator fix, phase 2b). */
  protected readonly hitPointsRolls = signal<readonly number[]>([]);
  /** What the server said about the draft's feats (a prerequisite not met), by feat key. */
  protected readonly featIssues = computed(() => this.serverHitPoints.answer()?.featIssues ?? {});
  /** The hit points the server derives for the draft while the "Pontos de vida" box is on screen. */
  private readonly serverHitPoints = new ServerHitPoints(() => {
    const s = this.state();
    return s.status === 'ready'
      ? this.source.previewCharacter(previewInputOf(s, this.buildFullValue(), true))
      : pageNotReady();
  });
  /** What the server says the effects add to the hit points of the draft; `null` while it has no answer (the box adds up by itself). */
  protected readonly hitPointsFromEffects = computed(
    () => this.serverHitPoints.answer()?.hitPointsFromEffects ?? null,
  );

  /** The picks of the class and race choices, in the order they were made, and their free texts (PM-05). */
  protected readonly featureChoiceKeys = signal<readonly string[]>([]);
  /** The sheet's feats and the ones taken in place of an improvement (MR-025): the master's "Talentos" section edits them. */
  protected readonly featKeys = signal<readonly string[]>([]);
  protected readonly featSlots = signal<Readonly<Record<string, string>>>({});
  protected readonly featureChoiceText = signal<Readonly<Record<string, string>>>({});
  /** The choices the server reads in the draft while the "Escolhas" step may be on screen. */
  private readonly serverChoices = new ServerChoices(() => {
    const s = this.state();
    return s.status === 'ready'
      ? this.source.previewChoices(previewInputOf(s, this.buildFullValue(), false))
      : pageNotReady();
  });
  protected readonly choicesAnswer = this.serverChoices.answer.asReadonly();
  protected readonly choiceGroupsOfDraft = computed(() => this.choicesAnswer()?.groups ?? []);
  /** The step exists only when the race or a class asks a choice (a human rogue 1 has none). */
  protected readonly choicesStep = computed(() => this.choiceGroupsOfDraft().length > 0);
  private readonly pendingChoices = computed(() => pendingLabels(this.choiceGroupsOfDraft()));
  /** The tab carries the "!" while a choice is open. */
  protected readonly pendingSteps = computed(() =>
    this.choicesStep() && this.pendingChoices().length > 0 ? [EDITOR_STEP_LABELS.escolhas] : [],
  );
  /** The steps built from the start: the ones whose content the save depends on. */
  protected readonly eagerSteps = computed(() => [
    ...(this.abilityTable() ? [EDITOR_STEP_LABELS.atributos] : []),
    ...(this.choicesStep() ? [EDITOR_STEP_LABELS.escolhas] : []),
  ]);
  /** A player's sheet cannot be saved with a choice open (the server refuses it); the master's NPCs and edits can. */
  protected readonly choicesBlocked = computed(() => {
    const s = this.state();
    return (
      s.status === 'ready' &&
      s.kind === 'player' &&
      !s.catalog.viewerIsMaster &&
      this.pendingChoices().length > 0
    );
  });
  /** "Falta uma escolha: Estilo de Luta.", the description of the blocked button. */
  protected readonly missingText = computed(() => missingSentence(this.choiceGroupsOfDraft()));
  /** The line next to the counter of the step: "Tharn · Guerreiro 1 · Draconato". */
  protected readonly choicesSubtitle = computed(() => {
    this.formChanges();
    if (this.state().status !== 'ready') {
      return '';
    }
    const v = this.fullForm.getRawValue();
    return choicesSubtitleOf(
      v.name,
      this.selectedClass(),
      v.level,
      this.selectedRace(),
      this.selectedSubrace(),
    );
  });

  /** Catalog-backed pickers (integrator fix: the editor must never make a
   * person type a content key) — cantrips and the known/prepared spell
   * lists, each a filterable checkbox list over `RulesCatalogVm.spells`. */
  private readonly picks = new SpellPicks({ sections: () => this.sections() });
  protected readonly selectedCantrips = this.picks.cantrips;
  protected readonly selectedSpellsKnown = this.picks.known;
  protected readonly selectedSpellsPrepared = this.picks.prepared;
  /** Spells the sheet has that neither a pick of this form nor its subclass gave it (a race's or a feature's spell):
   * only known on an edit, from the server's own derived sheet, as it was when the sheet was opened. */
  protected readonly grantedSpells = signal<readonly SpellOptionVm[]>([]);
  /** The saved sheet has Palavras de Interrupção (a College of Lore bard): its "Perguntar" setting shows. Known on an edit. */
  protected readonly hasCuttingWords = signal(false);
  protected readonly cuttingWordsOptions = CUTTING_WORDS_OPTIONS;

  protected readonly isFullSheetKind = isFullSheetKind;
  protected readonly stepLabels = EDITOR_STEP_LABELS;
  protected readonly alignmentKeys: readonly AlignmentKey[] = [
    '',
    'lawful_good',
    'neutral_good',
    'chaotic_good',
    'lawful_neutral',
    'neutral',
    'chaotic_neutral',
    'lawful_evil',
    'neutral_evil',
    'chaotic_evil',
  ];
  protected readonly alignmentLabel = (key: AlignmentKey) => ALIGNMENT_LABELS[key];

  protected readonly fullForm = createFullForm(this.fb);

  protected readonly basicForm = createBasicForm(this.fb);

  /** The locked sheet's XP, to read ("2.716 XP"): only the master's awards change it. */
  private readonly experienceXp = toSignal(this.fullForm.controls.experiencePoints.valueChanges, {
    initialValue: 0,
  });
  protected readonly experienceLabel = computed(() => formatXp(this.experienceXp()));

  /** "Rolar 4d6" or "Conjunto padrão" with results still to place: saving
   * waits, so a half-placed roll never turns into six default 10s. */
  protected readonly abilitiesIncomplete = signal(false);
  /** What the table's way of making scores still lacks, in words ("role as habilidades"), `''` when it is complete. */
  protected readonly abilitiesProblem = signal('');
  /** The table's ways of making scores, when a player makes a new sheet (RN-24); `null` is the free editor of the master's NPCs and of an edit. */
  protected readonly abilityTable = signal<AbilityTableVm | null>(null);
  /** The way the player chose; the server checks the scores against it. */
  protected readonly abilityMethod = signal<AbilityMethodKey>('typed');
  /** A player editing their own draft: the way the server recorded the scores were made, which the step keeps (RN-24). */
  protected readonly lockedOrigin = signal<NonNullable<CharacterForEdit['abilityOrigin']> | null>(
    null,
  );
  /** What the table leaves of the hit points of a new sheet: both ways, only "Rolado" or only "Média". */
  protected readonly hpRule = computed<'player_chooses' | 'roll' | 'average'>(() => {
    const s = this.state();
    return s.status === 'ready' && s.mode === 'create'
      ? (this.abilityTable()?.hitPoints ?? 'player_chooses')
      : 'player_chooses';
  });

  /** Any change of the form's fields; the signals the draft is made of are read where it is built. */
  private readonly formChanges = toSignal(this.fullForm.valueChanges);
  protected readonly selectedRaceKey = toSignal(this.fullForm.controls.race.valueChanges, {
    initialValue: '',
  });
  protected readonly selectedClassKey = toSignal(this.fullForm.controls.className.valueChanges, {
    initialValue: '',
  });
  protected readonly selectedBackground = toSignal(this.fullForm.controls.background.valueChanges, {
    initialValue: '',
  });
  private readonly selectedLevel = toSignal(this.fullForm.controls.level.valueChanges, {
    initialValue: 1,
  });
  protected readonly selectedSubclassKey = toSignal(
    this.fullForm.controls.subclassName.valueChanges,
    {
      initialValue: '',
    },
  );
  private readonly selectedCustomSubclass = toSignal(
    this.fullForm.controls.customSubclassName.valueChanges,
    { initialValue: '' },
  );
  protected readonly selectedHitPointsMethod = toSignal(
    this.fullForm.controls.hitPointsMethod.valueChanges,
    { initialValue: 'average' as const },
  );
  /** Every class of the sheet, in the order of the blocks: the first one is the form's own fields. */
  protected readonly blocks = computed<ClassBlock[]>(() =>
    blocksOf(
      {
        classKey: this.selectedClassKey(),
        level: this.selectedLevel(),
        subclassKey: this.selectedSubclassKey(),
        customSubclassName: this.selectedCustomSubclass(),
      },
      this.extraClasses(),
    ),
  );
  /** More than one class: the Básico step shows one block per class and the total level. */
  protected readonly multiclass = computed(() => this.extraClasses().length > 0);
  protected readonly totalLevelValue = computed(() => totalLevel(this.blocks()));
  /** The level the whole sheet has, up to 20, or why it does not fit. */
  protected readonly totalLevelProblem = computed(() =>
    this.totalLevelValue() > MAX_TOTAL_LEVEL ? `O nível total vai até ${MAX_TOTAL_LEVEL}.` : '',
  );
  /** How many rolls "Dados de Vida" needs: one per level after the first, of all the classes together. */
  protected readonly rollsNeeded = computed(() => Math.max(0, this.totalLevelValue() - 1));
  /** The die of each of those levels, which differs between the classes of a multiclass. */
  protected readonly levelDice = computed(() => {
    const catalog = this.readyCatalog();
    return catalog ? hitDiceAfterFirst(catalog, this.blocks()) : [];
  });
  protected readonly selectedSubraceKey = toSignal(this.fullForm.controls.subrace.valueChanges, {
    initialValue: '',
  });
  private readonly baseScores = toSignal(this.fullForm.controls.abilities.valueChanges, {
    initialValue: this.fullForm.controls.abilities.getRawValue(),
  });

  protected readonly availableSubraces = computed(
    () => this.readyCatalog()?.races.find((r) => r.key === this.selectedRaceKey())?.subraces ?? [],
  );

  /** The Portuguese name of any entry of the catalog, for a message that names one (never the key itself). */
  private nameOfKey(key: string): string | undefined {
    return catalogNameOf(this.readyCatalog() ?? undefined, key);
  }

  /** The caller is the master: the one who is offered what is switched off for the players. */
  protected readonly master = computed(() => {
    const s = this.state();
    return s.status === 'ready' && s.catalog.viewerIsMaster;
  });
  /** What each list offers as a NEW choice: never a retired entry, unless it is the value the form already has. */
  protected readonly raceOptions = computed(() =>
    offered(this.readyCatalog()?.races ?? [], [this.selectedRaceKey()], this.master()),
  );
  protected readonly subraceOptions = computed(() =>
    offered(this.availableSubraces(), [this.selectedSubraceKey()], this.master()),
  );
  protected readonly classOptions = computed(() =>
    offered(this.readyCatalog()?.classes ?? [], [this.selectedClassKey()], this.master()),
  );
  protected readonly subclassOptions = computed(() =>
    offered(this.selectedClass()?.subclasses ?? [], [this.selectedSubclassKey()], this.master()),
  );
  protected readonly backgroundOptions = computed(() =>
    offered(this.readyCatalog()?.backgrounds ?? [], [this.selectedBackground()], this.master()),
  );
  /** A value the catalog does not list at all (a key from an old sheet): the select says so instead of staying blank. */
  protected readonly raceUnlisted = computed(() =>
    unlisted(this.readyCatalog()?.races ?? [], this.selectedRaceKey()),
  );
  protected readonly classUnlisted = computed(() =>
    unlisted(this.readyCatalog()?.classes ?? [], this.selectedClassKey()),
  );
  protected readonly backgroundUnlisted = computed(
    () =>
      this.selectedBackground() !== 'custom' &&
      unlisted(this.readyCatalog()?.backgrounds ?? [], this.selectedBackground()),
  );

  protected readonly selectedClass = computed(() =>
    this.readyCatalog()?.classes.find((c) => c.key === this.selectedClassKey()),
  );

  /** Faces of the chosen class's hit die, or 0 before a class is chosen. */
  protected readonly hitDie = computed(() => this.selectedClass()?.hitDie ?? 0);

  /** The Constitution the rolled hit points use (the same sum the server makes; this one only feeds the preview). */
  protected readonly finalConstitution = computed(() =>
    finalConstitutionOf(
      this.baseScores().con,
      this.selectedRace(),
      this.selectedSubrace(),
      this.bonusesValue().con,
    ),
  );

  /** The spell lists the sheet reads from: one section per class that casts, or per subclass that
   * casts (a third caster), from its level. Empty for a sheet that casts nothing: no "Magias" step. */
  protected readonly sections = computed<CasterSection[]>(() => {
    const catalog = this.readyCatalog();
    return catalog ? casterSections(catalog, this.blocks()) : [];
  });
  protected readonly isCaster = computed(() => this.sections().length > 0);
  /** The heading of a section when there is more than one: "Clérigo · 1º nível". */
  protected sectionHeading(section: CasterSection): string {
    return sectionHeadingOf(section);
  }
  protected sectionLabel = sectionName;

  /** Every cantrip and spell of the sheet's lists, for "the chosen line" and for what a save may send. */
  protected readonly availableCantrips = computed(() => {
    const catalog = this.readyCatalog();
    return catalog ? availableCantripsOf(catalog, this.sections(), this.selectedCantrips()) : [];
  });
  protected readonly availableSpells = computed(() => {
    const catalog = this.readyCatalog();
    return catalog ? availableSpellsOf(catalog, this.sections(), this.sectionExtras()) : [];
  });
  /** For each section, the spells outside its list that `PreviewChoices` lets it pick. */
  private readonly sectionExtras = computed(() => {
    const catalog = this.readyCatalog();
    if (!catalog) {
      return [];
    }
    const picked = new Set([...this.selectedSpellsKnown(), ...this.selectedSpellsPrepared()]);
    return sectionExtrasOf(
      catalog,
      this.sections(),
      this.choicesAnswer()?.spellSources ?? [],
      picked,
      this.master(),
    );
  });

  /** What each section's pickers list now (`buildSectionView`). */
  protected readonly sectionViews = computed(() => {
    const catalog = this.readyCatalog();
    if (!catalog) {
      return [];
    }
    const sections = this.sections();
    return sections.map((section) =>
      buildSectionView({
        catalog,
        sections,
        section,
        cantrips: this.selectedCantrips(),
        known: this.selectedSpellsKnown(),
        prepared: this.selectedSpellsPrepared(),
        master: this.master(),
        filters: this.picks.filters(),
        extras: this.sectionExtras(),
        // The draft's own numbers when the server has answered; else the saved sheet's prepared number (an edit).
        serverLimits: this.spellLimits().get(section.classKey),
        savedPreparedMax: this.preparedMaxByClass()[section.classKey],
      }),
    );
  });
  /** How many spells each casting class prepares, from the saved sheet's derived numbers (an edit only). */
  protected readonly preparedMaxByClass = signal<Readonly<Record<string, number>>>({});
  /** What the server says each casting class of the draft knows and prepares at its level (class table and the
   * SRD's formulas); empty while it has no answer. */
  private readonly spellLimits = computed(
    () => new Map((this.serverHitPoints.answer()?.spellcasting ?? []).map((l) => [l.classKey, l])),
  );

  /** What the sheet has that no pick of this form gave it and no subclass of the catalog explains (a race's or a
   * feature's spell): said apart, with where it comes from. */
  protected readonly otherGranted = this.grantedSpells.asReadonly();

  /** The search of an extra list ("0:patron"), and what it narrows. */
  protected spellFilterOf(id: string): string {
    return this.picks.filterOf(id);
  }
  protected setExtraFilter(id: string, value: string): void {
    this.picks.setFilter(id, value);
  }
  protected filterSpells(spells: readonly SpellOptionVm[], id: string): readonly SpellOptionVm[] {
    return filterByName(spells, this.picks.filterOf(id));
  }

  protected setSpellFilter(
    section: number,
    list: 'cantrips' | 'known' | 'prepared',
    value: string,
  ): void {
    this.picks.setFilter(`${section}:${list}`, value);
  }

  /** The entries chosen in the closed selects, for the "Da mesa" tag beside the name (a select shows only text). */
  protected readonly selectedRace = computed(() =>
    this.readyCatalog()?.races.find((r) => r.key === this.selectedRaceKey()),
  );
  protected readonly selectedSubrace = computed(() =>
    this.availableSubraces().find((r) => r.key === this.selectedSubraceKey()),
  );
  protected readonly selectedSubclass = computed(() =>
    this.selectedClass()?.subclasses.find((c) => c.key === this.selectedSubclassKey()),
  );
  protected readonly selectedBackgroundVm = computed(() =>
    this.readyCatalog()?.backgrounds.find((b) => b.key === this.selectedBackground()),
  );

  /** The skills the race, the subrace and the background give, with where each comes from (`grantedSkillsOf`). */
  protected readonly grantedSkills = computed<ReadonlyMap<string, string>>(() =>
    grantedSkillsOf(
      this.selectedRace(),
      this.selectedSubrace(),
      this.selectedBackgroundVm(),
      this.selectedBackground() === 'custom',
      Array.from(this.customBackgroundSkills()),
    ),
  );
  /** The skills the player picked, without the ones already given. */
  protected readonly chosenSkills = computed(() =>
    chosenSkillsOf(this.selectedSkills(), this.grantedSkills()),
  );

  /** What the first class asks of the skills and the saving throws, from the server's entry for it. */
  protected readonly skillNote = computed(() => skillNoteOf(this.selectedClass()));
  protected readonly spellCircle = spellLevelLabel;
  /** "Ver em Magias", for a spell the lists leave out. */
  protected readonly magiasLink = computed(() => {
    const s = this.state();
    return s.status === 'ready' ? ['/campaigns', s.campaignId, 'spells'] : [];
  });

  /** The race's "+2 e +1 à sua escolha": where the player puts them (the server's numbers, from the catalog). */
  protected readonly choiceBonusesHint = computed(() => choiceBonusesHintOf(this.selectedRace()));
  /** A table background's equipment, as the master wrote it. */
  protected readonly backgroundEquipment = computed(
    () => this.selectedBackgroundVm()?.equipmentPt ?? '',
  );
  protected readonly toolOptions = computed(() => this.toolsAndLanguages('tool'));
  protected readonly languageOptions = computed(() => this.toolsAndLanguages('language'));
  private toolsAndLanguages(kind: 'tool' | 'language') {
    return this.readyCatalog()?.toolsAndLanguages.filter((t) => t.kind === kind) ?? [];
  }

  /** The hint under "Subclasse" while the level is below the one where the
   * class chooses it; empty once the level reaches it. */
  protected readonly subclassHint = computed(() =>
    subclassHintOf(this.selectedClass(), this.selectedLevel()),
  );

  /** The player picked another class: a subclass belongs to one class, so the
   * old one (SRD or custom) must not survive. Wired to the select's
   * `selectionChange`, not `valueChanges`, because loading a saved sheet also
   * sets the class and must keep its subclass. */
  protected onClassChange(): void {
    this.fullForm.patchValue({ subclassName: '', customSubclassName: '' });
  }

  /** A block of the multiclass layout changed: block 0 is the form's own fields, the rest the extra classes. */
  protected changeBlock(index: number, change: Partial<ClassBlock>): void {
    if (index === 0) {
      this.patchFirstBlock(change);
      return;
    }
    this.extraClasses.update((list) =>
      list.map((c, i) => (i === index - 1 ? { ...c, ...change } : c)),
    );
  }

  private patchFirstBlock(change: Partial<ClassBlock>): void {
    const f = this.fullForm.controls;
    if (change.classKey !== undefined) f.className.setValue(change.classKey);
    if (change.level !== undefined) f.level.setValue(change.level);
    if (change.subclassKey !== undefined) f.subclassName.setValue(change.subclassKey);
    if (change.customSubclassName !== undefined)
      f.customSubclassName.setValue(change.customSubclassName);
  }

  /** The classes the other blocks already use: a class cannot be taken twice (the SRD's multiclass), so they are not offered here. */
  protected otherClassKeys(index: number): readonly string[] {
    return this.blocks()
      .filter((_, i) => i !== index)
      .map((b) => b.classKey)
      .filter((k) => k !== '');
  }

  /** "Adicionar classe": a new block at level 1 with nothing chosen. The server checks the multiclass
   * prerequisites of every class and shows what the sheet does not meet as issues on the sheet. */
  protected addClass(): void {
    this.extraClasses.update((list) => [
      ...list,
      { classKey: '', level: 1, subclassKey: '', customSubclassName: '' },
    ]);
  }

  protected removeClass(index: number): void {
    this.extraClasses.update((list) => list.filter((_, i) => i !== index - 1));
  }

  /** Set by a submit with an invalid field: from then on, the notice above
   * the buttons lists what is still wrong, and the steps that have it are
   * marked, until everything is fixed. */
  private readonly showErrors = signal(false);
  /** A save was tried: the blocks that lack a class say so. */
  protected readonly submitTried = this.showErrors.asReadonly();
  private readonly fullFormValue = toSignal(this.fullForm.valueChanges);
  private readonly basicFormValue = toSignal(this.basicForm.valueChanges);
  private readonly invalidFullFields = computed(() => {
    this.fullFormValue();
    return this.showErrors() ? this.currentInvalidFullFields() : [];
  });
  private readonly invalidBasicFields = computed(() => {
    this.basicFormValue();
    return this.showErrors() ? invalidBasicFields(this.basicForm) : [];
  });
  /** "Básico: Nome do personagem, Raça. Habilidades: Força." — or `''` when
   * nothing needs fixing (or no submit was tried yet). */
  protected readonly invalidSummary = computed(() => {
    const s = this.state();
    if (s.status !== 'ready') {
      return '';
    }
    return describeInvalidFields(
      isFullSheetKind(s.kind) ? this.invalidFullFields() : this.invalidBasicFields(),
    );
  });
  private readonly stepsWithErrors = computed(
    () => new Set(this.invalidFullFields().map((field) => field.step)),
  );

  /** "Bônus manuais" starts closed; a submit with an invalid bonus opens it. */
  protected readonly bonusesOpen = signal(false);
  private readonly bonusesValue = toSignal(
    this.fullForm.controls.extraAbilityBonuses.valueChanges,
    {
      initialValue: this.fullForm.controls.extraAbilityBonuses.getRawValue(),
    },
  );
  /** "Constituição +1, Inteligência +2", so the closed section still says
   * what it holds. */
  protected readonly bonusesInUse = computed(() => describeBonusesInUse(this.bonusesValue()));

  protected readonly backgroundSkillsCount = computed(() =>
    countLabel(
      this.customBackgroundSkills().size,
      'de 2 escolhida',
      'de 2 escolhidas',
      'Nenhuma escolhida',
    ),
  );

  /** The line under the title: what this form is for, and that the
   * sheet's numbers come from the server. */
  protected readonly lead = computed(() => leadOf(this.state()));
  protected readonly pageTitle = computed(() => pageTitleOf(this.state()));
  /** The label of the create button: "Salvar como reservado" for a character made for a player to claim. */
  protected readonly createLabel = computed(() => createLabelOf(this.state()));

  /** Whether the character being made or edited is reserved: the page tells the master so. */
  protected readonly reserved = computed(() => {
    const s = this.state();
    return s.status === 'ready' && s.reserved;
  });

  /** Cancel goes back where the person came from: the sheet being edited,
   * or the campaign. */
  protected readonly cancelLink = computed(() => cancelLinkOf(this.state()));

  /** What the buttons do, in one line next to them. */
  protected readonly actionsNote = computed(() => actionsNoteOf(this.state()));

  protected stepHasError(step: EditorStepKey): boolean {
    return this.stepsWithErrors().has(step);
  }

  /** What the page says after the table's content changed under the person ("" when nothing did). */
  protected readonly contentNote = signal('');
  private readonly campaignIdSignal = signal('');

  /** What the page reads from the server to open, and the routes it drops the answers of (`EditorLoader`). */
  private readonly loader = new EditorLoader({
    source: this.source,
    state: this.state,
    abilityTable: this.abilityTable,
    lockedOrigin: this.lockedOrigin,
    contentNote: this.contentNote,
    campaignId: this.campaignIdSignal,
    reservedRoute: () => this.isReservedRoute(),
    reset: () => this.resetEditing(),
    created: (ready, table) => this.createLoaded(ready, table),
    edited: (ready, existing, catalog) => this.editLoaded(ready, existing, catalog),
  });

  constructor() {
    // Only a player's editor follows the hint: the master writes the content himself, and an open stream would keep the
    // page from ever being quiet (every NPC editor of a live campaign would hold one).
    this.watcher.whileLive(
      () => (this.master() ? '' : this.campaignIdSignal()),
      () => void this.refreshCatalog(),
    );
    // The subclass of a one-class sheet waits for the level the class chooses it at (as in a block), unless one is chosen.
    effect(() => {
      const waits = subclassWaits(
        this.selectedClass(),
        this.selectedLevel(),
        this.selectedSubclassKey(),
      );
      const control = this.fullForm.controls.subclassName;
      if (waits && control.enabled) {
        control.disable({ emitEvent: false });
      } else if (!waits && control.disabled) {
        control.enable({ emitEvent: false });
      }
    });
    // While the "Pontos de vida" box or the "Magias" step is on screen, the server derives the draft (a race and a
    // class at least): the box adds up what the effects give, and the spell pickers count against the class's numbers.
    effect(() => {
      const s = this.state();
      const boxOn = serverDraftNeeded({
        ready: s.status === 'ready',
        rolledBox:
          this.selectedHitPointsMethod() === 'rolled' &&
          this.rollsNeeded() > 0 &&
          this.hitDie() > 0,
        caster: this.isCaster(),
        feats: this.master() && this.featKeys().length > 0,
      });
      if (!boxOn) {
        untracked(() => this.serverHitPoints.stop());
        return;
      }
      this.formChanges();
      const ready = this.selectedRaceKey() !== '' && this.selectedClassKey() !== '';
      // Reading the draft here makes every signal it is built from a reason to ask again.
      if (ready) {
        this.buildFullValue();
      }
      untracked(() => this.serverHitPoints.draftChanged(ready));
    });
    // The server reads the draft for its choices (a race and a class at least) and the "Escolhas" step shows them.
    effect(() => {
      const s = this.state();
      if (s.status !== 'ready' || !isFullSheetKind(s.kind)) {
        untracked(() => this.serverChoices.stop());
        return;
      }
      this.formChanges();
      const ready = this.selectedRaceKey() !== '' && this.selectedClassKey() !== '';
      if (ready) {
        this.buildFullValue();
      }
      untracked(() => this.serverChoices.draftChanged(ready));
    });
    // A pick the draft stores that no choice offers any more (the class or the race changed) goes, with its texts.
    effect(() => {
      const answer = this.choicesAnswer();
      if (!answer) {
        return;
      }
      untracked(() => {
        const next = choicesAfterAnswer(answer, this.featureChoiceKeys(), this.featureChoiceText());
        if (next.keys) {
          this.featureChoiceKeys.set(next.keys);
        }
        if (next.texts) {
          this.featureChoiceText.set(next.texts);
        }
      });
    });
    // A new player's sheet made above level 1 starts at its level's XP in a campaign that levels by XP, until the
    // person types in the field; a milestone campaign keeps the 0.
    effect(() => {
      const xp = startingXp(this.state(), this.totalLevelValue());
      const control = this.fullForm.controls.experiencePoints;
      if (xp !== null && !control.dirty) {
        untracked(() => control.setValue(xp));
      }
    });
    this.destroyRef.onDestroy(() => {
      this.loader.destroy();
      this.serverChoices.stop();
      this.serverHitPoints.stop();
    });
    this.route.paramMap.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((params) => {
      this.loader.resolve(params);
    });
  }

  /** A reused component starts every route from nothing: nothing of the previous character stays in the form or the pick sets. */
  private resetEditing(): void {
    this.fullForm.reset();
    this.basicForm.reset();
    this.selectedSkills.set(new Set());
    this.customBackgroundSkills.set(new Set());
    this.customBackgroundProficiencies.set([]);
    this.extraClasses.set([]);
    this.expertiseSkills.set(new Set());
    this.hitPointsRolls.set([]);
    this.picks.reset();
    this.featureChoiceKeys.set([]);
    this.featureChoiceText.set({});
    this.featKeys.set([]);
    this.featSlots.set({});
    this.serverChoices.stop();
    this.grantedSpells.set([]);
    this.hasCuttingWords.set(false);
    this.preparedMaxByClass.set({});
    this.abilityTable.set(null);
    this.abilityMethod.set('typed');
    this.lockedOrigin.set(null);
    this.saving.reset();
    this.contentNote.set('');
  }

  /**
   * Whether the route is `/campaigns/:id/reserved/new`: its `data` says so. The router hands a child route the data of its
   * parent (an empty path under a lazy parent), so the snapshot is where it is read; a route a test builds by hand may have none.
   */
  private isReservedRoute(): boolean {
    const snapshot = (this.route as Partial<ActivatedRoute>).snapshot;
    return snapshot?.data['reserved'] === true;
  }

  /** The lists of a new sheet arrived (`EditorLoader`). */
  private createLoaded(ready: ReadyState, table: AbilityTableVm | null): void {
    this.abilityTable.set(table);
    const method = tableHitPointsMethod(table);
    if (method) {
      this.fullForm.controls.hitPointsMethod.setValue(method);
    }
    this.state.set(ready);
    // A new enemy, boss or minion starts at ND 0 and 10 XP, so nobody is left without a
    // number; a story NPC gives none (and never shows the fields).
    if (ready.kind === 'enemy' || ready.kind === 'boss') {
      this.fullForm.patchValue({ challengeRating: '0', xpValue: 10 });
    } else if (ready.kind === 'story') {
      this.basicForm.patchValue({ challengeRating: '', xpValue: 0 });
    }
  }

  /** The lists and the sheet of an edit arrived (`EditorLoader`). */
  private editLoaded(ready: ReadyState, existing: CharacterForEdit, catalog: RulesCatalogVm): void {
    this.state.set(ready);
    if (existing.full) {
      this.openSavedSheet(existing, existing.full, catalog);
    }
    if (existing.basic) {
      patchBasicForm(this.fb, this.basicForm, existing.basic);
    }
  }

  private openSavedSheet(
    existing: CharacterForEdit,
    full: CharacterFormValue,
    catalog: RulesCatalogVm,
  ): void {
    this.patchFullForm(full);
    this.preparedMaxByClass.set(existing.preparedMax ?? {});
    this.hasCuttingWords.set((existing.featureKeys ?? []).includes('feature:cutting-words'));
    // What the sheet has from outside the form is told once, as the sheet has it: a spell of the subclass it
    // has now stays out of this list, and stays out when the subclass is changed (the save drops it with the subclass).
    const granted = new Set(existing.grantedSpellKeys ?? []);
    const fromSubclass = new Set(this.sections().flatMap((sec) => sec.alwaysPrepared));
    this.grantedSpells.set(
      catalog.spells.filter((sp) => granted.has(sp.key) && !fromSubclass.has(sp.key)),
    );
  }

  /** The table's rules for the scores changed under the person (the server refused the way of rolling the step offered):
   * they are read again, so the step offers what is allowed now. */
  protected rereadAbilityTable(): Promise<void> {
    return this.loader.rereadAbilityTable();
  }

  private patchFullForm(full: CharacterFormValue): void {
    this.fullForm.setValue(fullFormValueOf(full));
    this.selectedSkills.set(new Set(full.skillProficiencies));
    this.customBackgroundSkills.set(new Set(full.customBackgroundSkills ?? []));
    this.customBackgroundProficiencies.set(full.customBackgroundProficiencies);
    this.extraClasses.set(full.extraClasses);
    this.expertiseSkills.set(new Set(full.expertiseSkillKeys));
    this.hitPointsRolls.set(full.hitPointsRolls);
    this.picks.load(full.cantrips, full.spellsKnown, full.spellsPrepared);
    this.featureChoiceKeys.set(full.featureChoiceKeys);
    this.featureChoiceText.set(full.featureChoiceText);
    this.featKeys.set(full.featKeys ?? []);
    this.featSlots.set(full.featSlots ?? {});
  }

  /** The master gives the sheet a feat (MR-025). */
  protected addFeat(key: string): void {
    this.featKeys.update((keys) => (keys.includes(key) ? keys : [...keys, key]));
  }

  /** The master takes a feat off; the slot of an improvement it replaced goes with it (the server drops it too). */
  protected removeFeat(key: string): void {
    this.featKeys.update((keys) => keys.filter((k) => k !== key));
    this.featSlots.update((slots) =>
      Object.fromEntries(Object.entries(slots).filter(([k]) => k !== key)),
    );
  }

  /** A pick on the "Escolhas" step: the stored keys follow it, and the server reads the draft again. */
  protected pickChoice(pick: ChoiceSelection): void {
    this.featureChoiceKeys.update((keys) => applySelection(keys, pick.choice, pick.optionKeys));
  }

  protected editChoiceText(edit: ChoiceTextEdit): void {
    this.featureChoiceText.update((texts) =>
      withChoiceText(texts, edit.choice.key, edit.n, edit.text),
    );
  }

  /** "Escolher Rajada Mística agora": the spell becomes one of the sheet's cantrips (the "Magias" step), and the choices are read again. */
  protected addChoiceCantrip(spellKey: string): void {
    this.picks.addCantrip(spellKey);
  }

  /** The index of the stepper's step with that label; -1 when the page has no stepper or the step. */
  private stepIndexOf(label: string): number {
    return this.stepper?.steps.toArray().findIndex((step) => step.label === label) ?? -1;
  }

  /** The blocked "Criar personagem": the "Escolhas" step opens and the focus goes to the first choice still open. */
  private focusFirstPendingChoice(): void {
    const stepper = this.stepper;
    const index = this.stepIndexOf(EDITOR_STEP_LABELS.escolhas);
    if (!stepper || index < 0) {
      return;
    }
    if (index === stepper.selectedIndex) {
      this.choiceGroups?.focusFirstPending();
      return;
    }
    stepper.goTo(index);
    afterNextRender(() => this.choiceGroups?.focusFirstPending(), { injector: this.injector });
  }

  protected toggleSkill(key: string): void {
    const next = toggledSkill(
      this.selectedSkills(),
      this.expertiseSkills(),
      this.grantedSkills(),
      key,
    );
    if (!next) {
      return;
    }
    if (next.expertise) {
      this.expertiseSkills.set(next.expertise);
    }
    this.selectedSkills.set(next.skills);
  }

  /** Toggles expertise on a proficient skill (doubles its proficiency
   * bonus — Bard, Rogue). A no-op on a skill that is not proficient yet:
   * the checkbox is disabled for those in the template. */
  protected toggleExpertise(key: string): void {
    const next = toggledExpertise(
      this.selectedSkills(),
      this.grantedSkills(),
      this.expertiseSkills(),
      key,
    );
    if (next) {
      this.expertiseSkills.set(next);
    }
  }

  protected toggleCantrip(key: string): void {
    this.picks.toggleCantrip(key);
  }

  protected toggleSpellKnown(key: string): void {
    this.picks.toggleKnown(key);
  }

  protected toggleSpellPrepared(key: string): void {
    this.picks.togglePrepared(key);
  }

  /** Only the selected keys that are still in the (class-filtered) catalog
   * list — see `buildFullValue`'s comment. */
  private intersectWithAvailable(
    selected: ReadonlySet<string>,
    available: readonly { readonly key: string }[],
  ): string[] {
    return keepAvailable(selected, available, this.readyCatalog()?.spells ?? []);
  }

  /** The "?" next to a spell: its description, as a bottom sheet on a phone
   * and a dialog from a tablet up. Focus goes back to the "?" on close. */
  protected describeSpell(spell: SpellOptionVm): void {
    const s = this.state();
    if (s.status !== 'ready') {
      return;
    }
    const data: SpellDetailsData = {
      namePt: spell.namePt,
      load: () => this.loadSpellDetails(s.campaignId, spell.key),
    };
    openSpellDetails(this.dialog, this.bottomSheet, data);
  }

  private loadSpellDetails(campaignId: string, key: string): Promise<SpellDetailsVm> {
    let pending = this.spellDetails.get(key);
    if (!pending) {
      pending = this.source.loadSpellDetails(campaignId, key);
      this.spellDetails.set(key, pending);
      pending.catch(() => this.spellDetails.delete(key));
    }
    return pending;
  }

  /** Toggles one of the custom background's two granted skills. Caps at 2
   * (`CustomBackground.skill_keys`: "at most 2"): a third click while two
   * are already chosen does nothing, so the player unchecks one first. */
  protected toggleCustomBackgroundSkill(key: string): void {
    this.customBackgroundSkills.set(
      toggledCapped(this.customBackgroundSkills(), key, CUSTOM_BACKGROUND_SKILLS),
    );
  }

  /** Picks or drops one of the "Outro" background's two tools or languages; a third does nothing, like the skills. */
  protected setCustomProficiencies(keys: readonly string[]): void {
    this.customBackgroundProficiencies.set(keys.slice(0, CUSTOM_BACKGROUND_PROFICIENCIES));
  }

  protected readonly backgroundProficienciesCount = computed(() =>
    countLabel(
      this.customBackgroundProficiencies().length,
      'de 2 escolhida',
      'de 2 escolhidas',
      'Nenhuma escolhida',
    ),
  );

  private buildFullValue(): CharacterFormValue {
    return buildFullValue({
      form: this.fullForm.getRawValue(),
      customBackgroundSkills: this.customBackgroundSkills(),
      customBackgroundProficiencies: this.customBackgroundProficiencies(),
      extraClasses: this.extraClasses(),
      chosenSkills: this.chosenSkills(),
      expertiseSkills: this.expertiseSkills(),
      hitPointsRolls: this.hitPointsRolls(),
      rollsNeeded: this.rollsNeeded(),
      isCaster: this.isCaster(),
      cantrips: this.intersectWithAvailable(this.selectedCantrips(), this.availableCantrips()),
      spellsKnown: this.intersectWithAvailable(this.selectedSpellsKnown(), this.availableSpells()),
      spellsPrepared: this.intersectWithAvailable(
        this.selectedSpellsPrepared(),
        this.availableSpells(),
      ),
      featureChoiceKeys: this.featureChoiceKeys(),
      featureChoiceText: this.featureChoiceText(),
      feats: this.master() ? { keys: this.featKeys(), slots: this.featSlots() } : null,
    });
  }

  /** What the class blocks still lack, in words ("Classe 2", "o nível total vai até 20"): not controls of the
   * form, so the page adds them to the invalid fields while they stand. */
  private readonly classProblems = computed<string[]>(() =>
    classProblemsOf(this.extraClasses(), this.totalLevelProblem()),
  );

  /** With "Rolado", the levels whose roll is empty or does not fit that level's die: the form keeps no control for
   * them, so the page adds them to the invalid fields while they stand. */
  private readonly rollProblems = computed<string[]>(() =>
    rollProblemsOf(
      this.selectedHitPointsMethod() === 'rolled',
      this.hitDie(),
      this.rollsNeeded(),
      this.levelDice(),
      this.hitPointsRolls(),
    ),
  );

  /** The full sheet's invalid fields right now, plus the placing that is not finished (it has no control of its
   * own: see `abilitiesIncomplete`). */
  private currentInvalidFullFields(): EditorField[] {
    return invalidFullFieldsOf(this.fullForm, this.classProblems(), this.rollProblems(), {
      incomplete: this.abilitiesIncomplete(),
      problem: this.abilitiesProblem(),
    });
  }

  /** After a submit with an invalid field: opens the step of the first one
   * (and "Bônus manuais", if that is where it is), so the field and its
   * error message are on screen. */
  private openFirstInvalidStep(): void {
    const [first] = this.currentInvalidFullFields();
    if (!first?.step) {
      return;
    }
    if (first.path.startsWith('extraAbilityBonuses.')) {
      this.bonusesOpen.set(true);
    }
    const stepper = this.stepper;
    const index = this.stepIndexOf(EDITOR_STEP_LABELS[first.step]);
    if (stepper && index >= 0 && index !== stepper.selectedIndex) {
      stepper.goTo(index);
    }
  }

  protected async submit(): Promise<void> {
    const s = this.state();
    if (s.status !== 'ready') {
      return;
    }
    const isBasic = !isFullSheetKind(s.kind);
    // A choice still open: the button only leads to it (it is aria-disabled, not disabled, so it can be reached and activated).
    if (!isBasic && this.choicesBlocked()) {
      this.focusFirstPendingChoice();
      return;
    }
    if (this.hasProblems(isBasic)) {
      this.showProblems(isBasic);
      return;
    }
    await this.saving.run(s, isBasic);
  }

  /** The form of the kind being saved is invalid, or what the full sheet keeps outside its controls is unfinished. */
  private hasProblems(isBasic: boolean): boolean {
    if (isBasic) {
      return this.basicForm.invalid;
    }
    return (
      this.fullForm.invalid ||
      this.abilitiesIncomplete() ||
      this.classProblems().length > 0 ||
      this.rollProblems().length > 0
    );
  }

  /** A save with something to fix: every field says so, the notice lists them and the first step opens. */
  private showProblems(isBasic: boolean): void {
    (isBasic ? this.basicForm : this.fullForm).markAllAsTouched();
    this.saving.state.set({ status: 'idle' });
    this.showErrors.set(true);
    if (!isBasic) {
      this.openFirstInvalidStep();
    }
  }

  /**
   * The server refused a key the master switched off: its field gets the error (`switchedOff`, so it is invalid, red and
   * `aria-invalid`), the step is marked "(com erro)", the editor opens it and the focus goes to the field. Returns false when
   * no field holds the key (a spell, say): the notice above the buttons says it by name instead.
   */
  private markSwitchedOff(key: string): boolean {
    const control = offControlOf(key, this.fullForm.getRawValue());
    if (!control) {
      return false;
    }
    const ref = contentRef(key, (k) => this.nameOfKey(k));
    this.fullForm.controls[control].setErrors({
      switchedOff: characterBlockedMessage('switched_off_content', { ...ref, step: '' }),
    });
    this.fullForm.controls[control].markAsTouched();
    this.showErrors.set(true);
    this.saving.state.set({ status: 'idle' });
    this.openFirstInvalidStep();
    afterNextRender(
      () =>
        this.host.nativeElement
          .querySelector<HTMLElement>(`[formcontrolname="${control}"]`)
          ?.focus(),
      { injector: this.injector },
    );
    return true;
  }

  /** The sentence of a switched-off option on its field, for the template. */
  protected switchedOff(
    control: 'className' | 'race' | 'subrace' | 'subclassName' | 'background',
  ): string {
    return (this.fullForm.controls[control].getError('switchedOff') as string | null) ?? '';
  }

  /** `content_changed` (RN-23): the lists are read again with this person's role; what was typed stays. Resolves when done. */
  private refreshCatalog(): Promise<void> {
    return this.loader.refreshCatalog();
  }
}
