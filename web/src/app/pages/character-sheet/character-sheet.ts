import { Component, DestroyRef, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';

import {
  abilityLabel,
  characterKindLabel,
  characterStateLabel,
  formatDateTime,
  formatModifier,
  formatSpeedFt,
  formatSpellSlots,
  skillProficiencyLabel,
  splitArmorDescription,
} from '../../core/characters/character-labels';
import { describeCharacterError } from '../../core/characters/character-errors';
import { FictionNotice } from '../../shared/fiction-notice/fiction-notice';
import {
  BasicSheetVm,
  CharacterSheetSource,
  CharacterSheetVm,
  CharacterStoryVm,
  FullSheetVm,
} from './character-sheet.types';

type PageState =
  | { status: 'loading' }
  | { status: 'not-found' }
  | { status: 'error'; message: string }
  | { status: 'ready'; vm: CharacterSheetVm };

type MasterNotesState =
  | { status: 'not-applicable' }
  | { status: 'loading' }
  | { status: 'ready'; notes: string }
  | { status: 'error'; message: string };

type SavingState = { status: 'idle' } | { status: 'saving' } | { status: 'error'; message: string };

const emptyStory: CharacterStoryVm = {
  personality: { traits: '', ideals: '', bonds: '', flaws: '' },
  appearance: { age: '', height: '', weight: '', eyes: '', skin: '', hair: '', description: '' },
  backstory: '',
  allies: '',
};

/**
 * "/campanhas/:id/personagens/:characterId" (MR-004): the sheet, following
 * the official PDF's layout — desktop three independent columns (each its
 * own flex stack, so a long "Características e traços" never pushes
 * another column's sections down), mobile a single column, every section a
 * `<section>` with an `h2` (see the template and `character-sheet.scss`'s
 * `.sheet-grid` / `.sheet-column`).
 *
 * The browser never computes a rule (ADR-0008): everything under `vm.sheet`
 * is exactly what `GetCharacter` sent, only formatted for display.
 *
 * Master-only: the "Notas do mestre" panel and "Marcar como morto" — never
 * fetched or rendered for a player (RN-11; `character-sheet.spec.ts` checks
 * `getMasterNotes` is never called for one).
 *
 * The story (personality, appearance, backstory, allies) is independent of
 * "Editar ficha" (RN-01's lock, driven by `canEdit`): the master can always
 * edit it, and can toggle whether the player currently can too — "Permitir
 * editar a história" / "Travar a história" — for a locked sheet
 * (`canToggleStoryEditing`, `storyEditingAllowed`). "Editar história" itself
 * only ever reads `canEditStory`, whatever the caller's role (integrator
 * amendment to A3, 29/09/2026).
 */
@Component({
  selector: 'app-character-sheet',
  imports: [
    FictionNotice,
    MatButtonModule,
    MatCardModule,
    MatFormFieldModule,
    MatInputModule,
    MatProgressSpinnerModule,
    ReactiveFormsModule,
    RouterLink,
  ],
  templateUrl: './character-sheet.html',
  styleUrl: './character-sheet.scss',
})
export class CharacterSheetPage {
  private readonly source = inject(CharacterSheetSource);
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly destroyRef = inject(DestroyRef);
  private readonly fb = inject(FormBuilder);

  protected readonly state = signal<PageState>({ status: 'loading' });
  protected readonly masterNotesState = signal<MasterNotesState>({ status: 'not-applicable' });
  protected readonly notesSaveState = signal<SavingState>({ status: 'idle' });
  protected readonly markDeadState = signal<SavingState>({ status: 'idle' });
  protected readonly storyEditing = signal(false);
  protected readonly storySaveState = signal<SavingState>({ status: 'idle' });
  protected readonly storyToggleState = signal<SavingState>({ status: 'idle' });
  /** "Aprovar personagem" / "Recusar personagem" (MR-024). */
  protected readonly approvalState = signal<SavingState>({ status: 'idle' });
  /** Rejecting deletes the character for good, so it takes a second click
   * ("Confirmar recusa") after "Recusar personagem". */
  protected readonly confirmingReject = signal(false);

  protected readonly abilityLabel = abilityLabel;
  protected readonly characterKindLabel = characterKindLabel;
  protected readonly characterStateLabel = characterStateLabel;
  protected readonly formatDateTime = formatDateTime;
  protected readonly formatModifier = formatModifier;
  protected readonly formatSpeedFt = formatSpeedFt;
  protected readonly formatSpellSlots = formatSpellSlots;
  protected readonly skillProficiencyLabel = skillProficiencyLabel;
  protected readonly splitArmorDescription = splitArmorDescription;

  protected readonly notesForm = this.fb.nonNullable.group({
    notes: ['', Validators.maxLength(20000)],
  });

  protected readonly storyForm = this.fb.nonNullable.group({
    traits: ['', Validators.maxLength(1000)],
    ideals: ['', Validators.maxLength(1000)],
    bonds: ['', Validators.maxLength(1000)],
    flaws: ['', Validators.maxLength(1000)],
    age: ['', Validators.maxLength(40)],
    height: ['', Validators.maxLength(40)],
    weight: ['', Validators.maxLength(40)],
    eyes: ['', Validators.maxLength(40)],
    skin: ['', Validators.maxLength(40)],
    hair: ['', Validators.maxLength(40)],
    appearanceDescription: ['', Validators.maxLength(2000)],
    backstory: ['', Validators.maxLength(10000)],
    allies: ['', Validators.maxLength(2000)],
  });

  constructor() {
    this.route.paramMap.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((params) => {
      const campaignId = params.get('id');
      const characterId = params.get('characterId');
      if (campaignId && characterId) {
        this.load(campaignId, characterId);
      }
    });
  }

  private load(campaignId: string, characterId: string): void {
    this.state.set({ status: 'loading' });
    this.masterNotesState.set({ status: 'not-applicable' });
    this.source.getCharacterSheet(campaignId, characterId).then(
      (vm) => {
        this.state.set({ status: 'ready', vm });
        if (vm.canAccessMasterNotes) {
          this.loadMasterNotes(campaignId, characterId);
        }
      },
      (err: unknown) => {
        this.state.set({ status: 'error', message: describeCharacterError(err) });
      },
    );
  }

  private loadMasterNotes(campaignId: string, characterId: string): void {
    this.masterNotesState.set({ status: 'loading' });
    this.source.getMasterNotes(campaignId, characterId).then(
      (notes) => {
        this.masterNotesState.set({ status: 'ready', notes });
        this.notesForm.setValue({ notes });
      },
      (err: unknown) => {
        this.masterNotesState.set({ status: 'error', message: describeCharacterError(err) });
      },
    );
  }

  protected async saveNotes(campaignId: string, characterId: string): Promise<void> {
    if (this.notesForm.invalid) {
      this.notesForm.markAllAsTouched();
      return;
    }
    this.notesSaveState.set({ status: 'saving' });
    try {
      const notes = this.notesForm.getRawValue().notes;
      await this.source.updateMasterNotes(campaignId, characterId, notes);
      this.notesSaveState.set({ status: 'idle' });
      this.masterNotesState.set({ status: 'ready', notes });
    } catch (err) {
      this.notesSaveState.set({ status: 'error', message: describeCharacterError(err) });
    }
  }

  protected async markDead(campaignId: string, characterId: string): Promise<void> {
    this.markDeadState.set({ status: 'saving' });
    try {
      const vm = await this.source.markCharacterDead(campaignId, characterId);
      this.state.set({ status: 'ready', vm });
      this.markDeadState.set({ status: 'idle' });
    } catch (err) {
      this.markDeadState.set({
        status: 'error',
        message: describeCharacterError(err),
      });
    }
  }

  protected startEditingStory(story: CharacterStoryVm | null): void {
    const s = story ?? emptyStory;
    this.storyForm.setValue({
      traits: s.personality.traits,
      ideals: s.personality.ideals,
      bonds: s.personality.bonds,
      flaws: s.personality.flaws,
      age: s.appearance.age,
      height: s.appearance.height,
      weight: s.appearance.weight,
      eyes: s.appearance.eyes,
      skin: s.appearance.skin,
      hair: s.appearance.hair,
      appearanceDescription: s.appearance.description,
      backstory: s.backstory,
      allies: s.allies,
    });
    this.storySaveState.set({ status: 'idle' });
    this.storyEditing.set(true);
  }

  protected cancelEditingStory(): void {
    this.storyEditing.set(false);
  }

  /** Master only: flips whether the player may currently edit the story
   * (integrator amendment to A3, 29/09/2026; `SetStoryEditing` takes no
   * revision and never changes one — see `CharacterSheetSource`'s doc
   * comment). Never changes `canEditStory` directly — the next
   * `getCharacterSheet` / mutation response is what updates it, same as
   * every other server-computed flag. */
  protected async toggleStoryEditingAllowed(
    campaignId: string,
    characterId: string,
    currentlyAllowed: boolean,
  ): Promise<void> {
    this.storyToggleState.set({ status: 'saving' });
    try {
      const vm = await this.source.setStoryEditingAllowed(campaignId, characterId, !currentlyAllowed);
      this.state.set({ status: 'ready', vm });
      this.storyToggleState.set({ status: 'idle' });
    } catch (err) {
      this.storyToggleState.set({ status: 'error', message: describeCharacterError(err) });
    }
  }

  /** Master only (MR-024): the character joins the campaign as a draft, and
   * its player as a member. The response is the approved character, so the
   * page simply shows it. */
  protected async approve(campaignId: string, characterId: string): Promise<void> {
    this.approvalState.set({ status: 'saving' });
    try {
      const vm = await this.source.approveCharacter(campaignId, characterId);
      this.state.set({ status: 'ready', vm });
      this.approvalState.set({ status: 'idle' });
    } catch (err) {
      this.approvalState.set({ status: 'error', message: describeCharacterError(err) });
    }
  }

  /** Master only (MR-024), after "Confirmar recusa": the character is gone,
   * so the master goes back to the campaign's page. */
  protected async reject(campaignId: string, characterId: string): Promise<void> {
    this.approvalState.set({ status: 'saving' });
    try {
      await this.source.rejectCharacter(campaignId, characterId);
      this.approvalState.set({ status: 'idle' });
      await this.router.navigate(['/campanhas', campaignId]);
    } catch (err) {
      this.confirmingReject.set(false);
      this.approvalState.set({ status: 'error', message: describeCharacterError(err) });
    }
  }

  /** Explicit casts for the template, which narrows `vm.sheet.kind` in an
   * `@if` but cannot carry that narrowing into a `@let` binding. */
  protected asFullSheet(sheet: FullSheetVm | BasicSheetVm): FullSheetVm {
    return sheet as FullSheetVm;
  }

  protected asBasicSheet(sheet: FullSheetVm | BasicSheetVm): BasicSheetVm {
    return sheet as BasicSheetVm;
  }

  /** "Equipamento" lists the weapons carried by name, next to the armor and
   * shield: `DerivedSheet.attacks` already lists both weapon attacks and
   * damage cantrips (`AttackVm.kind`), so the weapons are simply the
   * `'weapon'` ones — no separate request for `FullSheet.weapon_keys`'
   * names (integrator fix: the section used to show only free-text items
   * and coins, never what the player actually equipped). */
  protected weaponNames(sheet: FullSheetVm): readonly string[] {
    return sheet.attacks.filter((a) => a.kind === 'weapon').map((a) => a.namePt);
  }

  protected async saveStory(campaignId: string, characterId: string, revision: number): Promise<void> {
    if (this.storyForm.invalid) {
      this.storyForm.markAllAsTouched();
      return;
    }
    this.storySaveState.set({ status: 'saving' });
    const v = this.storyForm.getRawValue();
    const story: CharacterStoryVm = {
      personality: { traits: v.traits, ideals: v.ideals, bonds: v.bonds, flaws: v.flaws },
      appearance: {
        age: v.age,
        height: v.height,
        weight: v.weight,
        eyes: v.eyes,
        skin: v.skin,
        hair: v.hair,
        description: v.appearanceDescription,
      },
      backstory: v.backstory,
      allies: v.allies,
    };
    try {
      const vm = await this.source.updateCharacterStory(campaignId, characterId, revision, story);
      this.state.set({ status: 'ready', vm });
      this.storySaveState.set({ status: 'idle' });
      this.storyEditing.set(false);
    } catch (err) {
      this.storySaveState.set({ status: 'error', message: describeCharacterError(err) });
    }
  }
}
