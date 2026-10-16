import {
  Component,
  DestroyRef,
  ElementRef,
  Injector,
  ViewChild,
  computed,
  effect,
  inject,
  signal,
  untracked,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
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

import { ContentWatcher } from '../../core/content/content-watcher';
import { LiveSessionSourceLive } from '../../core/play/live-session-source.live';
import { isFullSheetKind } from '../../core/characters/characters.types';
import { ChoiceGroups } from '../../shared/choice-groups/choice-groups';
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
  SpellOptionVm,
} from './character-editor.types';
import { ClassBlockFields } from './class-block/class-block';
import { GrantedSpells } from './granted-spells/granted-spells';
import type { CasterSection, ClassBlock } from './class-blocks';
import { subclassWaits } from './catalog-views';
import { CUTTING_WORDS_OPTIONS } from './editor-options';
import { EditorLoader } from './editor-loader';
import { type PageState, startingXp } from './editor-state';
import { EditorStepper } from './editor-stepper/editor-stepper';
import { HitPointsRolls } from './hit-points-rolls/hit-points-rolls';
import { EDITOR_STEP_LABELS, EditorStepKey } from './editor-labels';
import { createFullForm, patchFirstClass } from './full-form';
import { EditorCatalog } from './editor-catalog';
import { EditorDraft } from './editor-draft';
import { EditorPicks } from './editor-picks';
import { basicFormToValue, createBasicForm } from './npc-short-form/basic-form';
import { SaveFlow } from './save-flow';
import { filterByName } from './spell-views';
import { SpellPicks } from './spell-picks';
import { DefeatXp } from './defeat-xp/defeat-xp';
import { FeatsField } from './feats-field/feats-field';
import { PortraitField } from './portrait-field/portrait-field';
import { NpcShortForm } from './npc-short-form/npc-short-form';
import { SkillPicker } from './skill-picker/skill-picker';
import { SpellPicker } from './spell-picker/spell-picker';

