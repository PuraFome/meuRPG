import { computed, signal } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';

import { isFullSheetKind } from '../../core/characters/characters.types';
import type { CharacterFormValue } from './character-editor.types';
import type { EditorCatalog } from './editor-catalog';
import {
  type EditorField,
  type EditorStepKey,
  describeBonusesInUse,
  describeInvalidFields,
} from './editor-labels';
import type { EditorCore } from './editor-core';
import { buildFullValue, classProblemsOf, invalidFullFieldsOf, rollProblemsOf } from './full-draft';
import { invalidBasicFields } from './npc-short-form/basic-form';
import { keepAvailable } from './spell-views';

/** What `EditorDraft` reads from the page. */
export interface EditorDraftHost extends EditorCore {
  readonly catalog: EditorCatalog;
}

/**
 * The draft on the page and what is wrong with it: the full sheet the form, the picks and the lists add up to,
 * the problems that have no control of their own (the class blocks, the rolls, the placing of the scores), the
 * fields a save tried and found invalid, and the summary above the buttons. A plain class the editor builds.
 */
export class EditorDraft {
  /** "Rolar 4d6" or "Conjunto padrão" with results still to place: saving
   * waits, so a half-placed roll never turns into six default 10s. */
  readonly abilitiesIncomplete = signal(false);
  /** What the table's way of making scores still lacks, in words ("role as habilidades"), `''` when it is complete. */
  readonly abilitiesProblem = signal('');
  /** Set by a submit with an invalid field: from then on, the notice above
   * the buttons lists what is still wrong, and the steps that have it are
   * marked, until everything is fixed. */
  readonly showErrors = signal(false);
  private readonly fullFormValue;
  private readonly basicFormValue;

  /** What the class blocks still lack, in words ("Classe 2", "o nível total vai até 20"): not controls of the
   * form, so the page adds them to the invalid fields while they stand. */
  readonly classProblems = computed<string[]>(() =>
    classProblemsOf(this.host.picks.extraClasses(), this.host.catalog.totalLevelProblem()),
  );

  /** With "Rolado", the levels whose roll is empty or does not fit that level's die: the form keeps no control for
   * them, so the page adds them to the invalid fields while they stand. */
  readonly rollProblems = computed<string[]>(() => {
    const c = this.host.catalog;
    return rollProblemsOf(
      c.selectedHitPointsMethod() === 'rolled',
      c.hitDie(),
      c.rollsNeeded(),
      c.levelDice(),
      this.host.picks.hitPointsRolls(),
    );
  });

  private readonly invalidFullFields = computed(() => {
    this.fullFormValue();
    return this.showErrors() ? this.currentInvalidFullFields() : [];
  });
  private readonly invalidBasicFields = computed(() => {
    this.basicFormValue();
    return this.showErrors() ? invalidBasicFields(this.host.basicForm) : [];
  });
  /** "Básico: Nome do personagem, Raça. Habilidades: Força." — or `''` when
   * nothing needs fixing (or no submit was tried yet). */
  readonly invalidSummary = computed(() => {
    const s = this.host.state();
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
  /** "Constituição +1, Inteligência +2", so the closed section still says
   * what it holds. */
  readonly bonusesInUse = computed(() => describeBonusesInUse(this.host.catalog.bonusesValue()));

  constructor(private readonly host: EditorDraftHost) {
    this.fullFormValue = toSignal(host.fullForm.valueChanges);
    this.basicFormValue = toSignal(host.basicForm.valueChanges);
  }

  stepHasError(step: EditorStepKey): boolean {
    return this.stepsWithErrors().has(step);
  }

  /** Only the selected keys that are still in the (class-filtered) catalog
   * list — see `build`'s comment. */
  intersectWithAvailable(
    selected: ReadonlySet<string>,
    available: readonly { readonly key: string }[],
  ): string[] {
    return keepAvailable(selected, available, this.host.catalog.catalog()?.spells ?? []);
  }

  build(): CharacterFormValue {
    const { catalog, picks, spellPicks } = this.host;
    return buildFullValue({
      form: this.host.fullForm.getRawValue(),
      customBackgroundSkills: picks.customBackgroundSkills(),
      customBackgroundProficiencies: picks.customBackgroundProficiencies(),
      extraClasses: picks.extraClasses(),
      chosenSkills: catalog.chosenSkills(),
      expertiseSkills: picks.expertiseSkills(),
      hitPointsRolls: picks.hitPointsRolls(),
      rollsNeeded: catalog.rollsNeeded(),
      isCaster: catalog.isCaster(),
      cantrips: this.intersectWithAvailable(spellPicks.cantrips(), catalog.availableCantrips()),
      spellsKnown: this.intersectWithAvailable(spellPicks.known(), catalog.availableSpells()),
      spellsPrepared: this.intersectWithAvailable(spellPicks.prepared(), catalog.availableSpells()),
      featureChoiceKeys: picks.featureChoiceKeys(),
      featureChoiceText: picks.featureChoiceText(),
      feats: catalog.master() ? { keys: picks.featKeys(), slots: picks.featSlots() } : null,
    });
  }

  /** The full sheet's invalid fields right now, plus the placing that is not finished (it has no control of its
   * own: see `abilitiesIncomplete`). */
  currentInvalidFullFields(): EditorField[] {
    return invalidFullFieldsOf(this.host.fullForm, this.classProblems(), this.rollProblems(), {
      incomplete: this.abilitiesIncomplete(),
      problem: this.abilitiesProblem(),
    });
  }

  /** The form of the kind being saved is invalid, or what the full sheet keeps outside its controls is unfinished. */
  hasProblems(isBasic: boolean): boolean {
    if (isBasic) {
      return this.host.basicForm.invalid;
    }
    return (
      this.host.fullForm.invalid ||
      this.abilitiesIncomplete() ||
      this.classProblems().length > 0 ||
      this.rollProblems().length > 0
    );
  }
}
