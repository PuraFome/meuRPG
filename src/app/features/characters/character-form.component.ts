import { Component, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import {
  FormBuilder,
  FormGroup,
  ReactiveFormsModule,
  Validators,
} from '@angular/forms';
import { Router } from '@angular/router';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { PageHeaderComponent } from '../../shared/components/page-header.component';
import { StoreService } from '../../core/store/store.service';
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
  ],
  template: `
    <!-- Breadcrumbs + Title -->
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
            <input
              matInput
              formControlName="name"
              placeholder="Nome do personagem"
            />
            @if (characterForm.get('name')?.invalid && characterForm.get('name')?.touched) {
              <mat-error>O nome é obrigatório</mat-error>
            }
          </mat-form-field>

          <mat-form-field appearance="outline" subscriptSizing="dynamic" class="type-field">
            <mat-label>Tipo</mat-label>
            <mat-select formControlName="type">
              <mat-option value="player">Jogador</mat-option>
              <mat-option value="npc">NPC</mat-option>
              <mat-option value="boss">Boss</mat-option>
            </mat-select>
          </mat-form-field>
        </div>

        <mat-form-field
          appearance="outline"
          subscriptSizing="dynamic"
          class="description-field"
        >
          <mat-label>Descrição</mat-label>
          <textarea
            matInput
            formControlName="description"
            rows="3"
            placeholder="Descrição opcional"
          ></textarea>
        </mat-form-field>
      </section>

      <!-- ═══════════════ Atributos ═══════════════ -->
      <section class="form-section">
        <h2 class="section-title">Atributos</h2>
        <div class="attributes-grid" formGroupName="attributes">
          @for (attr of attributeKeys; track attr) {
            <div class="attribute-field">
              <mat-form-field appearance="outline" subscriptSizing="dynamic">
                <mat-label>{{ attr | uppercase }}</mat-label>
                <input
                  matInput
                  type="number"
                  min="1"
                  max="30"
                  [formControlName]="attr"
                />
              </mat-form-field>
            </div>
          }
        </div>
      </section>

      <!-- ═══════════════ Ações ═══════════════ -->
      <div class="form-actions">
        <button
          mat-raised-button
          color="primary"
          type="submit"
          [disabled]="characterForm.invalid"
        >
          <mat-icon>save</mat-icon>
          Salvar
        </button>

        <button mat-button type="button" (click)="onCancel()">
          <mat-icon>arrow_back</mat-icon>
          Cancelar
        </button>
      </div>
    </form>
  `,
  styles: `
    :host {
      display: block;
      max-width: 720px;
      margin: 0 auto;
    }

    .character-form {
      display: flex;
      flex-direction: column;
      gap: 24px;
      padding: 8px 0 32px;
    }

    /* ── Section ──────────────────────────── */

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

    /* ── Fields ───────────────────────────── */

    .field-grid {
      display: grid;
      grid-template-columns: 2fr 1fr;
      gap: 16px;
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

    /* ── Attributes grid ──────────────────── */

    .attributes-grid {
      display: grid;
      grid-template-columns: repeat(3, 1fr);
      gap: 16px;
    }

    @media (max-width: 480px) {
      .attributes-grid {
        grid-template-columns: repeat(2, 1fr);
      }
    }

    .attribute-field {
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 4px;
    }

    .attribute-field mat-form-field {
      width: 100%;
      max-width: 120px;
    }

    .attribute-field input {
      text-align: center;
      font-size: 1.25rem;
      font-weight: 700;
    }

    /* ── Actions ──────────────────────────── */

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

  readonly attributeKeys = ['for', 'des', 'con', 'int', 'sab', 'car'] as const;

  readonly breadcrumbs: BreadcrumbItem[] = [
    { label: 'Personagens', route: '/personagens' },
    { label: 'Novo Personagem' },
  ];

  characterForm!: FormGroup;

  ngOnInit(): void {
    this.characterForm = this.fb.group({
      name: ['', Validators.required],
      type: ['player'],
      description: [''],
      attributes: this.fb.group({
        for: [10, [Validators.required, Validators.min(1), Validators.max(30)]],
        des: [10, [Validators.required, Validators.min(1), Validators.max(30)]],
        con: [10, [Validators.required, Validators.min(1), Validators.max(30)]],
        int: [10, [Validators.required, Validators.min(1), Validators.max(30)]],
        sab: [10, [Validators.required, Validators.min(1), Validators.max(30)]],
        car: [10, [Validators.required, Validators.min(1), Validators.max(30)]],
      }),
    });
  }

  /** Navigate back to the character list. */
  onCancel(): void {
    this.router.navigate(['/personagens']);
  }

  /** Build a new Character and persist it via the store. */
  onSubmit(): void {
    if (this.characterForm.invalid) return;

    const formValue = this.characterForm.value;

    const character: Character = {
      id: crypto.randomUUID(),
      name: (formValue.name ?? '') as string,
      description: (formValue.description ?? '') as string,
      type: (formValue.type ?? 'player') as Character['type'],
      attributes: formValue.attributes as Record<string, number>,
      skills: [],
      inventory: [],
      quotes: [],
      createdAt: new Date(),
      updatedAt: new Date(),
    };

    this.store.set('characters', character);
    this.router.navigate(['/personagens', character.id]);
  }
}