import { EditorOpening } from './editor-opening';
import { EditorServerDraft } from './editor-server-draft';
import { EditorSubmit } from './editor-submit';
import type { EditorCore } from './editor-core';
import { EditorSpellDetails } from './editor-spell-details';
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
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);
  // The session's stream says when the table's content changes (10.1d, RN-23): one stream, one read per hint, with the debounce.
  private readonly watcher = inject(ContentWatcher);

  @ViewChild(EditorStepper) private stepper?: EditorStepper;
  @ViewChild(ChoiceGroups) private choiceGroups?: ChoiceGroups;

  protected readonly state = signal<PageState>({ status: 'loading', title: 'Ficha' });
  protected readonly fullForm = createFullForm(this.fb);
  protected readonly basicForm = createBasicForm(this.fb);

  /** The table's ways of making scores, when a player makes a new sheet (RN-24); `null` is the free editor of the master's NPCs and of an edit. */
  protected readonly abilityTable = signal<AbilityTableVm | null>(null);
  /** The way the player chose; the server checks the scores against it. */
  protected readonly abilityMethod = signal<AbilityMethodKey>('typed');
  /** A player editing their own draft: the way the server recorded the scores were made, which the step keeps (RN-24). */
  protected readonly lockedOrigin = signal<NonNullable<CharacterForEdit['abilityOrigin']> | null>(
    null,
  );
  /** "Bônus manuais" starts closed; a submit with an invalid bonus opens it. */
  protected readonly bonusesOpen = signal(false);
  /** What the page says after the table's content changed under the person ("" when nothing did). */
  protected readonly contentNote = signal('');
  private readonly campaignIdSignal = signal('');

  /** What the person picked beside the form's controls (`EditorPicks`) and the spells of the "Magias" step. */
  private readonly picks: EditorPicks = new EditorPicks(() => this.views.grantedSkills());
  private readonly spellPicks: SpellPicks = new SpellPicks({
    sections: () => this.views.sections(),
  });
  /** What every plain class below reads: the state, the two forms and the picks. */
  private readonly core: EditorCore = {
    state: this.state,
    fullForm: this.fullForm,
    basicForm: this.basicForm,
    picks: this.picks,
    spellPicks: this.spellPicks,
  };
  /** What the catalog and the form say together: the lists, the blocks, the spell pickers (`EditorCatalog`). */
  protected readonly views: EditorCatalog = new EditorCatalog({
    ...this.core,
    derived: () => this.server.hitPoints.answer(),
    choices: () => this.server.choices.answer(),
  });
  /** The draft and what is wrong with it (`EditorDraft`). */
  private readonly draft = new EditorDraft({ ...this.core, catalog: this.views });
  /** What the server says about the draft: its hit points and spell numbers, and its choices (`EditorServerDraft`). */
  private readonly server = new EditorServerDraft({
    ...this.core,
    source: this.source,
    views: this.views,
    abilityTable: this.abilityTable,
    build: () => this.draft.build(),
  });
  /** The save: the request, the navigation after it and what a refusal does (`SaveFlow`). */
  private readonly saving: SaveFlow = new SaveFlow({
    source: this.source,
    navigate: (commands) => this.router.navigate(commands),
    full: () => this.draft.build(),
    basic: () => basicFormToValue(this.basicForm),
    name: (isBasic) =>
      isBasic ? this.basicForm.getRawValue().name : this.fullForm.getRawValue().name,
    abilityMethod: () => (this.abilityTable() ? this.abilityMethod() : null),
    stillHere: () => this.loader.stillHere(),
    nameOfKey: (key) => this.views.nameOfKey(key),
    markSwitchedOff: (key) => this.submitting.markSwitchedOff(key),
    refreshCatalog: () => this.refreshCatalog(),
  });
  /** The class block a refusal points at (`full.classes[i]`), so the block says so. */
  protected readonly serverClassProblem = this.saving.classProblem;
  protected readonly saveState = this.saving.state;
  /** The save button and what it does before the request (`EditorSubmit`). */
  private readonly submitting: EditorSubmit = new EditorSubmit({
    ...this.core,
    draft: this.draft,
    views: this.views,
    saving: this.saving,
    choicesBlocked: this.server.choicesBlocked,
    bonusesOpen: this.bonusesOpen,
    stepper: () => this.stepper,
    choiceGroups: () => this.choiceGroups,
    element: this.host.nativeElement,
    injector: this.injector,
  });
  /** What the page does with the lists once they are read (`EditorOpening`). */
  private readonly opening = new EditorOpening({
    ...this.core,
    fb: this.fb,
    views: this.views,
    abilityTable: this.abilityTable,
    abilityMethod: this.abilityMethod,
    lockedOrigin: this.lockedOrigin,
    contentNote: this.contentNote,
    forget: () => {
      this.server.choices.stop();
      this.saving.reset();
    },
  });
  /** The "?" next to a spell (`EditorSpellDetails`). */
  private readonly spellDetails = new EditorSpellDetails({
    source: this.source,
    state: this.state,
    dialog: inject(MatDialog),
    bottomSheet: inject(MatBottomSheet),
  });
  protected readonly describeSpell = (spell: SpellOptionVm) => this.spellDetails.describe(spell);
  protected readonly submit = () => this.submitting.submit();
  protected readonly switchedOff = (control: Parameters<EditorSubmit['switchedOff']>[0]) =>
    this.submitting.switchedOff(control);

  protected readonly customBackgroundSkills = this.picks.customBackgroundSkills;
  protected readonly customBackgroundProficiencies = this.picks.customBackgroundProficiencies;
  protected readonly expertiseSkills = this.picks.expertiseSkills;
  protected readonly hitPointsRolls = this.picks.hitPointsRolls;
  protected readonly featKeys = this.picks.featKeys;
  protected readonly featSlots = this.picks.featSlots;
  protected readonly otherGranted = this.picks.grantedSpells.asReadonly();
  protected readonly hasCuttingWords = this.picks.hasCuttingWords;
  protected readonly toggleSkill = this.picks.toggleSkill;
  protected readonly toggleExpertise = this.picks.toggleExpertise;
  protected readonly toggleCustomBackgroundSkill = this.picks.toggleCustomBackgroundSkill;
  protected readonly setCustomProficiencies = this.picks.setCustomProficiencies;
  protected readonly addClass = this.picks.addClass;
  protected readonly removeClass = this.picks.removeClass;
  protected readonly addFeat = this.picks.addFeat;
  protected readonly removeFeat = this.picks.removeFeat;
  protected readonly pickChoice = this.picks.pickChoice;
  protected readonly editChoiceText = this.picks.editChoiceText;
  protected readonly selectedCantrips = this.spellPicks.cantrips;
  protected readonly selectedSpellsKnown = this.spellPicks.known;
  protected readonly selectedSpellsPrepared = this.spellPicks.prepared;

  protected readonly availableSubraces = this.views.availableSubraces;
  protected readonly backgroundEquipment = this.views.backgroundEquipment;
  protected readonly backgroundOptions = this.views.backgroundOptions;
  protected readonly backgroundUnlisted = this.views.backgroundUnlisted;
  protected readonly blocks = this.views.blocks;
  protected readonly choiceBonusesHint = this.views.choiceBonusesHint;
  protected readonly choicesSubtitle = this.views.choicesSubtitle;
  protected readonly chosenSkills = this.views.chosenSkills;
  protected readonly classOptions = this.views.classOptions;
  protected readonly classUnlisted = this.views.classUnlisted;
  protected readonly finalConstitution = this.views.finalConstitution;
  protected readonly grantedSkills = this.views.grantedSkills;
  protected readonly hitDie = this.views.hitDie;
  protected readonly isCaster = this.views.isCaster;
  protected readonly languageOptions = this.views.languageOptions;
  protected readonly levelDice = this.views.levelDice;
  protected readonly magiasLink = this.views.magiasLink;
  protected readonly master = this.views.master;
  protected readonly multiclass = this.views.multiclass;
  protected readonly raceOptions = this.views.raceOptions;
  protected readonly raceUnlisted = this.views.raceUnlisted;
  protected readonly rollsNeeded = this.views.rollsNeeded;
  protected readonly sectionViews = this.views.sectionViews;
  protected readonly sections = this.views.sections;
  protected readonly selectedBackground = this.views.selectedBackground;
  protected readonly selectedBackgroundVm = this.views.selectedBackgroundVm;
  protected readonly selectedClass = this.views.selectedClass;
  protected readonly selectedClassKey = this.views.selectedClassKey;
  protected readonly selectedHitPointsMethod = this.views.selectedHitPointsMethod;
  protected readonly selectedRace = this.views.selectedRace;
  protected readonly selectedRaceKey = this.views.selectedRaceKey;
  protected readonly selectedSubclass = this.views.selectedSubclass;
  protected readonly selectedSubrace = this.views.selectedSubrace;
  protected readonly skillNote = this.views.skillNote;
  protected readonly subclassHint = this.views.subclassHint;
  protected readonly subclassOptions = this.views.subclassOptions;
  protected readonly subraceOptions = this.views.subraceOptions;
  protected readonly toolOptions = this.views.toolOptions;
  protected readonly totalLevelProblem = this.views.totalLevelProblem;
  protected readonly totalLevelValue = this.views.totalLevelValue;
  protected readonly sectionHeading = (section: CasterSection) =>
    this.views.sectionHeading(section);
  protected readonly otherClassKeys = (index: number) => this.views.otherClassKeys(index);

  protected readonly abilitiesIncomplete = this.draft.abilitiesIncomplete;
  protected readonly abilitiesProblem = this.draft.abilitiesProblem;
  protected readonly bonusesInUse = this.draft.bonusesInUse;
  protected readonly invalidSummary = this.draft.invalidSummary;
  /** A save was tried: the blocks that lack a class say so. */
  protected readonly submitTried = this.draft.showErrors.asReadonly();
  protected readonly stepHasError = (step: EditorStepKey) => this.draft.stepHasError(step);

  protected readonly featIssues = this.server.featIssues;
  protected readonly hitPointsFromEffects = this.server.hitPointsFromEffects;
  protected readonly choicesAnswer = this.server.choicesAnswer;
  protected readonly choiceGroupsOfDraft = this.server.choiceGroups;
  protected readonly choicesStep = this.server.choicesStep;
  protected readonly pendingSteps = this.server.pendingSteps;
  protected readonly eagerSteps = this.server.eagerSteps;
  protected readonly choicesBlocked = this.server.choicesBlocked;
  protected readonly missingText = this.server.missingText;

  protected readonly cuttingWordsOptions = CUTTING_WORDS_OPTIONS;
  protected readonly isFullSheetKind = isFullSheetKind;
  protected readonly stepLabels = EDITOR_STEP_LABELS;
  protected readonly alignmentKeys = Object.keys(ALIGNMENT_LABELS) as AlignmentKey[];
  protected readonly alignmentLabel = (key: AlignmentKey) => ALIGNMENT_LABELS[key];

  /** What the table leaves of the hit points of a new sheet: both ways, only "Rolado" or only "Média". */
  protected readonly hpRule = computed<'player_chooses' | 'roll' | 'average'>(() => {
    const s = this.state();
    return s.status === 'ready' && s.mode === 'create'
      ? (this.abilityTable()?.hitPoints ?? 'player_chooses')
      : 'player_chooses';
  });

  protected readonly experienceLabel = this.views.experienceLabel;
  protected readonly backgroundSkillsCount = this.views.backgroundSkillsCount;
  protected readonly backgroundProficienciesCount = this.views.backgroundProficienciesCount;
  protected readonly lead = this.views.lead;
  protected readonly pageTitle = this.views.pageTitle;
  protected readonly createLabel = this.views.createLabel;
  protected readonly reserved = this.views.reserved;
  protected readonly cancelLink = this.views.cancelLink;
  protected readonly actionsNote = this.views.actionsNote;

  /** What the page reads from the server to open, and the routes it drops the answers of (`EditorLoader`). */
  private readonly loader = new EditorLoader({
    source: this.source,
    state: this.state,
    abilityTable: this.abilityTable,
    lockedOrigin: this.lockedOrigin,
    contentNote: this.contentNote,
    campaignId: this.campaignIdSignal,
    reservedRoute: () => this.isReservedRoute(),
    reset: () => this.opening.reset(),
    created: (ready, table) => this.opening.created(ready, table),
    edited: (ready, existing, catalog) => this.opening.edited(ready, existing, catalog),
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
        this.views.selectedLevel(),
        this.views.selectedSubclassKey(),
      );
      const control = this.fullForm.controls.subclassName;
      if (waits && control.enabled) {
        control.disable({ emitEvent: false });
      } else if (!waits && control.disabled) {
        control.enable({ emitEvent: false });
      }
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
      this.server.stop();
    });
    this.route.paramMap.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((params) => {
      this.loader.resolve(params);
    });
  }

  /**
   * Whether the route is `/campaigns/:id/reserved/new`: its `data` says so. The router hands a child route the data of its
   * parent (an empty path under a lazy parent), so the snapshot is where it is read; a route a test builds by hand may have none.
   */
  private isReservedRoute(): boolean {
    const snapshot = (this.route as Partial<ActivatedRoute>).snapshot;
    return snapshot?.data['reserved'] === true;
  }

  /** The table's rules for the scores changed under the person (the server refused the way of rolling the step offered):
   * they are read again, so the step offers what is allowed now. */
  protected rereadAbilityTable(): Promise<void> {
    return this.loader.rereadAbilityTable();
  }

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
      patchFirstClass(this.fullForm, change);
      return;
    }
    this.picks.changeExtraClass(index - 1, change);
  }

  /** The search of an extra list ("0:patron"), and what it narrows. */
  protected readonly spellFilterOf = (id: string) => this.spellPicks.filterOf(id);
  protected readonly setExtraFilter = (id: string, value: string) =>
    this.spellPicks.setFilter(id, value);
  protected readonly filterSpells = (spells: readonly SpellOptionVm[], id: string) =>
    filterByName(spells, this.spellPicks.filterOf(id));
  protected readonly setSpellFilter = (
    section: number,
    list: 'cantrips' | 'known' | 'prepared',
    value: string,
  ) => this.spellPicks.setFilter(`${section}:${list}`, value);
  /** "Escolher Rajada Mística agora": the spell becomes one of the sheet's cantrips (the "Magias" step), and the choices are read again. */
  protected readonly addChoiceCantrip = (key: string) => this.spellPicks.addCantrip(key);
  protected readonly toggleCantrip = (key: string) => this.spellPicks.toggleCantrip(key);
  protected readonly toggleSpellKnown = (key: string) => this.spellPicks.toggleKnown(key);
  protected readonly toggleSpellPrepared = (key: string) => this.spellPicks.togglePrepared(key);

  /** `content_changed` (RN-23): the lists are read again with this person's role; what was typed stays. Resolves when done. */
  private refreshCatalog(): Promise<void> {
    return this.loader.refreshCatalog();
  }
}
