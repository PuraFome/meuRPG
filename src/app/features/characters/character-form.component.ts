import { ChangeDetectorRef, Component, EventEmitter, Input, OnInit, Output, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormBuilder, FormControl, FormGroup, ReactiveFormsModule } from '@angular/forms';
import { Router } from '@angular/router';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { PageHeaderComponent } from '../../shared/components/page-header.component';
import { StoreService } from '../../core/store/store.service';
import { CharactersService } from '../../core/services/characters.service';
import { AuthService } from '../../core/auth/auth.service';
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
import type { BreadcrumbItem } from '../../shared/components/page-header.component';

@Component({
  selector: 'app-character-form',
  standalone: true,
  imports: [
    CommonModule,
    ReactiveFormsModule,
    MatButtonModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatSelectModule,
    PageHeaderComponent,
    CharacterSheetFieldsComponent,
  ],
  template: `
    <app-page-header
      [title]="'Novo Personagem'"
      icon="person"
      [breadcrumbs]="breadcrumbs"
    />

    <form [formGroup]="characterForm" class="character-form" (ngSubmit)="onSubmit()">
      <!-- ═══════════════ Identificação ═══════════════ -->
      <section class="form-section">
        <h2 class="section-title">Identificação</h2>

        <div class="field-grid">
          <mat-form-field appearance="outline" subscriptSizing="dynamic" class="name-field">
            <mat-label>Nome</mat-label>
            <input matInput [formControl]="nameControl" placeholder="Nome do personagem" />
            @if (nameControl.invalid && nameControl.touched) {
              <mat-error>O nome é obrigatório</mat-error>
            }
          </mat-form-field>

          @if (!isJoinMode) {
            <mat-form-field appearance="outline" subscriptSizing="dynamic" class="type-field">
              <mat-label>Tipo</mat-label>
              <mat-select [formControl]="typeControl">
                <mat-option value="player">Jogador</mat-option>
                <mat-option value="npc">NPC</mat-option>
                <mat-option value="boss">Boss</mat-option>
                <mat-option value="minion">Minion</mat-option>
              </mat-select>
            </mat-form-field>
          } @else {
            <mat-form-field appearance="outline" subscriptSizing="dynamic" class="type-field">
              <mat-label>Tipo</mat-label>
              <input matInput value="Jogador" readonly />
            </mat-form-field>
          }
        </div>

        <mat-form-field appearance="outline" subscriptSizing="dynamic" class="description-field">
          <mat-label>Descrição</mat-label>
          <textarea
            matInput
            [formControl]="descriptionControl"
            rows="3"
            placeholder="Descrição opcional"
          ></textarea>
        </mat-form-field>
      </section>

      <!-- ═══════════════ Ficha completa (guiada) ═══════════════ -->
      <app-character-sheet-fields [form]="characterForm" />

      <!-- ═══════════════ Erro ao salvar ═══════════════ -->
      @if (saveError) {
        <div class="save-error" role="alert">
          <mat-icon>error_outline</mat-icon>
          <div>
            <strong>Não foi possível salvar o personagem.</strong>
            <span>{{ saveError }}</span>
          </div>
        </div>
      }

      <!-- ═══════════════ Ações ═══════════════ -->
      <div class="form-actions">
        <button
          mat-raised-button
          color="primary"
          type="submit"
          [disabled]="characterForm.invalid || saving"
        >
          <mat-icon>{{ saving ? 'hourglass_top' : 'save' }}</mat-icon>
          {{ saving ? 'Salvando...' : 'Salvar' }}
        </button>

        <button mat-button type="button" [disabled]="saving" (click)="onCancel()">
          <mat-icon>arrow_back</mat-icon>
          Cancelar
        </button>
      </div>
    </form>
  `,
  styles: `
    :host {
      display: block;
      max-width: 900px;
      margin: 0 auto;
    }

    .character-form {
      display: flex;
      flex-direction: column;
      gap: 24px;
      padding: 8px 0 32px;
    }

    .form-section {
      background: rgba(255, 255, 255, 0.03);
      border: 1px solid rgba(255, 255, 255, 0.06);
      border-radius: 12px;
      padding: 20px 24px 24px;
    }

    .section-title {
      margin: 0 0 16px;
      font-size: 1.15rem;
      font-weight: 600;
      letter-spacing: 0.3px;
      color: #c4b5fd;
    }

    .field-grid {
      display: grid;
      grid-template-columns: 2fr 1fr;
      gap: 16px;
    }

    .field-grid > * {
      min-width: 0;
    }

    .field-grid mat-form-field {
      width: 100%;
    }

    @media (max-width: 560px) {
      .field-grid {
        grid-template-columns: 1fr;
      }
    }

    .description-field {
      width: 100%;
      margin-top: 16px;
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

    .save-error div {
      display: flex;
      flex-direction: column;
      gap: 4px;
      font-size: 0.9rem;
      line-height: 1.4;
    }

    .form-actions {
      display: flex;
      align-items: center;
      gap: 12px;
      padding: 8px 0;
    }
  `,
})
export class CharacterFormComponent implements OnInit {
  private readonly fb = inject(FormBuilder);
  private readonly router = inject(Router);
  private readonly store = inject(StoreService<Character>);
  private readonly characters = inject(CharactersService);
  private readonly auth = inject(AuthService);
  private readonly cdr = inject(ChangeDetectorRef);

