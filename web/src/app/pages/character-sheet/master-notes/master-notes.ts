import { Component, OnInit, inject, input, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';

import { describeCharacterError } from '../../../core/characters/character-errors';
import { FictionNotice } from '../../../shared/fiction-notice/fiction-notice';
import { CharacterSheetSource } from '../character-sheet.types';

type NotesState =
  | { status: 'loading' }
  | { status: 'ready' }
  | { status: 'error'; message: string };

type SavingState =
  | { status: 'idle' }
  | { status: 'saving' }
  | { status: 'saved' }
  | { status: 'error'; message: string };

/**
 * "Notas do mestre" (RN-11): the page renders this only when the character
 * says `canAccessMasterNotes`, so `getMasterNotes` is never called for a
 * player. Loads the notes once, then saves them with `UpdateMasterNotes`.
 */
@Component({
  selector: 'app-master-notes',
  imports: [
    FictionNotice,
    MatButtonModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    ReactiveFormsModule,
  ],
  templateUrl: './master-notes.html',
  styleUrl: './master-notes.scss',
})
export class MasterNotes implements OnInit {
  private readonly source = inject(CharacterSheetSource);

  readonly campaignId = input.required<string>();
  readonly characterId = input.required<string>();

  protected readonly state = signal<NotesState>({ status: 'loading' });
  protected readonly saveState = signal<SavingState>({ status: 'idle' });
  protected readonly form = inject(FormBuilder).nonNullable.group({
    notes: ['', Validators.maxLength(20000)],
  });

  ngOnInit(): void {
    this.source.getMasterNotes(this.campaignId(), this.characterId()).then(
      (notes) => {
        this.form.setValue({ notes });
        this.state.set({ status: 'ready' });
      },
      (err: unknown) => this.state.set({ status: 'error', message: describeCharacterError(err) }),
    );
  }

  protected async save(): Promise<void> {
    if (this.form.invalid) {
      this.form.markAllAsTouched();
      return;
    }
    this.saveState.set({ status: 'saving' });
    try {
      await this.source.updateMasterNotes(
        this.campaignId(),
        this.characterId(),
        this.form.getRawValue().notes,
      );
      this.saveState.set({ status: 'saved' });
    } catch (err) {
      this.saveState.set({ status: 'error', message: describeCharacterError(err) });
    }
  }
}
