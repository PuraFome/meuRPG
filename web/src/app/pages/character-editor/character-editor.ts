import { Component, DestroyRef, ViewChild, computed, inject, signal } from '@angular/core';
import { takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { CdkStep } from '@angular/cdk/stepper';
import { MatButtonModule } from '@angular/material/button';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatRadioModule } from '@angular/material/radio';
import { MatSelectModule } from '@angular/material/select';
import { ActivatedRoute, ParamMap, Router, RouterLink } from '@angular/router';

import { describeCharacterError } from '../../core/characters/character-errors';
import { characterKindLabel } from '../../core/characters/character-labels';
import { CharacterKind, isFullSheetKind } from '../../core/characters/characters.types';
import { FictionNotice } from '../../shared/fiction-notice/fiction-notice';
import { AbilityFields } from './ability-fields/ability-fields';
import {
  ALIGNMENT_LABELS,
  AlignmentKey,
  CharacterEditorMode,
  CharacterEditorSource,
  CharacterFormValue,
  HitPointsMethod,
  RulesCatalogVm,
} from './character-editor.types';
import { EditorStepper } from './editor-stepper/editor-stepper';
import {
  BASIC_SHEET_FIELDS,
  EDITOR_STEP_LABELS,
  EditorStepKey,
  FULL_SHEET_FIELDS,
  countLabel,
  describeBonusesInUse,
  describeInvalidFields,
  invalidFields,
} from './editor-labels';
import { NpcShortForm } from './npc-short-form/npc-short-form';
import { SkillPicker } from './skill-picker/skill-picker';
import { SpellPicker } from './spell-picker/spell-picker';

/** The NPC route's `:tipo` segment (plan §5) to `CharacterKind`. */
const TIPO_TO_KIND: Record<string, CharacterKind> = {
  inimigo: 'enemy',
  boss: 'boss',
  minion: 'minion',
  historia: 'story',
};

type ReadyState = {
  status: 'ready';
  mode: CharacterEditorMode;
  kind: CharacterKind;
  campaignId: string;
  characterId: string | null;
  revision: number;
  catalog: RulesCatalogVm;
};

type ErrorState = {
  status: 'error';
  message: string;
  /** Where "Voltar" goes: the sheet when editing one, else the campaign. */
  backLink: string[];
  backLabel: string;
};

type PageState = { status: 'loading'; title: string } | ErrorState | ReadyState;

/** The page title: known from the route alone, so the loading state shows
 * it too. */
function titleFor(mode: CharacterEditorMode, kind: CharacterKind): string {
  if (mode === 'edit') {
    return 'Editar ficha';
  }
  return kind === 'player' ? 'Criar personagem' : 'Criar NPC';
}

type SavingState = { status: 'idle' } | { status: 'saving' } | { status: 'error'; message: string };

/** The "search box" the catalog-backed pickers use to narrow a long list
 * (cantrips, spells) — a plain case-insensitive substring match on the
 * Portuguese name, no new dependency. */
function filterByName<T extends { readonly namePt: string }>(
  items: readonly T[],
  query: string,
): readonly T[] {
  const q = query.trim().toLowerCase();
  return q ? items.filter((item) => item.namePt.toLowerCase().includes(q)) : items;
}