  /**
   * `'default'` — standalone creation page (write-through store + navigation).
   * `'join'` — embedded in a share-link page: player-only, saves via the join API.
   */
  @Input() mode: 'default' | 'join' = 'default';

  /** Share-link token, required when `mode === 'join'`. */
  @Input() joinToken: string | null = null;

  @Output() saved = new EventEmitter<Character>();

  get isJoinMode(): boolean {
    return this.mode === 'join';
  }

  saving = false;
  saveError: string | null = null;

  readonly breadcrumbs: BreadcrumbItem[] = [
    { label: 'Personagens', route: '/personagens' },
    { label: 'Novo Personagem' },
  ];

  characterForm!: FormGroup;

  get nameControl(): FormControl {
    return this.characterForm.get('name') as FormControl;
  }

  get typeControl(): FormControl {
    return this.characterForm.get('type') as FormControl;
  }

  get descriptionControl(): FormControl {
    return this.characterForm.get('description') as FormControl;
  }

  ngOnInit(): void {
    this.characterForm = createCharacterForm(this.fb);

    if (this.isJoinMode) {
      const type = this.characterForm.get('type');
      type?.setValue('player', { emitEvent: false });
      type?.disable({ emitEvent: false });
    }
  }

  onCancel(): void {
    this.router.navigate(['/personagens']);
  }

  onSubmit(): void {
    if (this.characterForm.invalid || this.saving) return;

    this.saving = true;
    this.saveError = null;

    const character = this.buildCharacter();

    if (this.isJoinMode && this.joinToken) {
      this.characters.join(this.joinToken, character).subscribe({
        next: (created) => {
          // Redeeming an invite downgrades the account to visitor server-side;
          // refresh so the shell reflects the restricted role immediately.
          void this.auth.load();
          this.saving = false;
          this.saved.emit(created);
          this.cdr.detectChanges();
        },
        error: (err: unknown) => {
          this.saving = false;
          this.saveError = this.describeSaveError(err);
          this.cdr.detectChanges();
        },
      });
      return;
    }

    try {
      this.store.set('characters', character);
    } catch (err) {
      this.saving = false;
      this.saveError = this.describeSaveError(err);
      return;
    }

    this.saving = false;
    this.router.navigate(['/personagens', character.id]);
  }

  private buildCharacter(): Character {
    const raw = this.characterForm.getRawValue() as { name: string; description: string; type: string };
    const type = (this.isJoinMode ? 'player' : (raw.type ?? 'player')) as Character['type'];

    const character: Character = {
      id: crypto.randomUUID(),
      name: raw.name ?? '',
      description: raw.description ?? '',
      type,
      attributes: buildAttributesFromForm(this.characterForm),
      skills: buildSkillsFromForm(this.characterForm),
      inventory: buildInventoryFromForm(this.characterForm),
      quotes: [],
      createdAt: new Date(),
      updatedAt: new Date(),
    };

    character.sheet = buildSheetFromForm(this.characterForm, type);

    if (type === 'minion') {
      character.minion = buildMinionFromForm(this.characterForm);
      character.attributes = {};
      character.skills = [];
      character.inventory = [];
    }

    return character;
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
}