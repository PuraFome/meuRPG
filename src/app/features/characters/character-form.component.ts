import { Component, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import {
  FormArray,
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
import type { Character, DndSheet } from '../../core/models/character';
import type { BreadcrumbItem } from '../../shared/components/page-header.component';

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

      @if (isFullSheet) {
        <!-- ═══════════════ Identidade ═══════════════ -->
        <section class="form-section">
          <h2 class="section-title">Identidade</h2>
          <div class="identity-grid" formGroupName="identity">
            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>Raça</mat-label>
              <input matInput formControlName="race" placeholder="Ex.: Humano" />
            </mat-form-field>

            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>Classe</mat-label>
              <input matInput formControlName="class" placeholder="Ex.: Guerreiro" />
            </mat-form-field>

            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>Nível</mat-label>
              <input matInput type="number" min="1" max="20" formControlName="level" />
            </mat-form-field>

            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>Antecedente</mat-label>
              <input matInput formControlName="background" placeholder="Ex.: Soldado" />
            </mat-form-field>

            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>Alinhamento</mat-label>
              <input matInput formControlName="alignment" placeholder="Ex.: Leal e Bom" />
            </mat-form-field>

            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>XP</mat-label>
              <input matInput type="number" min="0" formControlName="xp" />
            </mat-form-field>
          </div>
        </section>
      }

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

      @if (isFullSheet) {
        <!-- ═══════════════ Combate ═══════════════ -->
        <section class="form-section">
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

            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>Dados de Vida</mat-label>
              <input matInput formControlName="hitDice" placeholder="Ex.: 1d10" />
            </mat-form-field>

            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>Bônus de Proficiência</mat-label>
              <input matInput type="number" min="1" formControlName="proficiencyBonus" />
            </mat-form-field>
          </div>
        </section>

        <!-- ═══════════════ Perícias ═══════════════ -->
        <section class="form-section">
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

          @if (skills.length === 0) {
            <p class="empty-hint">Nenhuma perícia adicionada.</p>
          }

          <div formArrayName="skills" class="dynamic-list">
            @for (skill of skills.controls; track skill; let i = $index) {
              <div class="dynamic-row" [formGroupName]="i">
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
        <section class="form-section">
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

          @if (inventory.length === 0) {
            <p class="empty-hint">Nenhum item no inventário.</p>
          }

          <div formArrayName="inventory" class="dynamic-list">
            @for (item of inventory.controls; track item; let i = $index) {
              <div class="dynamic-row" [formGroupName]="i">
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

        <!-- ═══════════════ Proficiências e Idiomas ═══════════════ -->
        <section class="form-section">
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
        <section class="form-section">
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

  readonly breadcrumbs: BreadcrumbItem[] = [
    { label: 'Personagens', route: '/personagens' },
    { label: 'Novo Personagem' },
  ];

  characterForm!: FormGroup;

  /** True when the selected type has a full D&D sheet (Jogador/Boss). */
  get isFullSheet(): boolean {
    return (
      this.characterForm?.get('type')?.value === 'player' ||
      this.characterForm?.get('type')?.value === 'boss'
    );
  }

  get skills(): FormArray {
    return this.characterForm.get('skills') as FormArray;
  }

  get inventory(): FormArray {
    return this.characterForm.get('inventory') as FormArray;
  }

  get proficiencies(): FormArray {
    return this.characterForm.get('proficiencies') as FormArray;
  }

  get languages(): FormArray {
    return this.characterForm.get('languages') as FormArray;
  }

  get features(): FormArray {
    return this.characterForm.get('features') as FormArray;
  }

  ngOnInit(): void {
    this.characterForm = this.fb.group({
      name: ['', Validators.required],
      type: ['player'],
      description: [''],
      identity: this.fb.group({
        race: [''],
        class: [''],
        level: [1, [Validators.required, Validators.min(1), Validators.max(20)]],
        background: [''],
        alignment: [''],
        xp: [0, [Validators.min(0)]],
      }),
      attributes: this.fb.group({
        for: [10, [Validators.required, Validators.min(1), Validators.max(30)]],
        des: [10, [Validators.required, Validators.min(1), Validators.max(30)]],
        con: [10, [Validators.required, Validators.min(1), Validators.max(30)]],
        int: [10, [Validators.required, Validators.min(1), Validators.max(30)]],
        sab: [10, [Validators.required, Validators.min(1), Validators.max(30)]],
        car: [10, [Validators.required, Validators.min(1), Validators.max(30)]],
      }),
      combat: this.fb.group({
        hpMax: [10, [Validators.required, Validators.min(1)]],
        hpCurrent: [10, [Validators.min(0)]],
        hpTemp: [0, [Validators.min(0)]],
        armorClass: [10, [Validators.min(0)]],
        initiative: [0],
        speed: [30, [Validators.min(0)]],
        hitDice: ['1d10'],
        proficiencyBonus: [2, [Validators.required, Validators.min(1)]],
      }),
      skills: this.fb.array([]),
      inventory: this.fb.array([]),
      proficiencies: this.fb.array([]),
      languages: this.fb.array([]),
      features: this.fb.array([]),
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
      skills: (formValue.skills as SkillFormValue[]).map((s) => JSON.stringify(s)),
      inventory: (formValue.inventory as InventoryFormValue[]).map((item) =>
        JSON.stringify(item),
      ),
      quotes: [],
      createdAt: new Date(),
      updatedAt: new Date(),
    };

    if (this.isFullSheet) {
      const identity = formValue.identity;
      const combat = formValue.combat;
      character.sheet = {
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

    this.store.set('characters', character);
    this.router.navigate(['/personagens', character.id]);
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
    this.skills.removeAt(index);
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
    this.inventory.removeAt(index);
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
}
