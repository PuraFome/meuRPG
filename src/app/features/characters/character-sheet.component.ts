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
import { DndOptionSelectComponent } from './dnd-option-select.component';
import type { Character, DndSheet } from '../../core/models/character';

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
    DndOptionSelectComponent,
  ],
  template: `
    <form [formGroup]="sheetForm" class="character-sheet" (ngSubmit)="onSave()">
      @if (isMinion) {
        <!-- ═══════════════ Combate do Minion ═══════════════ -->
        <section class="sheet-section">
          <h2 class="section-title">Combate do Minion</h2>
          <div class="combat-grid" formGroupName="minion">
            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>PV</mat-label>
              <input matInput type="number" min="1" formControlName="hp" />
            </mat-form-field>

            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>Ataque</mat-label>
              <input matInput type="number" min="1" formControlName="attack" />
            </mat-form-field>
          </div>
        </section>
      }
      @if (!isMinion) {
        @if (isFullSheet) {
          <!-- ═══════════════ Identidade ═══════════════ -->
          <section class="sheet-section">
          <h2 class="section-title">Identidade</h2>
          <div class="identity-grid" formGroupName="identity">
            <app-dnd-option-select
              formControlName="race"
              category="race"
              label="Raça"
            />

            <app-dnd-option-select
              formControlName="class"
              category="class"
              label="Classe"
            />

            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>Nível</mat-label>
              <input matInput type="number" min="1" max="20" formControlName="level" />
            </mat-form-field>

            <app-dnd-option-select
              formControlName="background"
              category="background"
              label="Antecedente"
            />

            <app-dnd-option-select
              formControlName="alignment"
              category="alignment"
              label="Alinhamento"
            />

            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>XP</mat-label>
              <input matInput type="number" min="0" formControlName="xp" />
            </mat-form-field>
          </div>
        </section>

        <!-- ═══════════════ Combate ═══════════════ -->
        <section class="sheet-section">
          <h2 class="section-title">Combate</h2>
          <div class="combat-grid" formGroupName="combat">
            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>PV Máximo</mat-label>
              <input matInput type="number" min="1" formControlName="hpMax" />
            </mat-form-field>

            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>PV Atuais</mat-label>
              <input matInput type="number" min="0" formControlName="hpCurrent" />
            </mat-form-field>

            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>PV Temporários</mat-label>
              <input matInput type="number" min="0" formControlName="hpTemp" />
            </mat-form-field>

            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>CA</mat-label>
              <input matInput type="number" min="0" formControlName="armorClass" />
            </mat-form-field>

            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>Iniciativa</mat-label>
              <input matInput type="number" formControlName="initiative" />
            </mat-form-field>

            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>Deslocamento</mat-label>
              <input matInput type="number" min="0" formControlName="speed" />
            </mat-form-field>

            <app-dnd-option-select
              formControlName="hitDice"
              category="hitDice"
              label="Dados de Vida"
            />

            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>Bônus de Proficiência</mat-label>
              <input matInput type="number" min="1" formControlName="proficiencyBonus" />
            </mat-form-field>
          </div>
        </section>
      }

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

      @if (isFullSheet) {
        <!-- ═══════════════ Proficiências e Idiomas ═══════════════ -->
        <section class="sheet-section">
          <div class="section-header">
            <h2 class="section-title">Proficiências e Idiomas</h2>
            <div class="section-actions">
              <button
                mat-stroked-button
                type="button"
                size="small"
                (click)="addProficiency()"
              >
                <mat-icon>add</mat-icon>
                Proficiência
              </button>
              <button
                mat-stroked-button
                type="button"
                size="small"
                (click)="addLanguage()"
              >
                <mat-icon>add</mat-icon>
                Idioma
              </button>
            </div>
          </div>

          <div class="dual-list">
            <div>
              <h3 class="sub-title">Proficiências</h3>
              @if (proficiencies.length === 0) {
                <p class="empty-hint">Nenhuma proficiência.</p>
              }
              <div formArrayName="proficiencies" class="dynamic-list">
                @for (proficiency of proficiencies.controls; track proficiency; let i = $index) {
                  <div class="simple-row">
                    <mat-form-field appearance="outline" subscriptSizing="dynamic" class="flex-2">
                      <mat-label>Proficiência</mat-label>
                      <input matInput [formControlName]="i" placeholder="Ex.: Armaduras pesadas" />
                    </mat-form-field>
                    <button
                      mat-icon-button
                      type="button"
                      (click)="removeProficiency(i)"
                      class="remove-btn"
                      aria-label="Remover proficiência"
                    >
                      <mat-icon>delete</mat-icon>
                    </button>
                  </div>
                }
              </div>
            </div>

            <div>
              <h3 class="sub-title">Idiomas</h3>
              @if (languages.length === 0) {
                <p class="empty-hint">Nenhum idioma.</p>
              }
              <div formArrayName="languages" class="dynamic-list">
                @for (language of languages.controls; track language; let i = $index) {
                  <div class="simple-row">
                    <mat-form-field appearance="outline" subscriptSizing="dynamic" class="flex-2">
                      <mat-label>Idioma</mat-label>
                      <input matInput [formControlName]="i" placeholder="Ex.: Comum, Élfico" />
                    </mat-form-field>
                    <button
                      mat-icon-button
                      type="button"
                      (click)="removeLanguage(i)"
                      class="remove-btn"
                      aria-label="Remover idioma"
                    >
                      <mat-icon>delete</mat-icon>
                    </button>
                  </div>
                }
              </div>
            </div>
          </div>
        </section>

        <!-- ═══════════════ Traços e Características ═══════════════ -->
        <section class="sheet-section">
          <div class="section-header">
            <h2 class="section-title">Traços e Características</h2>
            <button
              mat-stroked-button
              type="button"
              size="small"
              (click)="addFeature()"
            >
              <mat-icon>add</mat-icon>
              Adicionar
            </button>
          </div>

          @if (features.length === 0) {
            <p class="empty-hint">Nenhum traço adicionado.</p>
          }

          <div formArrayName="features" class="dynamic-list">
            @for (feature of features.controls; track feature; let i = $index) {
              <div class="simple-row">
                <mat-form-field appearance="outline" subscriptSizing="dynamic" class="flex-2">
                  <mat-label>Traço</mat-label>
                  <input matInput [formControlName]="i" placeholder="Ex.: Ataque Extra" />
                </mat-form-field>
                <button
                  mat-icon-button
                  type="button"
                  (click)="removeFeature(i)"
                  class="remove-btn"
                  aria-label="Remover traço"
                >
                  <mat-icon>delete</mat-icon>
                </button>
              </div>
            }
          </div>
        </section>
      }
      }

      <!-- ═══════════════ Ações ═══════════════ -->
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

    .section-actions {
      display: flex;
      gap: 8px;
      flex-wrap: wrap;
    }

    .sub-title {
      margin: 0 0 12px;
      font-size: 0.95rem;
      font-weight: 600;
      opacity: 0.8;
    }

    .empty-hint {
      text-align: center;
      opacity: 0.5;
      font-size: 0.875rem;
      margin: 16px 0;
    }

    /* ── Identity / combat grids ──────────── */

    .identity-grid {
      display: grid;
      grid-template-columns: repeat(2, 1fr);
      gap: 16px;
    }

    .combat-grid {
      display: grid;
      grid-template-columns: repeat(4, 1fr);
      gap: 16px;
    }

    @media (max-width: 720px) {
      .combat-grid {
        grid-template-columns: repeat(2, 1fr);
      }
    }

    @media (max-width: 480px) {
      .identity-grid,
      .combat-grid {
        grid-template-columns: 1fr;
      }
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
      flex-wrap: wrap;
    }

    .dynamic-row mat-form-field {
      flex-shrink: 0;
    }

    .flex-1 { flex: 1 1 80px; min-width: 60px; }
    .flex-2 { flex: 2 1 160px; min-width: 120px; }
    .flex-3 { flex: 3 1 200px; min-width: 140px; }

    .simple-row {
      display: flex;
      align-items: flex-start;
      gap: 8px;
    }

    .remove-btn {
      margin-top: 4px;
      flex-shrink: 0;
    }

    .row-removing {
      opacity: 0.4;
      pointer-events: none;
    }

    .dual-list {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 24px;
    }

    @media (max-width: 640px) {
      .dual-list {
        grid-template-columns: 1fr;
        gap: 16px;
      }
    }

    /* ── Actions ──────────────────────────── */

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

  /** True while a save is in progress (prevents double-submit). */
  saving = false;

  /** Human-readable message of the last save failure, or null when no error. */
  saveError: string | null = null;

  /** Full D&D sheet applies to Jogador and Boss types only. */
  get isFullSheet(): boolean {
    return this.character?.type === 'player' || this.character?.type === 'boss';
  }

  get isMinion(): boolean {
    return this.character?.type === 'minion';
  }

  get skills(): FormArray {
    return this.sheetForm.get('skills') as FormArray;
  }

  get inventory(): FormArray {
    return this.sheetForm.get('inventory') as FormArray;
  }

  get proficiencies(): FormArray {
    return this.sheetForm.get('proficiencies') as FormArray;
  }

  get languages(): FormArray {
    return this.sheetForm.get('languages') as FormArray;
  }

  get features(): FormArray {
    return this.sheetForm.get('features') as FormArray;
  }

  ngOnInit(): void {
    this.initForm();
  }

  // ─── Form initialization ───────────────────────────────

  private initForm(): void {
    const sheet = this.character?.sheet;
    this.sheetForm = this.fb.group({
      identity: this.fb.group({
        race: [sheet?.race ?? ''],
        class: [sheet?.class ?? ''],
        level: [sheet?.level ?? 1, [Validators.required, Validators.min(1), Validators.max(20)]],
        background: [sheet?.background ?? ''],
        alignment: [sheet?.alignment ?? ''],
        xp: [sheet?.xp ?? 0, [Validators.min(0)]],
      }),
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
      combat: this.fb.group({
        hpMax: [sheet?.hpMax ?? 10, [Validators.required, Validators.min(1)]],
        hpCurrent: [sheet?.hpCurrent ?? 10, [Validators.min(0)]],
        hpTemp: [sheet?.hpTemp ?? 0, [Validators.min(0)]],
        armorClass: [sheet?.armorClass ?? 10, [Validators.min(0)]],
        initiative: [sheet?.initiative ?? 0],
        speed: [sheet?.speed ?? 30, [Validators.min(0)]],
        hitDice: [sheet?.hitDice ?? '1d10'],
        proficiencyBonus: [sheet?.proficiencyBonus ?? 2, [Validators.required, Validators.min(1)]],
      }),
      skills: this.fb.array(this.parseSkills()),
      inventory: this.fb.array(this.parseInventory()),
      proficiencies: this.fb.array(sheet?.proficiencies ?? []),
      languages: this.fb.array(sheet?.languages ?? []),
      features: this.fb.array(sheet?.features ?? []),
      minion: this.fb.group({
        hp: [this.character?.minion?.hp ?? 10, [Validators.required, Validators.min(1)]],
        attack: [this.character?.minion?.attack ?? 3, [Validators.required, Validators.min(1)]],
      }),
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

  // ─── Proficiencies / languages ────────────────────────

  addProficiency(): void {
    this.proficiencies.push(this.fb.control(''));
  }

  removeProficiency(index: number): void {
    this.proficiencies.removeAt(index);
  }

  addLanguage(): void {
    this.languages.push(this.fb.control(''));
  }

  removeLanguage(index: number): void {
    this.languages.removeAt(index);
  }

  // ─── Features ─────────────────────────────────────────

  addFeature(): void {
    this.features.push(this.fb.control(''));
  }

  removeFeature(index: number): void {
    this.features.removeAt(index);
  }

  // ─── Save ─────────────────────────────────────────────

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
    const formValue = this.sheetForm.value;

    const updated: Character = {
      ...character,
      attributes: formValue.attributes as Record<string, number>,
      skills: (formValue.skills as SkillFormValue[]).map((s) => JSON.stringify(s)),
      inventory: (formValue.inventory as InventoryFormValue[]).map((item) =>
        JSON.stringify(item),
      ),
      updatedAt: new Date(),
    };

    if (this.isFullSheet) {
      const identity = formValue.identity;
      const combat = formValue.combat;
      updated.sheet = {
        race: identity.race ?? '',
        class: identity.class ?? '',
        level: identity.level ?? 1,
        background: identity.background ?? '',
        alignment: identity.alignment ?? '',
        xp: identity.xp ?? 0,
        hpMax: combat.hpMax ?? 10,
        hpCurrent: combat.hpCurrent ?? 10,
        hpTemp: combat.hpTemp ?? 0,
        armorClass: combat.armorClass ?? 10,
        initiative: combat.initiative ?? 0,
        speed: combat.speed ?? 30,
        hitDice: combat.hitDice ?? '1d10',
        proficiencyBonus: combat.proficiencyBonus ?? 2,
        proficiencies: (formValue.proficiencies as string[]).filter(Boolean),
        languages: (formValue.languages as string[]).filter(Boolean),
        features: (formValue.features as string[]).filter(Boolean),
      } satisfies DndSheet;
    }

    if (this.isMinion) {
      const minion = formValue.minion;
      updated.minion = {
        hp: minion.hp ?? 10,
        attack: minion.attack ?? 3,
      };
      updated.attributes = {};
      updated.skills = [];
      updated.inventory = [];
    }

    return updated;
  }
}