/**
 * The character editor (MR-003, MR-005, MR-006): a stepper for a full
 * sheet (player, enemy, boss) — Básico, Atributos, Perícias, Magias (only
 * for a caster class), Equipamento, `EditorStepper` over the CDK stepper —
 * or a single short form for a basic sheet (minion, story, `NpcShortForm`),
 * routed from three places (plan §5):
 *
 * - `/campanhas/:id/personagens/novo` — a player creates their character.
 * - `/campanhas/:id/npcs/novo/:tipo` — a master creates an NPC.
 * - `/campanhas/:id/personagens/:characterId/editar` — either edits.
 *
 * The submit action sits outside the stepper, right under the open step,
 * so the whole form can be saved from any step. A submit with an invalid
 * field lists what to fix (`invalidSummary`), marks the steps that have
 * one, and opens the first of them. RN-01 is enforced on the server: this
 * page renders whatever `describeCharacterError` maps a
 * `failed_precondition` / `SHEET_LOCKED` response to, exactly like
 * `character-sheet` does. No D&D rule runs here: the page never shows a
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
    AbilityFields,
    CdkStep,
    EditorStepper,
    FictionNotice,
    MatButtonModule,
    MatCheckboxModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatProgressSpinnerModule,
    MatRadioModule,
    MatSelectModule,
    NpcShortForm,
    ReactiveFormsModule,
    RouterLink,
    SkillPicker,
    SpellPicker,
  ],
  templateUrl: './character-editor.html',
  styleUrl: './character-editor.scss',
})
export class CharacterEditor {
  private readonly source = inject(CharacterEditorSource);
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly destroyRef = inject(DestroyRef);
  private readonly fb = inject(FormBuilder);

  @ViewChild(EditorStepper) private stepper?: EditorStepper;

  protected readonly state = signal<PageState>({ status: 'loading', title: 'Ficha' });
  protected readonly saveState = signal<SavingState>({ status: 'idle' });
  protected readonly selectedSkills = signal<ReadonlySet<string>>(new Set());
  /** The custom background's two granted skills (`CustomBackground.skills`,
   * integrator fix, phase 2) — a separate, capped-at-2 selection from
   * `selectedSkills`, only shown when `background === 'custom'`. */
  protected readonly customBackgroundSkills = signal<ReadonlySet<string>>(new Set());
  /** Expertise doubles a skill's proficiency bonus — always a subset of
   * `selectedSkills` (integrator fix, phase 2b). */
  protected readonly expertiseSkills = signal<ReadonlySet<string>>(new Set());
  /** One roll per level after the first, index 0 = level 2. Only sent when
   * `hitPointsMethod` is "rolled" (integrator fix, phase 2b). */
  protected readonly hitPointsRolls = signal<readonly number[]>([]);

  /** Catalog-backed pickers (integrator fix: the editor must never make a
   * person type a content key) — cantrips and the known/prepared spell
   * lists, each a filterable checkbox list over `RulesCatalogVm.spells`. */
  protected readonly selectedCantrips = signal<ReadonlySet<string>>(new Set());
  protected readonly selectedSpellsKnown = signal<ReadonlySet<string>>(new Set());
  protected readonly selectedSpellsPrepared = signal<ReadonlySet<string>>(new Set());
  protected readonly cantripsFilter = signal('');
  protected readonly spellsKnownFilter = signal('');
  protected readonly spellsPreparedFilter = signal('');

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

  protected readonly fullForm = this.fb.nonNullable.group({
    name: ['', [Validators.required, Validators.maxLength(80)]],
    race: ['', Validators.required],
    subrace: [''],
    className: ['', Validators.required],
    subclassName: [''],
    customSubclassName: [''],
    level: [1, [Validators.required, Validators.min(1), Validators.max(20)]],
    background: ['', Validators.required],
    customBackgroundName: ['', Validators.maxLength(40)],
    /** A content key from the catalog, or `''` for "Sem armadura" — never
     * typed (integrator fix). */
    armor: [''],
    shield: [false],
    /** Content keys, `<mat-select multiple>` — never typed. */
    weaponKeys: [[] as string[]],
    equipmentText: ['', Validators.maxLength(4000)],
    languagesText: ['', Validators.maxLength(2000)],
    toolProficienciesText: ['', Validators.maxLength(2000)],
    experiencePoints: [0, [Validators.required, Validators.min(0), Validators.max(1000000)]],
    alignment: ['' as AlignmentKey],
    customFeaturesText: ['', Validators.maxLength(5000)],
    hitPointsMethod: ['average' as HitPointsMethod],
    abilities: this.fb.nonNullable.group({
      str: [10, [Validators.required, Validators.min(1), Validators.max(30)]],
      dex: [10, [Validators.required, Validators.min(1), Validators.max(30)]],
      con: [10, [Validators.required, Validators.min(1), Validators.max(30)]],
      int: [10, [Validators.required, Validators.min(1), Validators.max(30)]],
      wis: [10, [Validators.required, Validators.min(1), Validators.max(30)]],
      cha: [10, [Validators.required, Validators.min(1), Validators.max(30)]],
    }),
    extraAbilityBonuses: this.fb.nonNullable.group({
      str: [0, [Validators.required, Validators.min(-10), Validators.max(10)]],
      dex: [0, [Validators.required, Validators.min(-10), Validators.max(10)]],
      con: [0, [Validators.required, Validators.min(-10), Validators.max(10)]],
      int: [0, [Validators.required, Validators.min(-10), Validators.max(10)]],
      wis: [0, [Validators.required, Validators.min(-10), Validators.max(10)]],
      cha: [0, [Validators.required, Validators.min(-10), Validators.max(10)]],
    }),
  });

  protected readonly basicForm = this.fb.nonNullable.group({
    name: ['', [Validators.required, Validators.maxLength(80)]],
    hitPointsMax: [1, [Validators.required, Validators.min(1)]],
    armorClass: [10, [Validators.required, Validators.min(1)]],
    speedWalkFt: [30, [Validators.required, Validators.min(0)]],
    attackBonus: [0, Validators.required],
    damage: ['', [Validators.required, Validators.maxLength(80)]],
    description: ['', Validators.maxLength(2000)],
  });

  private readonly selectedRaceKey = toSignal(this.fullForm.controls.race.valueChanges, {
    initialValue: '',
  });
  private readonly selectedClassKey = toSignal(this.fullForm.controls.className.valueChanges, {
    initialValue: '',
  });
  protected readonly selectedBackground = toSignal(this.fullForm.controls.background.valueChanges, {
    initialValue: '',
  });
  private readonly selectedLevel = toSignal(this.fullForm.controls.level.valueChanges, {
    initialValue: 1,
  });
  protected readonly selectedHitPointsMethod = toSignal(
    this.fullForm.controls.hitPointsMethod.valueChanges,
    { initialValue: 'average' as HitPointsMethod },
  );
  /** How many rolls "Dados de Vida" needs: one per level after the first. */
  protected readonly rollsNeeded = computed(() => Math.max(0, this.selectedLevel() - 1));
  /** `[0, 1, ..., rollsNeeded() - 1]`, for the template's `@for` over the
   * roll inputs — a real array, unlike `Array(n)`, which `@for` cannot
   * iterate (a sparse array has no elements to track). */
  protected readonly rollIndices = computed(() =>
    Array.from({ length: this.rollsNeeded() }, (_, i) => i),
  );

  protected readonly availableSubraces = computed(() => {
    const s = this.state();
    if (s.status !== 'ready') {
      return [];
    }
    return s.catalog.races.find((r) => r.key === this.selectedRaceKey())?.subraces ?? [];
  });

  protected readonly selectedClass = computed(() => {
    const s = this.state();
    if (s.status !== 'ready') {
      return undefined;
    }
    return s.catalog.classes.find((c) => c.key === this.selectedClassKey());
  });

  protected readonly isCaster = computed(() => this.selectedClass()?.isCaster ?? false);
  /** Picks which spell-list fields the Magias step shows (integrator
   * amendment, 29/09/2026): "known" and "spellbook" classes show "Magias
   * conhecidas"; "prepared" and "spellbook" classes show "Magias
   * preparadas". Truques (cantrips) show for every caster regardless. */
  protected readonly spellPreparation = computed(() => this.selectedClass()?.preparation ?? null);

  /** `RulesCatalogVm.spells` filtered to the chosen class's list — cantrips
   * (level 0) and leveled spells (1-9) are two different pools, never
   * character-level-gated (the browser never computes that rule; an
   * unavailable choice shows up as a `DerivedSheet.issue` instead). */
  protected readonly availableCantrips = computed(() => {
    const s = this.state();
    if (s.status !== 'ready') {
      return [];
    }
    const classKey = this.selectedClassKey();
    return s.catalog.spells.filter((sp) => sp.level === 0 && sp.classKeys.includes(classKey));
  });
  protected readonly availableSpells = computed(() => {
    const s = this.state();
    if (s.status !== 'ready') {
      return [];
    }
    const classKey = this.selectedClassKey();
    return s.catalog.spells.filter((sp) => sp.level >= 1 && sp.classKeys.includes(classKey));
  });
  protected readonly filteredCantrips = computed(() =>
    filterByName(this.availableCantrips(), this.cantripsFilter()),
  );
  protected readonly filteredSpellsKnown = computed(() =>
    filterByName(this.availableSpells(), this.spellsKnownFilter()),
  );
  protected readonly filteredSpellsPrepared = computed(() =>
    filterByName(this.availableSpells(), this.spellsPreparedFilter()),
  );

  /** Set by a submit with an invalid field: from then on, the notice above
   * the buttons lists what is still wrong, and the steps that have it are
   * marked, until everything is fixed. */
  private readonly showErrors = signal(false);
  private readonly fullFormValue = toSignal(this.fullForm.valueChanges);
  private readonly basicFormValue = toSignal(this.basicForm.valueChanges);
  private readonly invalidFullFields = computed(() => {
    this.fullFormValue();
    return this.showErrors() ? invalidFields(this.fullForm, FULL_SHEET_FIELDS) : [];
  });
  private readonly invalidBasicFields = computed(() => {
    this.basicFormValue();
    return this.showErrors() ? invalidFields(this.basicForm, BASIC_SHEET_FIELDS) : [];
  });
  /** "Básico: Nome do personagem, Raça. Atributos: Força." — or `''` when
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
  protected readonly lead = computed(() => {
    const s = this.state();
    if (s.status !== 'ready') {
      return '';
    }
    const kind = characterKindLabel(s.kind);
    if (s.mode === 'edit') {
      return isFullSheetKind(s.kind)
        ? 'As mudanças valem quando você salvar. Modificadores, Classe de Armadura e pontos de vida são recalculados na hora.'
        : `Ficha curta de ${kind.toLowerCase()}.`;
    }
    if (s.kind === 'player') {
      return 'Preencha os passos na ordem que quiser. Modificadores, Classe de Armadura e pontos de vida são calculados quando você criar.';
    }
    return isFullSheetKind(s.kind)
      ? `${kind} com ficha completa, como a de um jogador. Só você vê os NPCs da campanha.`
      : `${kind}: ficha curta, só com o que se usa na mesa. Só você vê os NPCs da campanha.`;
  });

  protected readonly pageTitle = computed(() => {
    const s = this.state();
    if (s.status === 'loading') {
      return s.title;
    }
    return s.status === 'ready' ? titleFor(s.mode, s.kind) : 'Não foi possível abrir o formulário';
  });

  /** Cancel goes back where the person came from: the sheet being edited,
   * or the campaign. */
  protected readonly cancelLink = computed(() => {
    const s = this.state();
    if (s.status !== 'ready') {
      return ['/'];
    }
    return s.characterId
      ? ['/campanhas', s.campaignId, 'personagens', s.characterId]
      : ['/campanhas', s.campaignId];
  });

  /** What the buttons do, in one line next to them. */
  protected readonly actionsNote = computed(() => {
    const s = this.state();
    if (s.status !== 'ready') {
      return '';
    }
    const full = isFullSheetKind(s.kind);
    if (s.mode === 'create') {
      return full
        ? 'Cria com o que estiver preenchido em todos os passos. Cancelar volta para a campanha sem criar.'
        : 'Cancelar volta para a campanha sem criar.';
    }
    return full
      ? 'Salva todos os passos de uma vez. Cancelar volta para a ficha sem salvar.'
      : 'Cancelar volta para a ficha sem salvar.';
  });

  protected stepHasError(step: EditorStepKey): boolean {
    return this.stepsWithErrors().has(step);
  }

  constructor() {
    this.route.paramMap.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((params) => {
      this.resolveAndLoad(params);
    });
  }

  private resolveAndLoad(params: ParamMap): void {
    const campaignId = params.get('id');
    if (!campaignId) {
      return;
    }
    const characterId = params.get('characterId');
    const tipo = params.get('tipo');

    if (characterId) {
      this.loadForEdit(campaignId, characterId);
      return;
    }
    const kind: CharacterKind = tipo ? (TIPO_TO_KIND[tipo] ?? 'enemy') : 'player';
    this.loadForCreate(campaignId, kind);
  }

  private loadForCreate(campaignId: string, kind: CharacterKind): void {
    this.state.set({ status: 'loading', title: titleFor('create', kind) });
    this.source.loadCatalog(campaignId).then(
      (catalog) => {
        this.state.set({
          status: 'ready',
          mode: 'create',
          kind,
          campaignId,
          characterId: null,
          revision: 1,
          catalog,
        });
      },
      (err: unknown) =>
        this.state.set({
          status: 'error',
          message: describeCharacterError(err),
          backLink: ['/campanhas', campaignId],
          backLabel: 'Voltar para a campanha',
        }),
    );
  }

  private loadForEdit(campaignId: string, characterId: string): void {
    this.state.set({ status: 'loading', title: titleFor('edit', 'player') });
    Promise.all([
      this.source.loadCatalog(campaignId),
      this.source.loadCharacterForEdit(campaignId, characterId),
    ])
      .then(([catalog, existing]) => {
        this.state.set({
          status: 'ready',
          mode: 'edit',
          kind: existing.kind,
          campaignId,
          characterId,
          revision: existing.revision,
          catalog,
        });
        if (existing.full) {
          this.patchFullForm(existing.full);
        }
        if (existing.basic) {
          this.basicForm.setValue(existing.basic);
        }
      })
      .catch((err: unknown) => {
        // "sheet_locked" is the common case here: a player opened the edit
        // URL for a sheet the server has since locked (RN-01).
        this.state.set({
          status: 'error',
          message: describeCharacterError(err),
          backLink: ['/campanhas', campaignId, 'personagens', characterId],
          backLabel: 'Voltar para a ficha',
        });
      });
  }

  private patchFullForm(full: CharacterFormValue): void {
    this.fullForm.setValue({
      name: full.name,
      race: full.race,
      subrace: full.subrace,
      className: full.className,
      subclassName: full.subclassName,
      customSubclassName: full.customSubclassName,
      level: full.level,
      background: full.background,
      customBackgroundName: full.customBackgroundName,
      armor: full.armor,
      shield: full.shield,
      weaponKeys: full.weapons,
      equipmentText: full.equipmentText,
      languagesText: full.languagesText,
      toolProficienciesText: full.toolProficienciesText,
      experiencePoints: full.experiencePoints,
      alignment: full.alignment,
      customFeaturesText: full.customFeaturesText,
      hitPointsMethod: full.hitPointsMethod,
      abilities: full.abilities,
      extraAbilityBonuses: full.extraAbilityBonuses,
    });
    this.selectedSkills.set(new Set(full.skillProficiencies));
    this.customBackgroundSkills.set(new Set(full.customBackgroundSkills ?? []));
    this.expertiseSkills.set(new Set(full.expertiseSkillKeys));
    this.hitPointsRolls.set(full.hitPointsRolls);
    this.selectedCantrips.set(new Set(full.cantrips));
    this.selectedSpellsKnown.set(new Set(full.spellsKnown));
    this.selectedSpellsPrepared.set(new Set(full.spellsPrepared));
  }

  protected toggleSkill(key: string): void {
    const next = new Set(this.selectedSkills());
    if (next.has(key)) {
      next.delete(key);
      // Expertise requires proficiency first — drop it too so the
      // invariant (expertise ⊆ proficient) never breaks silently.
      if (this.expertiseSkills().has(key)) {
        const nextExpertise = new Set(this.expertiseSkills());
        nextExpertise.delete(key);
        this.expertiseSkills.set(nextExpertise);
      }
    } else {
      next.add(key);
    }
    this.selectedSkills.set(next);
  }

  /** Toggles expertise on a proficient skill (doubles its proficiency
   * bonus — Bard, Rogue). A no-op on a skill that is not proficient yet:
   * the checkbox is disabled for those in the template. */
  protected toggleExpertise(key: string): void {
    if (!this.selectedSkills().has(key)) {
      return;
    }
    const next = new Set(this.expertiseSkills());
    if (next.has(key)) {
      next.delete(key);
    } else {
      next.add(key);
    }
    this.expertiseSkills.set(next);
  }

  private toggleInSet(current: ReadonlySet<string>, key: string): Set<string> {
    const next = new Set(current);
    if (next.has(key)) {
      next.delete(key);
    } else {
      next.add(key);
    }
    return next;
  }

  protected toggleCantrip(key: string): void {
    this.selectedCantrips.set(this.toggleInSet(this.selectedCantrips(), key));
  }

  protected toggleSpellKnown(key: string): void {
    this.selectedSpellsKnown.set(this.toggleInSet(this.selectedSpellsKnown(), key));
  }

  protected toggleSpellPrepared(key: string): void {
    this.selectedSpellsPrepared.set(this.toggleInSet(this.selectedSpellsPrepared(), key));
  }

  /** Only the selected keys that are still in the (class-filtered) catalog
   * list — see `buildFullValue`'s comment. */
  private intersectWithAvailable(
    selected: ReadonlySet<string>,
    available: readonly { readonly key: string }[],
  ): string[] {
    const availableKeys = new Set(available.map((item) => item.key));
    return Array.from(selected).filter((key) => availableKeys.has(key));
  }

  /** One roll per level after the first ("Dados de Vida", hit points
   * method "rolled") — `rollsNeeded()` inputs, indexed from 0 (level 2). */
  protected setHitPointsRoll(index: number, value: number): void {
    const next = [...this.hitPointsRolls()];
    next[index] = value;
    this.hitPointsRolls.set(next);
  }

  /** A roll may not be typed in yet — plain array indexing in the template
   * would type as `number` (TypeScript does not bounds-check indexing),
   * hiding that. */
  protected rollValueAt(index: number): number | undefined {
    return this.hitPointsRolls()[index];
  }

  /** Toggles one of the custom background's two granted skills. Caps at 2
   * (`CustomBackground.skill_keys`: "at most 2"): a third click while two
   * are already chosen does nothing, so the player unchecks one first. */
  protected toggleCustomBackgroundSkill(key: string): void {
    const next = new Set(this.customBackgroundSkills());
    if (next.has(key)) {
      next.delete(key);
    } else if (next.size < 2) {
      next.add(key);
    }
    this.customBackgroundSkills.set(next);
  }

  private buildFullValue(): CharacterFormValue {
    const v = this.fullForm.getRawValue();
    const bgSkills = Array.from(this.customBackgroundSkills());
    return {
      name: v.name,
      race: v.race,
      subrace: v.subrace,
      className: v.className,
      subclassName: v.subclassName,
      customSubclassName: v.customSubclassName,
      level: v.level,
      background: v.background,
      customBackgroundName: v.customBackgroundName,
      // CustomBackground.skill_keys: "fewer than 2 shows an issue" — the
      // server reports that; the editor just sends what was chosen, even 0
      // or 1, rather than guessing or blocking submission over it.
      customBackgroundSkills:
        v.background === 'custom' && bgSkills.length === 2 ? [bgSkills[0], bgSkills[1]] : null,
      skillProficiencies: Array.from(this.selectedSkills()),
      expertiseSkillKeys: Array.from(this.expertiseSkills()),
      abilities: v.abilities,
      extraAbilityBonuses: v.extraAbilityBonuses,
      hitPointsMethod: v.hitPointsMethod,
      // Only as many rolls as the current level needs — a roll left over
      // from a higher level typed earlier is dropped, not sent stale.
      hitPointsRolls: this.hitPointsRolls().slice(0, this.rollsNeeded()),
      isCaster: this.isCaster(),
      // Never send a stale pick: if the class or level changed after a
      // spell was chosen and it dropped off the (class-filtered) catalog
      // list, it never reaches the server — no typed key ever could get
      // here in the first place (integrator fix).
      cantrips: this.intersectWithAvailable(this.selectedCantrips(), this.availableCantrips()),
      spellsKnown: this.intersectWithAvailable(this.selectedSpellsKnown(), this.availableSpells()),
      spellsPrepared: this.intersectWithAvailable(
        this.selectedSpellsPrepared(),
        this.availableSpells(),
      ),
      armor: v.armor,
      shield: v.shield,
      weapons: v.weaponKeys,
      equipmentText: v.equipmentText,
      languagesText: v.languagesText,
      toolProficienciesText: v.toolProficienciesText,
      experiencePoints: v.experiencePoints,
      alignment: v.alignment,
      customFeaturesText: v.customFeaturesText,
    };
  }

  /** After a submit with an invalid field: opens the step of the first one
   * (and "Bônus manuais", if that is where it is), so the field and its
   * error message are on screen. */
  private openFirstInvalidStep(): void {
    const [first] = invalidFields(this.fullForm, FULL_SHEET_FIELDS);
    if (!first?.step) {
      return;
    }
    if (first.path.startsWith('extraAbilityBonuses.')) {
      this.bonusesOpen.set(true);
    }
    const stepper = this.stepper;
    if (!stepper) {
      return;
    }
    const index = stepper.steps
      .toArray()
      .findIndex((step) => step.label === EDITOR_STEP_LABELS[first.step!]);
    if (index >= 0 && index !== stepper.selectedIndex) {
      stepper.goTo(index);
    }
  }

  protected async submit(): Promise<void> {
    const s = this.state();
    if (s.status !== 'ready') {
      return;
    }
    const isBasic = !isFullSheetKind(s.kind);
    const form = isBasic ? this.basicForm : this.fullForm;
    if (form.invalid) {
      form.markAllAsTouched();
      this.saveState.set({ status: 'idle' });
      this.showErrors.set(true);
      if (!isBasic) {
        this.openFirstInvalidStep();
      }
      return;
    }

    this.saveState.set({ status: 'saving' });
    try {
      if (s.mode === 'create') {
        const res = await this.source.createCharacter({
          campaignId: s.campaignId,
          kind: s.kind,
          full: isBasic ? null : this.buildFullValue(),
          basic: isBasic ? this.basicForm.getRawValue() : null,
        });
        await this.router.navigate(['/campanhas', s.campaignId, 'personagens', res.characterId]);
      } else if (s.characterId) {
        await this.source.updateCharacter({
          campaignId: s.campaignId,
          characterId: s.characterId,
          revision: s.revision,
          name: isBasic ? this.basicForm.getRawValue().name : this.fullForm.getRawValue().name,
          full: isBasic ? null : this.buildFullValue(),
          basic: isBasic ? this.basicForm.getRawValue() : null,
        });
        await this.router.navigate(['/campanhas', s.campaignId, 'personagens', s.characterId]);
      }
      this.saveState.set({ status: 'idle' });
    } catch (err) {
      this.saveState.set({
        status: 'error',
        message: describeCharacterError(err),
      });
    }
  }
}
