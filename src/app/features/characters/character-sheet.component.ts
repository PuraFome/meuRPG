import { Component, Input, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormBuilder, FormGroup, ReactiveFormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { StoreService } from '../../core/store/store.service';
import { CharacterSheetFieldsComponent } from './character-sheet-fields.component';
import {
  buildAttributesFromForm,
  buildInventoryFromForm,
  buildMinionFromForm,
  buildSheetFromForm,
  buildSkillsFromForm,
  createCharacterForm,
} from './sheet-form.factory';
import type { Character } from '../../core/models/character';

/**
 * Editable "Ficha" tab. Delegates every field to the shared guided
 * `CharacterSheetFieldsComponent` so creation and editing never diverge.
 */
@Component({
  selector: 'app-character-sheet',
  standalone: true,
  imports: [
    CommonModule,
    ReactiveFormsModule,
    MatButtonModule,
    MatIconModule,
    CharacterSheetFieldsComponent,
  ],
  template: `
    <form [formGroup]="sheetForm" class="character-sheet" (ngSubmit)="onSave()">
      <app-character-sheet-fields [form]="sheetForm" />

      @if (saveError) {
        <div class="save-error" role="alert">
          <mat-icon>error_outline</mat-icon>
          <div>
            <strong>Não foi possível salvar a ficha.</strong>
            <span>{{ saveError }}</span>
          </div>
        </div>
      }

      <div class="form-actions">
        <button
          mat-raised-button
          color="primary"
          type="submit"
          [disabled]="sheetForm.invalid || saving"
        >
          <mat-icon>{{ saving ? 'hourglass_top' : 'save' }}</mat-icon>
          {{ saving ? 'Salvando...' : 'Salvar' }}
        </button>

        @if (saved) {
          <span class="save-feedback">Ficha salva!</span>
        }
      </div>
    </form>
  `,
  styles: `
    :host {
      display: block;
      padding-top: 8px;
    }

    .character-sheet {
      display: flex;
      flex-direction: column;
      gap: 24px;
    }

    .form-actions {
      display: flex;
      align-items: center;
      gap: 16px;
      padding: 8px 0;
    }

    .save-error {
      display: flex;
      align-items: flex-start;
      gap: 12px;
      padding: 14px 16px;
      border-radius: 10px;
      border: 1px solid rgba(239, 68, 68, 0.5);
      background: rgba(239, 68, 68, 0.12);
      color: #fecaca;
    }

    .save-error mat-icon {
      flex-shrink: 0;
      margin-top: 2px;
      color: #f87171;
    }

    .save-error div {
      display: flex;
      flex-direction: column;
      gap: 4px;
      font-size: 0.9rem;
      line-height: 1.4;
    }

    .save-feedback {
      color: #4ade80;
      font-size: 0.875rem;
      font-weight: 500;
      animation: fadeIn 0.3s ease;
    }

    @keyframes fadeIn {
      from { opacity: 0; transform: translateY(-4px); }
      to   { opacity: 1; transform: translateY(0); }
    }
  `,
})
export class CharacterSheetComponent implements OnInit {
  @Input() character: Character | null = null;

  private readonly fb = inject(FormBuilder);
  private readonly store = inject(StoreService<Character>);

  sheetForm!: FormGroup;

  saved = false;
  saving = false;
  saveError: string | null = null;

  ngOnInit(): void {
    this.sheetForm = createCharacterForm(this.fb, this.character);
  }

  onSave(): void {
    if (this.sheetForm.invalid || this.saving || !this.character) return;
    this.saving = true;
    this.saveError = null;

    try {
      const updated = this.buildUpdated();
      this.store.update('characters', updated.id, updated);
      this.character = updated;
      this.saved = true;
      setTimeout(() => (this.saved = false), 2000);
    } catch (err) {
      this.saveError = this.describeSaveError(err);
    } finally {
      this.saving = false;
    }
  }

  private describeSaveError(err: unknown): string {
    if (err instanceof Error && err.message) {
      const e = err as unknown as { name?: string };
      if (e.name === 'QuotaExceededError' || e.name === 'NS_ERROR_DOM_QUOTA_REACHED') {
        return 'O armazenamento do navegador está cheio. Exclua personagens ou dados antigos para liberar espaço.';
      }
      return err.message;
    }
    return 'Ocorreu um erro inesperado ao salvar. Tente novamente.';
  }

  private buildUpdated(): Character {
    const character = this.character!;
    const updated: Character = {
      ...character,
      attributes: buildAttributesFromForm(this.sheetForm),
      skills: buildSkillsFromForm(this.sheetForm),
      inventory: buildInventoryFromForm(this.sheetForm),
      updatedAt: new Date(),
    };

    updated.sheet = buildSheetFromForm(this.sheetForm, character.type);

    if (character.type === 'minion') {
      updated.minion = buildMinionFromForm(this.sheetForm);
      updated.attributes = {};
      updated.skills = [];
      updated.inventory = [];
    }

    return updated;
  }
}