import { Component, Input, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import {
  FormArray,
  FormBuilder,
  FormGroup,
  ReactiveFormsModule,
  Validators,
} from '@angular/forms';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { StoreService } from '../../core/store/store.service';
import type { Character } from '../../core/models/character';

export interface SkillFormValue {
  name: string;
  bonus: number;
  attribute: string;
}

export interface InventoryFormValue {
  name: string;
  quantity: number;
  weight: number;
  description: string;
}

@Component({
  selector: 'app-character-sheet',
  standalone: true,
  imports: [
    CommonModule,
    ReactiveFormsModule,
    MatFormFieldModule,
    MatInputModule,
    MatSelectModule,
    MatButtonModule,
    MatIconModule,
  ],
  template: `
    <form [formGroup]="sheetForm" class="character-sheet" (ngSubmit)="onSave()">
      <!-- ═══════════════ Atributos ═══════════════ -->
      <section class="sheet-section">
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
              <span class="bonus-indicator">Bônus: {{ getBonus(attr) }}</span>
            </div>
          }
        </div>
      </section>

      <!-- ═══════════════ Perícias ═══════════════ -->
      <section class="sheet-section">
        <div class="section-header">
          <h2 class="section-title">Perícias</h2>
          <button
            mat-stroked-button
            type="button"
            size="small"
            (click)="addSkill()"
          >
            <mat-icon>add</mat-icon>
            Adicionar
          </button>
        </div>

        @if (skills.controls.length === 0) {
          <p class="empty-hint">Nenhuma perícia adicionada.</p>
        }

        <div formArrayName="skills" class="dynamic-list">
          @for (skill of skills.controls; track skill; let i = $index) {
            <div
              class="dynamic-row"
              [formGroupName]="i"
              [class.row-removing]="removingSkillIndex === i"
            >
              <mat-form-field appearance="outline" subscriptSizing="dynamic" class="flex-2">
                <mat-label>Perícia</mat-label>
                <input
                  matInput
                  formControlName="name"
                  list="common-skills"
                  placeholder="Nome da perícia"
                />
                @if (skill.get('name')?.hasError('required') && skill.get('name')?.touched) {
                  <mat-error>O nome é obrigatório</mat-error>
                }
              </mat-form-field>

              <datalist id="common-skills">
                @for (s of commonSkills; track s) {
                  <option [value]="s"></option>
                }
              </datalist>

              <mat-form-field appearance="outline" subscriptSizing="dynamic" class="flex-1">
                <mat-label>Bônus</mat-label>
                <input matInput type="number" formControlName="bonus" />
              </mat-form-field>

              <mat-form-field appearance="outline" subscriptSizing="dynamic" class="flex-1">
                <mat-label>Atributo</mat-label>
                <mat-select formControlName="attribute">
                  @for (attr of attributeKeys; track attr) {
                    <mat-option [value]="attr">{{ attr | uppercase }}</mat-option>
                  }
                </mat-select>
              </mat-form-field>

              <button
                mat-icon-button
                type="button"
                (click)="removeSkill(i)"
                class="remove-btn"
                aria-label="Remover perícia"
              >
                <mat-icon>delete</mat-icon>
              </button>
            </div>
          }
        </div>
      </section>

      <!-- ═══════════════ Inventário ═══════════════ -->
      <section class="sheet-section">
        <div class="section-header">
          <h2 class="section-title">Inventário</h2>
          <button
            mat-stroked-button
            type="button"
            size="small"
            (click)="addInventoryItem()"
          >
            <mat-icon>add</mat-icon>
            Adicionar
          </button>
        </div>

        @if (inventory.controls.length === 0) {
          <p class="empty-hint">Nenhum item no inventário.</p>
        }

        <div formArrayName="inventory" class="dynamic-list">
          @for (item of inventory.controls; track item; let i = $index) {
            <div
              class="dynamic-row"
              [formGroupName]="i"
              [class.row-removing]="removingInventoryIndex === i"
            >
              <mat-form-field appearance="outline" subscriptSizing="dynamic" class="flex-2">
                <mat-label>Item</mat-label>
                <input
                  matInput
                  formControlName="name"
                  placeholder="Nome do item"
                />
                @if (item.get('name')?.hasError('required') && item.get('name')?.touched) {
                  <mat-error>O nome é obrigatório</mat-error>
                }
              </mat-form-field>

              <mat-form-field appearance="outline" subscriptSizing="dynamic" class="flex-1">
                <mat-label>Qtd</mat-label>
                <input matInput type="number" min="1" formControlName="quantity" />
              </mat-form-field>

              <mat-form-field appearance="outline" subscriptSizing="dynamic" class="flex-1">
                <mat-label>Peso</mat-label>
                <input matInput type="number" min="0" step="0.1" formControlName="weight" />
              </mat-form-field>

              <mat-form-field appearance="outline" subscriptSizing="dynamic" class="flex-3">
                <mat-label>Descrição</mat-label>
                <input matInput formControlName="description" placeholder="Descrição opcional" />
              </mat-form-field>

              <button
                mat-icon-button
                type="button"
                (click)="removeInventoryItem(i)"
                class="remove-btn"
                aria-label="Remover item"
              >
                <mat-icon>delete</mat-icon>
              </button>
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
          [disabled]="sheetForm.invalid"
        >
          <mat-icon>save</mat-icon>
          Salvar
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
      gap: 32px;
    }

    /* ── Section ──────────────────────────── */

    .sheet-section {
      background: rgba(255, 255, 255, 0.03);
      border: 1px solid rgba(255, 255, 255, 0.06);
      border-radius: 12px;
      padding: 20px 24px 24px;
    }

    .section-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      margin-bottom: 16px;
      flex-wrap: wrap;
      gap: 8px;
    }

    .section-title {
      margin: 0 0 16px;
      font-size: 1.15rem;
      font-weight: 600;
      letter-spacing: 0.3px;
      color: #c4b5fd;
    }

    .section-header .section-title {
      margin: 0;
    }

    .empty-hint {
      text-align: center;
      opacity: 0.5;
      font-size: 0.875rem;
      margin: 16px 0;
    }

    /* ── Attributes grid ──────────────────── */

    .attributes-grid {
      display: grid;
      grid-template-columns: repeat(3, 1fr);
      gap: 16px;
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

    .bonus-indicator {
      font-size: 0.8rem;
      font-weight: 500;
      color: #a78bfa;
      letter-spacing: 0.5px;
    }

    /* ── Dynamic list ─────────────────────── */

    .dynamic-list {
      display: flex;
      flex-direction: column;
      gap: 12px;
    }

    .dynamic-row {
      display: flex;
      align-items: flex-start;
      gap: 8px;
      flex-wrap: nowrap;
    }

    .dynamic-row mat-form-field {
      flex-shrink: 0;
    }

    .flex-1 { flex: 1 1 80px; min-width: 70px; }
    .flex-2 { flex: 2 1 160px; min-width: 120px; }
    .flex-3 { flex: 3 1 200px; min-width: 140px; }

    .remove-btn {
      margin-top: 4px;
      flex-shrink: 0;
    }

    .row-removing {
      opacity: 0.4;
      pointer-events: none;
    }

    /* ── Actions ──────────────────────────── */

    .form-actions {
      display: flex;
      align-items: center;
      gap: 16px;
      padding: 8px 0;
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

  readonly attributeKeys = ['for', 'des', 'con', 'int', 'sab', 'car'] as const;

  readonly commonSkills = [
    'Acrobacia',
    'Atletismo',
    'Percepção',
    'Furtividade',
    'Intimidação',
    'Persuasão',
    'Investigação',
    'Medicina',
    'Sobrevivência',
    'Arcanismo',
    'História',
  ];

  sheetForm!: FormGroup;

  /** Index of item being removed (for visual feedback). */
  removingSkillIndex: number | null = null;
  removingInventoryIndex: number | null = null;

  /** Whether the form was just saved. */
  saved = false;

  get skills(): FormArray {
    return this.sheetForm.get('skills') as FormArray;
  }

  get inventory(): FormArray {
    return this.sheetForm.get('inventory') as FormArray;
  }

  ngOnInit(): void {
    this.initForm();
  }

  // ─── Form initialization ───────────────────────────────

  private initForm(): void {
    this.sheetForm = this.fb.group({
      attributes: this.fb.group({
        for: [
          this.character?.attributes?.['for'] ?? 10,
          [Validators.required, Validators.min(1), Validators.max(30)],
        ],
        des: [
          this.character?.attributes?.['des'] ?? 10,
          [Validators.required, Validators.min(1), Validators.max(30)],
        ],
        con: [
          this.character?.attributes?.['con'] ?? 10,
          [Validators.required, Validators.min(1), Validators.max(30)],
        ],
        int: [
          this.character?.attributes?.['int'] ?? 10,
          [Validators.required, Validators.min(1), Validators.max(30)],
        ],
        sab: [
          this.character?.attributes?.['sab'] ?? 10,
          [Validators.required, Validators.min(1), Validators.max(30)],
        ],
        car: [
          this.character?.attributes?.['car'] ?? 10,
          [Validators.required, Validators.min(1), Validators.max(30)],
        ],
      }),
      skills: this.fb.array(this.parseSkills()),
      inventory: this.fb.array(this.parseInventory()),
    });
  }

  private parseSkills(): FormGroup[] {
    if (!this.character?.skills?.length) return [];
    return this.character.skills.map((s: string) => {
      let parsed: SkillFormValue;
      try {
        parsed = JSON.parse(s) as SkillFormValue;
      } catch {
        parsed = { name: s, bonus: 0, attribute: 'for' };
      }
      return this.fb.group({
        name: [parsed.name ?? '', Validators.required],
        bonus: [parsed.bonus ?? 0],
        attribute: [parsed.attribute ?? 'for'],
      });
    });
  }

  private parseInventory(): FormGroup[] {
    if (!this.character?.inventory?.length) return [];
    return this.character.inventory.map((item: string) => {
      let parsed: InventoryFormValue;
      try {
        parsed = JSON.parse(item) as InventoryFormValue;
      } catch {
        parsed = { name: item, quantity: 1, weight: 0, description: '' };
      }
      return this.fb.group({
        name: [parsed.name ?? '', Validators.required],
        quantity: [parsed.quantity ?? 1],
        weight: [parsed.weight ?? 0],
        description: [parsed.description ?? ''],
      });
    });
  }

  // ─── Bonus computation ────────────────────────────────

  getBonus(attr: string): number {
    const value = this.sheetForm?.get('attributes')?.get(attr)?.value ?? 10;
    return Math.floor((value - 10) / 2);
  }

  // ─── Skills management ────────────────────────────────

  addSkill(): void {
    this.skills.push(
      this.fb.group({
        name: ['', Validators.required],
        bonus: [0],
        attribute: ['for'],
      }),
    );
  }

  removeSkill(index: number): void {
    this.removingSkillIndex = index;
    setTimeout(() => {
      this.skills.removeAt(index);
      this.removingSkillIndex = null;
    }, 200);
  }

  // ─── Inventory management ─────────────────────────────

  addInventoryItem(): void {
    this.inventory.push(
      this.fb.group({
        name: ['', Validators.required],
        quantity: [1],
        weight: [0],
        description: [''],
      }),
    );
  }

  removeInventoryItem(index: number): void {
    this.removingInventoryIndex = index;
    setTimeout(() => {
      this.inventory.removeAt(index);
      this.removingInventoryIndex = null;
    }, 200);
  }

  // ─── Save ─────────────────────────────────────────────

  onSave(): void {
    if (this.sheetForm.invalid || !this.character) return;

    const formValue = this.sheetForm.value;

    const updated: Character = {
      ...this.character,
      attributes: formValue.attributes as Record<string, number>,
      skills: (formValue.skills as SkillFormValue[]).map((s) => JSON.stringify(s)),
      inventory: (formValue.inventory as InventoryFormValue[]).map((item) =>
        JSON.stringify(item),
      ),
      updatedAt: new Date(),
    };

    this.store.update('characters', updated.id, updated);
    this.saved = true;
    setTimeout(() => (this.saved = false), 2000);
  }
}
