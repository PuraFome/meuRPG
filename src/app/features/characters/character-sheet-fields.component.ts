import { Component, Input, OnDestroy, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormArray, FormBuilder, FormControl, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatStepperModule } from '@angular/material/stepper';
import { MatTooltipModule } from '@angular/material/tooltip';
import { Subject } from 'rxjs';
import { distinctUntilChanged, takeUntil } from 'rxjs/operators';
import {
  abilityModifier,
  effectiveSkillBonus,
  formatModifier,
  passivePerception,
  proficiencyBonusForLevel,
  savingThrowBonus,
  spellAttackBonus as computeSpellAttackBonus,
  spellSaveDc as computeSpellSaveDc,
} from './dnd-calculations';
import {
  ABILITIES,
  ATTRIBUTE_KEYS,
  DND_ARMOR_PROFICIENCIES,
  DND_CONDITIONS,
  DND_LANGUAGES,
  DND_SKILLS,
  DND_TOOLS,
  DND_WEAPON_PROFICIENCIES,
  findClass,
  raceSpeed,
} from './dnd-data';
import { DndOptionSelectComponent } from './dnd-option-select.component';
import { rollAttributeSet } from './dice-roll.util';
import type { AttributeKey, DerivedField, SkillEntry } from '../../core/models/character';

/**
 * Renders the complete D&D 5e sheet as a guided stepper. Shared by the creation
 * page and the character detail "Ficha" tab, so both stay in lockstep and no
 * field is ever silently dropped on save.
 */
@Component({
  selector: 'app-character-sheet-fields',
  standalone: true,
  imports: [
    CommonModule,
    ReactiveFormsModule,
    MatButtonModule,
    MatCheckboxModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatSelectModule,
    MatStepperModule,
    MatTooltipModule,
    DndOptionSelectComponent,
  ],
  template: `
    @if (isMinion) {
      <section class="sheet-section">
        <h2 class="section-title">Combate do Minion</h2>
        <div class="combat-grid" [formGroup]="minionGroup">
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
    } @else if (isFullSheet) {
      <mat-stepper class="sheet-stepper" linear="false" orientation="vertical">
        <!-- ═══════════════ 1. Identidade ═══════════════ -->
        <mat-step label="Identidade">
          <p class="step-hint">
            Quem é o personagem: raça, classe e antecedente definem sugestões automáticas
            (dados de vida, resistências e conjuração).
          </p>
          <div class="identity-grid" [formGroup]="identityGroup">
            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>Nome do Jogador</mat-label>
              <input matInput formControlName="playerName" placeholder="Quem interpreta" />
            </mat-form-field>

            <app-dnd-option-select formControlName="race" category="race" label="Raça" />

            <app-dnd-option-select formControlName="class" category="class" label="Classe" />

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

            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>Inspiração</mat-label>
              <input matInput type="number" min="0" formControlName="inspiration" />
            </mat-form-field>
          </div>
        </mat-step>

        <!-- ═══════════════ 2. Atributos ═══════════════ -->
        <mat-step label="Atributos">
          <div class="section-header">
            <p class="step-hint">
              Role 4d6 (descarta o menor) e distribua, ou digite os valores. O modificador é
              calculado automaticamente.
            </p>
            <div class="section-actions">
              <button mat-stroked-button type="button" (click)="onRollAttributes()">
                <mat-icon>casino</mat-icon>
                Rolar Atributos (4d6)
              </button>
              @if (hasRoll) {
                <button mat-button type="button" (click)="onClearRoll()">
                  <mat-icon>backspace</mat-icon>
                  Limpar
                </button>
              }
            </div>
          </div>

          @if (hasRoll) {
            <p class="roll-pool">
              <span class="roll-pool-label">
                {{ rolledPool.length ? 'Valores disponíveis' : 'Todos os valores distribuídos' }}
              </span>
              @for (value of rolledPool; track $index) {
                <span class="roll-chip">{{ value }}</span>
              }
            </p>
          }

          <div class="attributes-grid" [formGroup]="attributesGroup">
            @for (attr of attributeKeys; track attr) {
              <div class="attribute-field">
                <mat-form-field appearance="outline" subscriptSizing="dynamic">
                  <mat-label>{{ abilityShort(attr) }}</mat-label>
                  <input matInput type="number" min="1" max="30" [formControlName]="attr" />
                </mat-form-field>
                <span class="bonus-indicator">Mod: {{ formatModifier(modifier(attr)) }}</span>

                @if (hasRoll) {
                  <mat-form-field
                    appearance="outline"
                    subscriptSizing="dynamic"
                    class="assign-field"
                  >
                    <mat-label>Distribuir</mat-label>
                    <mat-select
                      [value]="assigned[attr]"
                      (selectionChange)="onAssign(attr, $event.value)"
                    >
                      @for (option of optionsFor(attr); track $index) {
                        <mat-option [value]="option.value" [disabled]="option.disabled">
                          {{ option.value }}
                        </mat-option>
                      }
                    </mat-select>
                  </mat-form-field>
                }
              </div>
            }
          </div>

          <h3 class="sub-title">Testes de Resistência</h3>
          <div class="saves-grid" [formGroup]="savingThrowsGroup">
            @for (attr of attributeKeys; track attr) {
              <div class="save-item">
                <mat-checkbox [formControlName]="attr">{{ abilityShort(attr) }}</mat-checkbox>
                <span class="save-bonus">{{ formatModifier(savingBonus(attr)) }}</span>
              </div>
            }
          </div>
        </mat-step>

        <!-- ═══════════════ 3. Perícias ═══════════════ -->
        <mat-step label="Perícias">
          <div class="section-header">
            <p class="step-hint">
              Marque proficiência (e especialização) — o bônus total é calculado com o
              modificador e o bônus de proficiência.
            </p>
            <button mat-stroked-button type="button" size="small" (click)="addSkill()">
              <mat-icon>add</mat-icon>
              Perícia extra
            </button>
          </div>

          <table class="skills-table">
            <thead>
              <tr>
                <th class="col-name">Perícia</th>
                <th class="col-ability">Atributo</th>
                <th class="col-check">Prof.</th>
                <th class="col-check">Espec.</th>
                <th class="col-bonus">Bônus</th>
                <th class="col-action"></th>
              </tr>
            </thead>
            <tbody>
              @for (skill of skills.controls; track skill; let i = $index) {
                <tr [formGroup]="asGroup(skill)">
                  <td class="col-name">
                    <input
                      matInput
                      formControlName="name"
                      list="sheet-skills"
                      class="inline-input"
                    />
                  </td>
                  <td class="col-ability">{{ abilityShort(skill.get('ability')?.value) }}</td>
                  <td class="col-check">
                    <mat-checkbox formControlName="proficient"></mat-checkbox>
                  </td>
                  <td class="col-check">
                    <mat-checkbox formControlName="expertise"></mat-checkbox>
                  </td>
                  <td class="col-bonus">
                    @if (skill.get('bonusOverride')?.value === null) {
                      <span class="computed-bonus">{{ formatModifier(skillBonus(i)) }}</span>
                    } @else {
                      <input
                        matInput
                        type="number"
                        formControlName="bonusOverride"
                        class="inline-input bonus-input"
                      />
                    }
                  </td>
                  <td class="col-action">
                    <button
                      mat-icon-button
                      type="button"
                      class="mini-btn"
                      [matTooltip]="
                        skill.get('bonusOverride')?.value === null
                          ? 'Sobrescrever bônus'
                          : 'Voltar ao automático'
                      "
                      (click)="toggleSkillOverride(i)"
                    >
                      <mat-icon>{{
                        skill.get('bonusOverride')?.value === null ? 'edit' : 'auto_awesome'
                      }}</mat-icon>
                    </button>
                    <button
                      mat-icon-button
                      type="button"
                      class="mini-btn"
                      aria-label="Remover perícia"
                      (click)="removeSkill(i)"
                    >
                      <mat-icon>delete</mat-icon>
                    </button>
                  </td>
                </tr>
              }
            </tbody>
          </table>

          <datalist id="sheet-skills">
            @for (s of skillNames; track s) {
              <option [value]="s"></option>
            }
          </datalist>

          <p class="derived-line">
            Percepção Passiva: <strong>{{ formatModifier(passivePerceptionValue()) }}</strong>
            <span class="muted">(10 + bônus de Percepção)</span>
          </p>
        </mat-step>

        <!-- ═══════════════ 4. Combate ═══════════════ -->
        <mat-step label="Combate">
          <p class="step-hint">
            Os valores com o ícone ✨ são automáticos; clique para sobrescrever manualmente.
          </p>
          <div class="combat-grid" [formGroup]="combatGroup">
            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>PV Máximo</mat-label>
              <input matInput type="number" min="0" formControlName="hpMax" />
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

            <mat-form-field appearance="outline" subscriptSizing="dynamic" class="auto-field">
              <mat-label>Iniciativa</mat-label>
              <input
                matInput
                type="number"
                formControlName="initiative"
                [readonly]="!isOverridden('initiative')"
              />
              <button
                matIconSuffix
                type="button"
                mat-icon-button
                [matTooltip]="isOverridden('initiative') ? 'Voltar ao automático' : 'Sobrescrever'"
                (click)="toggleOverride('initiative')"
              >
                <mat-icon>{{ isOverridden('initiative') ? 'edit' : 'auto_awesome' }}</mat-icon>
              </button>
            </mat-form-field>

            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>Deslocamento (pés)</mat-label>
              <input matInput type="number" min="0" formControlName="speed" />
            </mat-form-field>

            <app-dnd-option-select
              formControlName="hitDice"
              category="hitDice"
              label="Dados de Vida"
            />

            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>Dados de Vida (total)</mat-label>
              <input matInput type="number" min="0" formControlName="hitDiceTotal" />
            </mat-form-field>

            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>Dados de Vida (gastos)</mat-label>
              <input matInput type="number" min="0" formControlName="hitDiceSpent" />
            </mat-form-field>

            <mat-form-field appearance="outline" subscriptSizing="dynamic" class="auto-field">
              <mat-label>Bônus de Proficiência</mat-label>
              <input
                matInput
                type="number"
                min="1"
                formControlName="proficiencyBonus"
                [readonly]="!isOverridden('proficiencyBonus')"
              />
              <button
                matIconSuffix
                type="button"
                mat-icon-button
                [matTooltip]="
                  isOverridden('proficiencyBonus') ? 'Voltar ao automático' : 'Sobrescrever'
                "
                (click)="toggleOverride('proficiencyBonus')"
              >
                <mat-icon>{{ isOverridden('proficiencyBonus') ? 'edit' : 'auto_awesome' }}</mat-icon>
              </button>
            </mat-form-field>

            <mat-form-field appearance="outline" subscriptSizing="dynamic" class="auto-field">
              <mat-label>Percepção Passiva</mat-label>
              <input
                matInput
                type="number"
                min="0"
                formControlName="passivePerception"
                [readonly]="!isOverridden('passivePerception')"
              />
              <button
                matIconSuffix
                type="button"
                mat-icon-button
                [matTooltip]="
                  isOverridden('passivePerception') ? 'Voltar ao automático' : 'Sobrescrever'
                "
                (click)="toggleOverride('passivePerception')"
              >
                <mat-icon>{{
                  isOverridden('passivePerception') ? 'edit' : 'auto_awesome'
                }}</mat-icon>
              </button>
            </mat-form-field>
          </div>

          <h3 class="sub-title">Testes de Morte</h3>
          <div class="death-saves" [formGroup]="combatGroup">
            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>Sucessos (0-3)</mat-label>
              <input matInput type="number" min="0" max="3" formControlName="deathSaveSuccesses" />
            </mat-form-field>
            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>Falhas (0-3)</mat-label>
              <input matInput type="number" min="0" max="3" formControlName="deathSaveFailures" />
            </mat-form-field>
          </div>

          <h3 class="sub-title">Condições</h3>
          <mat-form-field
            appearance="outline"
            subscriptSizing="dynamic"
            class="full-width"
            [formGroup]="combatGroup"
          >
            <mat-label>Condições ativas</mat-label>
            <mat-select formControlName="conditions" multiple>
              @for (condition of conditions; track condition) {
                <mat-option [value]="condition">{{ condition }}</mat-option>
              }
            </mat-select>
          </mat-form-field>

          <div class="section-header">
            <h3 class="sub-title">Ataques</h3>
            <button mat-stroked-button type="button" size="small" (click)="addAttack()">
              <mat-icon>add</mat-icon>
              Ataque
            </button>
          </div>
          @if (attacks.length === 0) {
            <p class="empty-hint">Nenhum ataque cadastrado.</p>
          }
          <div class="dynamic-list">
            @for (attack of attacks.controls; track attack; let i = $index) {
              <div class="dynamic-row" [formGroup]="asGroup(attack)">
                <mat-form-field appearance="outline" subscriptSizing="dynamic" class="flex-2">
                  <mat-label>Ataque (nome)</mat-label>
                  <input matInput formControlName="name" placeholder="Espada Longa" />
                </mat-form-field>
                <mat-form-field appearance="outline" subscriptSizing="dynamic" class="flex-1">
                  <mat-label>Ataque</mat-label>
                  <input matInput formControlName="attackBonus" placeholder="+5" />
                </mat-form-field>
                <mat-form-field appearance="outline" subscriptSizing="dynamic" class="flex-1">
                  <mat-label>Dano</mat-label>
                  <input matInput formControlName="damage" placeholder="1d8+3" />
                </mat-form-field>
                <mat-form-field appearance="outline" subscriptSizing="dynamic" class="flex-1">
                  <mat-label>Tipo</mat-label>
                  <input matInput formControlName="damageType" placeholder="Cortante" />
                </mat-form-field>
                <button
                  mat-icon-button
                  type="button"
                  class="remove-btn"
                  aria-label="Remover ataque"
                  (click)="removeAttack(i)"
                >
                  <mat-icon>delete</mat-icon>
                </button>
              </div>
            }
          </div>
        </mat-step>

        <!-- ═══════════════ 5. Magia ═══════════════ -->
        <mat-step label="Magia">
          <div class="section-header" [formGroup]="spellcastingGroup">
            <p class="step-hint">Conjuração é opcional — ative somente para conjuradores.</p>
            <mat-checkbox formControlName="active" #spellToggle>Conjura magias</mat-checkbox>
          </div>

          @if (spellToggle.checked) {
            <div class="combat-grid" [formGroup]="spellcastingGroup">
              <mat-form-field appearance="outline" subscriptSizing="dynamic">
                <mat-label>Atributo de Conjuração</mat-label>
                <mat-select formControlName="ability">
                  <mat-option value="">—</mat-option>
                  @for (attr of attributeKeys; track attr) {
                    <mat-option [value]="attr">{{ abilityShort(attr) }}</mat-option>
                  }
                </mat-select>
              </mat-form-field>

              <mat-form-field appearance="outline" subscriptSizing="dynamic" class="auto-field">
                <mat-label>CD de Magia</mat-label>
                <input
                  matInput
                  type="number"
                  min="0"
                  formControlName="saveDc"
                  [readonly]="!isOverridden('spellSaveDc')"
                />
                <button
                  matIconSuffix
                  type="button"
                  mat-icon-button
                  [matTooltip]="
                    isOverridden('spellSaveDc') ? 'Voltar ao automático' : 'Sobrescrever'
                  "
                  (click)="toggleOverride('spellSaveDc')"
                >
                  <mat-icon>{{ isOverridden('spellSaveDc') ? 'edit' : 'auto_awesome' }}</mat-icon>
                </button>
              </mat-form-field>

              <mat-form-field appearance="outline" subscriptSizing="dynamic" class="auto-field">
                <mat-label>Ataque de Magia</mat-label>
                <input
                  matInput
                  type="number"
                  formControlName="attackBonus"
                  [readonly]="!isOverridden('spellAttackBonus')"
                />
                <button
                  matIconSuffix
                  type="button"
                  mat-icon-button
                  [matTooltip]="
                    isOverridden('spellAttackBonus') ? 'Voltar ao automático' : 'Sobrescrever'
                  "
                  (click)="toggleOverride('spellAttackBonus')"
                >
                  <mat-icon>{{
                    isOverridden('spellAttackBonus') ? 'edit' : 'auto_awesome'
                  }}</mat-icon>
                </button>
              </mat-form-field>
            </div>

            <div class="section-header">
              <h3 class="sub-title">Magias</h3>
              <button mat-stroked-button type="button" size="small" (click)="addSpell()">
                <mat-icon>add</mat-icon>
                Magia
              </button>
            </div>
            @if (spells.length === 0) {
              <p class="empty-hint">Nenhuma magia cadastrada.</p>
            }
            <div class="dynamic-list">
            @for (spell of spells.controls; track spell; let i = $index) {
              <div class="dynamic-row" [formGroup]="asGroup(spell)">
                  <mat-form-field appearance="outline" subscriptSizing="dynamic" class="flex-2">
                    <mat-label>Magia</mat-label>
                    <input matInput formControlName="name" placeholder="Bola de Fogo" />
                  </mat-form-field>
                  <mat-form-field appearance="outline" subscriptSizing="dynamic" class="flex-1">
                    <mat-label>Nível</mat-label>
                    <input matInput type="number" min="0" max="9" formControlName="level" />
                  </mat-form-field>
                  <div class="inline-check">
                    <mat-checkbox formControlName="prepared">Preparada</mat-checkbox>
                  </div>
                  <button
                    mat-icon-button
                    type="button"
                    class="remove-btn"
                    aria-label="Remover magia"
                    (click)="removeSpell(i)"
                  >
                    <mat-icon>delete</mat-icon>
                  </button>
                </div>
              }
            </div>

            <div class="section-header">
              <h3 class="sub-title">Espaços de Magia</h3>
              <button mat-stroked-button type="button" size="small" (click)="addSlot()">
                <mat-icon>add</mat-icon>
                Círculo
              </button>
            </div>
            @if (slots.length === 0) {
              <p class="empty-hint">Nenhum espaço cadastrado.</p>
            }
<div class="dynamic-list">
            @for (slot of slots.controls; track slot; let i = $index) {
              <div class="dynamic-row" [formGroup]="asGroup(slot)">
                  <mat-form-field appearance="outline" subscriptSizing="dynamic" class="flex-1">
                    <mat-label>Círculo</mat-label>
                    <input matInput type="number" min="1" max="9" formControlName="level" />
                  </mat-form-field>
                  <mat-form-field appearance="outline" subscriptSizing="dynamic" class="flex-1">
                    <mat-label>Total</mat-label>
                    <input matInput type="number" min="0" formControlName="total" />
                  </mat-form-field>
                  <mat-form-field appearance="outline" subscriptSizing="dynamic" class="flex-1">
                    <mat-label>Usados</mat-label>
                    <input matInput type="number" min="0" formControlName="used" />
                  </mat-form-field>
                  <button
                    mat-icon-button
                    type="button"
                    class="remove-btn"
                    aria-label="Remover espaço"
                    (click)="removeSlot(i)"
                  >
                    <mat-icon>delete</mat-icon>
                  </button>
                </div>
              }
            </div>
          }
        </mat-step>

        <!-- ═══════════════ 6. Equipamento ═══════════════ -->
        <mat-step label="Equipamento">
          <h3 class="sub-title">Moedas</h3>
          <div class="currency-grid" [formGroup]="currencyGroup">
            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>PC (cobre)</mat-label>
              <input matInput type="number" min="0" formControlName="pc" />
            </mat-form-field>
            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>PP (prata)</mat-label>
              <input matInput type="number" min="0" formControlName="pp" />
            </mat-form-field>
            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>PE (electrum)</mat-label>
              <input matInput type="number" min="0" formControlName="pe" />
            </mat-form-field>
            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>PO (ouro)</mat-label>
              <input matInput type="number" min="0" formControlName="po" />
            </mat-form-field>
            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>PL (platina)</mat-label>
              <input matInput type="number" min="0" formControlName="pl" />
            </mat-form-field>
          </div>

          <div class="section-header">
            <h3 class="sub-title">Inventário</h3>
            <button mat-stroked-button type="button" size="small" (click)="addInventoryItem()">
              <mat-icon>add</mat-icon>
              Item
            </button>
          </div>
          @if (inventory.length === 0) {
            <p class="empty-hint">Nenhum item no inventário.</p>
          }
          <div class="dynamic-list">
            @for (item of inventory.controls; track item; let i = $index) {
              <div class="dynamic-row" [formGroup]="asGroup(item)">
                <mat-form-field appearance="outline" subscriptSizing="dynamic" class="flex-2">
                  <mat-label>Item</mat-label>
                  <input matInput formControlName="name" placeholder="Nome do item" />
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
                  <input matInput formControlName="description" placeholder="Opcional" />
                </mat-form-field>
                <button
                  mat-icon-button
                  type="button"
                  class="remove-btn"
                  aria-label="Remover item"
                  (click)="removeInventoryItem(i)"
                >
                  <mat-icon>delete</mat-icon>
                </button>
              </div>
            }
          </div>
        </mat-step>

        <!-- ═══════════════ 7. Proficiências & Idiomas ═══════════════ -->
        <mat-step label="Proficiências">
          <div class="three-list">
            <div>
              <div class="section-header">
                <h3 class="sub-title">Proficiências</h3>
                <button mat-icon-button type="button" aria-label="Adicionar" (click)="addProficiency()">
                  <mat-icon>add</mat-icon>
                </button>
              </div>
              <div class="dynamic-list">
                @for (prof of proficiencies.controls; track prof; let i = $index) {
                  <div class="simple-row">
                    <input matInput [formControl]="asControl(prof)" list="proficiency-options" class="inline-input" />
                    <button mat-icon-button type="button" class="mini-btn" aria-label="Remover" (click)="removeProficiency(i)">
                      <mat-icon>delete</mat-icon>
                    </button>
                  </div>
                }
              </div>
            </div>

            <div>
              <div class="section-header">
                <h3 class="sub-title">Ferramentas</h3>
                <button mat-icon-button type="button" aria-label="Adicionar" (click)="addTool()">
                  <mat-icon>add</mat-icon>
                </button>
              </div>
              <div class="dynamic-list">
                @for (tool of tools.controls; track tool; let i = $index) {
                  <div class="simple-row">
                    <input matInput [formControl]="asControl(tool)" list="tool-options" class="inline-input" />
                    <button mat-icon-button type="button" class="mini-btn" aria-label="Remover" (click)="removeTool(i)">
                      <mat-icon>delete</mat-icon>
                    </button>
                  </div>
                }
              </div>
            </div>

            <div>
              <div class="section-header">
                <h3 class="sub-title">Idiomas</h3>
                <button mat-icon-button type="button" aria-label="Adicionar" (click)="addLanguage()">
                  <mat-icon>add</mat-icon>
                </button>
              </div>
              <div class="dynamic-list">
                @for (lang of languages.controls; track lang; let i = $index) {
                  <div class="simple-row">
                    <input matInput [formControl]="asControl(lang)" list="language-options" class="inline-input" />
                    <button mat-icon-button type="button" class="mini-btn" aria-label="Remover" (click)="removeLanguage(i)">
                      <mat-icon>delete</mat-icon>
                    </button>
                  </div>
                }
              </div>
            </div>
          </div>

          <datalist id="proficiency-options">
            @for (p of armorProficiencies; track p) {
              <option [value]="p"></option>
            }
            @for (p of weaponProficiencies; track p) {
              <option [value]="p"></option>
            }
          </datalist>
          <datalist id="tool-options">
            @for (t of toolOptions; track t) {
              <option [value]="t"></option>
            }
          </datalist>
          <datalist id="language-options">
            @for (l of languageOptions; track l) {
              <option [value]="l"></option>
            }
          </datalist>
        </mat-step>

        <!-- ═══════════════ 8. Interpretação ═══════════════ -->
        <mat-step label="Interpretação">
          <h3 class="sub-title">Traços e Características</h3>
          <div class="section-header">
            <span class="muted">Habilidades de classe, raça e talentos.</span>
            <button mat-stroked-button type="button" size="small" (click)="addFeature()">
              <mat-icon>add</mat-icon>
              Traço
            </button>
          </div>
          @if (features.length === 0) {
            <p class="empty-hint">Nenhum traço adicionado.</p>
          }
          <div class="dynamic-list">
            @for (feature of features.controls; track feature; let i = $index) {
              <div class="simple-row">
                <input matInput [formControl]="asControl(feature)" placeholder="Ex.: Ataque Extra" class="inline-input" />
                <button mat-icon-button type="button" class="mini-btn" aria-label="Remover" (click)="removeFeature(i)">
                  <mat-icon>delete</mat-icon>
                </button>
              </div>
            }
          </div>

          <h3 class="sub-title">Personalidade</h3>
          <div class="personality-grid" [formGroup]="personalityGroup">
            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>Traços de Personalidade</mat-label>
              <textarea matInput rows="2" formControlName="traits"></textarea>
            </mat-form-field>
            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>Ideais</mat-label>
              <textarea matInput rows="2" formControlName="ideals"></textarea>
            </mat-form-field>
            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>Vínculos</mat-label>
              <textarea matInput rows="2" formControlName="bonds"></textarea>
            </mat-form-field>
            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>Defeitos</mat-label>
              <textarea matInput rows="2" formControlName="flaws"></textarea>
            </mat-form-field>
          </div>

          <h3 class="sub-title">Aparência</h3>
          <div class="appearance-grid" [formGroup]="appearanceGroup">
            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>Idade</mat-label>
              <input matInput formControlName="age" />
            </mat-form-field>
            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>Altura</mat-label>
              <input matInput formControlName="height" />
            </mat-form-field>
            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>Peso</mat-label>
              <input matInput formControlName="weight" />
            </mat-form-field>
            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>Olhos</mat-label>
              <input matInput formControlName="eyes" />
            </mat-form-field>
            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>Pele</mat-label>
              <input matInput formControlName="skin" />
            </mat-form-field>
            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>Cabelo</mat-label>
              <input matInput formControlName="hair" />
            </mat-form-field>
          </div>

          <mat-form-field appearance="outline" subscriptSizing="dynamic" class="full-width">
            <mat-label>Aliados e Organizações</mat-label>
            <textarea matInput rows="3" [formControl]="alliesControl"></textarea>
          </mat-form-field>
        </mat-step>
      </mat-stepper>
    } @else {
      <section class="sheet-section">
        <p class="step-hint">NPC usa apenas os atributos básicos. Escolha Jogador ou Boss para a ficha completa.</p>
        <div class="attributes-grid" [formGroup]="attributesGroup">
          @for (attr of attributeKeys; track attr) {
            <div class="attribute-field">
              <mat-form-field appearance="outline" subscriptSizing="dynamic">
                <mat-label>{{ abilityShort(attr) }}</mat-label>
                <input matInput type="number" min="1" max="30" [formControlName]="attr" />
              </mat-form-field>
              <span class="bonus-indicator">Mod: {{ formatModifier(modifier(attr)) }}</span>
            </div>
          }
        </div>
      </section>
    }
  `,
  styles: `
    :host {
      display: block;
    }

    .sheet-stepper {
      background: transparent;
    }

    .sheet-section {
      background: rgba(255, 255, 255, 0.03);
      border: 1px solid rgba(255, 255, 255, 0.06);
      border-radius: 12px;
      padding: 20px 24px 24px;
    }

    .step-hint {
      margin: 0 0 16px;
      font-size: 0.875rem;
      opacity: 0.65;
      line-height: 1.4;
      max-width: 60ch;
    }

    .section-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      flex-wrap: wrap;
      margin-bottom: 12px;
    }

    .section-actions {
      display: flex;
      gap: 8px;
      flex-wrap: wrap;
    }

    .sub-title {
      margin: 24px 0 12px;
      font-size: 1rem;
      font-weight: 600;
      color: #c4b5fd;
    }

    .section-header .sub-title {
      margin: 0;
    }

    .empty-hint {
      text-align: center;
      opacity: 0.5;
      font-size: 0.875rem;
      margin: 12px 0;
    }

    .muted {
      opacity: 0.55;
      font-size: 0.8rem;
    }

    .identity-grid,
    .personality-grid,
    .appearance-grid {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 16px;
    }

    .combat-grid,
    .currency-grid {
      display: grid;
      grid-template-columns: repeat(4, minmax(0, 1fr));
      gap: 16px;
    }

    .currency-grid {
      grid-template-columns: repeat(5, minmax(0, 1fr));
    }

    .attributes-grid {
      display: grid;
      grid-template-columns: repeat(6, minmax(0, 1fr));
      gap: 12px;
    }

    .identity-grid > *,
    .personality-grid > *,
    .appearance-grid > *,
    .combat-grid > *,
    .currency-grid > *,
    .attributes-grid > *,
    .saves-grid > *,
    .death-saves > * {
      min-width: 0;
    }

    .identity-grid mat-form-field,
    .personality-grid mat-form-field,
    .appearance-grid mat-form-field,
    .combat-grid mat-form-field,
    .currency-grid mat-form-field,
    .attributes-grid mat-form-field,
    .saves-grid mat-form-field {
      width: 100%;
    }

    .attribute-field {
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 4px;
    }

    .attribute-field mat-form-field {
      width: 100%;
    }

    .attribute-field input {
      text-align: center;
      font-size: 1.15rem;
      font-weight: 700;
    }

    .bonus-indicator {
      font-size: 0.8rem;
      font-weight: 600;
      color: #a78bfa;
    }

    .saves-grid {
      display: grid;
      grid-template-columns: repeat(6, minmax(0, 1fr));
      gap: 12px;
    }

    .save-item {
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 4px;
      padding: 8px;
      border: 1px solid rgba(255, 255, 255, 0.08);
      border-radius: 8px;
    }

    .save-bonus {
      font-weight: 700;
      font-variant-numeric: tabular-nums;
      color: #a78bfa;
    }

    .roll-pool {
      display: flex;
      align-items: center;
      flex-wrap: wrap;
      gap: 8px;
      margin: 0 0 12px;
    }

    .roll-pool-label {
      font-size: 0.875rem;
      opacity: 0.6;
    }

    .roll-chip {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      min-width: 36px;
      padding: 4px 8px;
      border-radius: 8px;
      background: rgba(196, 181, 253, 0.12);
      border: 1px solid rgba(196, 181, 253, 0.28);
      color: #c4b5fd;
      font-weight: 700;
      font-variant-numeric: tabular-nums;
    }

    .skills-table {
      width: 100%;
      border-collapse: collapse;
      font-size: 0.9rem;
    }

    .skills-table th {
      text-align: left;
      font-weight: 600;
      opacity: 0.6;
      padding: 4px 6px;
      font-size: 0.75rem;
      text-transform: uppercase;
      letter-spacing: 0.4px;
    }

    .skills-table td {
      padding: 2px 6px;
      border-top: 1px solid rgba(255, 255, 255, 0.06);
      vertical-align: middle;
    }

    .col-check,
    .col-bonus {
      text-align: center;
      width: 72px;
    }

    .col-ability {
      width: 72px;
      opacity: 0.75;
    }

    .col-action {
      width: 84px;
      white-space: nowrap;
    }

    .inline-input {
      width: 100%;
      padding: 6px 8px;
      background: rgba(255, 255, 255, 0.04);
      border: 1px solid rgba(255, 255, 255, 0.1);
      border-radius: 6px;
      color: inherit;
      font: inherit;
    }

    .bonus-input {
      width: 64px;
      text-align: center;
    }

    .computed-bonus {
      font-weight: 700;
      font-variant-numeric: tabular-nums;
      color: #a78bfa;
    }

    .mini-btn {
      width: 32px;
      height: 32px;
      line-height: 32px;
      padding: 0;
    }

    .mini-btn mat-icon {
      font-size: 18px;
      width: 18px;
      height: 18px;
      line-height: 18px;
    }

    .derived-line {
      margin: 16px 0 0;
      font-size: 0.95rem;
    }

    .derived-line strong {
      color: #a78bfa;
    }

    .death-saves {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 200px));
      gap: 16px;
    }

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

    .dynamic-row mat-form-field,
    .dynamic-row .inline-input,
    .simple-row .inline-input {
      min-width: 0;
    }

    .flex-1 { flex: 1 1 80px; min-width: 60px; }
    .flex-2 { flex: 2 1 160px; min-width: 120px; }
    .flex-3 { flex: 3 1 200px; min-width: 140px; }

    .simple-row {
      display: flex;
      align-items: center;
      gap: 4px;
    }

    .inline-check {
      display: flex;
      align-items: center;
      padding-top: 8px;
    }

    .remove-btn {
      margin-top: 4px;
      flex-shrink: 0;
    }

    .three-list {
      display: grid;
      grid-template-columns: repeat(3, 1fr);
      gap: 24px;
    }

    .full-width {
      width: 100%;
      margin-top: 16px;
    }

    @media (max-width: 720px) {
      .attributes-grid,
      .saves-grid {
        grid-template-columns: repeat(3, minmax(0, 1fr));
      }
      .combat-grid,
      .currency-grid {
        grid-template-columns: repeat(2, minmax(0, 1fr));
      }
      .three-list {
        grid-template-columns: 1fr;
      }
    }

    @media (max-width: 480px) {
      .identity-grid,
      .personality-grid,
      .appearance-grid,
      .combat-grid,
      .currency-grid {
        grid-template-columns: minmax(0, 1fr);
      }
      .attributes-grid,
      .saves-grid {
        grid-template-columns: repeat(2, minmax(0, 1fr));
      }
      .skills-table {
        display: block;
        overflow-x: auto;
      }
    }
  `,
})
export class CharacterSheetFieldsComponent implements OnInit, OnDestroy {
  /** The complete character form created by `createCharacterForm`. */
  @Input({ required: true }) form!: FormGroup;

  private readonly destroy$ = new Subject<void>();

  readonly attributeKeys = ATTRIBUTE_KEYS;
  readonly abilities = ABILITIES;
  readonly conditions = DND_CONDITIONS;
  readonly skillNames = DND_SKILLS.map((s) => s.name);
  readonly armorProficiencies = DND_ARMOR_PROFICIENCIES;
  readonly weaponProficiencies = DND_WEAPON_PROFICIENCIES;
  readonly toolOptions = DND_TOOLS;
  readonly languageOptions = DND_LANGUAGES;
  readonly formatModifier = formatModifier;

  private readonly fb = inject(FormBuilder);

  // ── Attribute roll state ──────────────────────────────
  rolledPool: number[] = [];
  assigned: Record<string, number> = {};
  private rolledAll: number[] = [];

  get hasRoll(): boolean {
    return this.rolledAll.length > 0;
  }

  get type(): string {
    return this.form?.get('type')?.value ?? 'player';
  }

  get isFullSheet(): boolean {
    return this.type === 'player' || this.type === 'boss';
  }

  get isMinion(): boolean {
    return this.type === 'minion';
  }

  // ── Form array accessors ──────────────────────────────
  get skills(): FormArray {
    return this.form.get('skills') as FormArray;
  }

  get inventory(): FormArray {
    return this.form.get('inventory') as FormArray;
  }

  get attacks(): FormArray {
    return this.form.get('attacks') as FormArray;
  }

  get spells(): FormArray {
    return this.form.get('spellcasting.spells') as FormArray;
  }

  get slots(): FormArray {
    return this.form.get('spellcasting.slots') as FormArray;
  }

  get proficiencies(): FormArray {
    return this.form.get('proficiencies') as FormArray;
  }

  get tools(): FormArray {
    return this.form.get('tools') as FormArray;
  }

  get languages(): FormArray {
    return this.form.get('languages') as FormArray;
  }

  get features(): FormArray {
    return this.form.get('features') as FormArray;
  }

  // ── Sub-group accessors (bound with [formGroup], not formGroupName, so the
  //    nested directives resolve inside this component's injector) ──────────
  get identityGroup(): FormGroup {
    return this.form.get('identity') as FormGroup;
  }

  get attributesGroup(): FormGroup {
    return this.form.get('attributes') as FormGroup;
  }

  get savingThrowsGroup(): FormGroup {
    return this.form.get('savingThrows') as FormGroup;
  }

  get combatGroup(): FormGroup {
    return this.form.get('combat') as FormGroup;
  }

  get spellcastingGroup(): FormGroup {
    return this.form.get('spellcasting') as FormGroup;
  }

  get currencyGroup(): FormGroup {
    return this.form.get('currency') as FormGroup;
  }

  get personalityGroup(): FormGroup {
    return this.form.get('personality') as FormGroup;
  }

  get appearanceGroup(): FormGroup {
    return this.form.get('appearance') as FormGroup;
  }

  get minionGroup(): FormGroup {
    return this.form.get('minion') as FormGroup;
  }

  get alliesControl(): FormControl {
    return this.form.get('allies') as FormControl;
  }

  /** Narrow a dynamic row control to its group/control type for the template. */
  asGroup(control: unknown): FormGroup {
    return control as FormGroup;
  }

  asControl(control: unknown): FormControl {
    return control as FormControl;
  }

  get spellcastingActive(): boolean {
    return Boolean(this.form.get('spellcasting')?.get('active')?.value);
  }

  ngOnInit(): void {
    this.form.valueChanges.pipe(takeUntil(this.destroy$)).subscribe(() => {
      this.clearExpertiseWithoutProficiency();
      this.recomputeDerived();
    });

    this.form
      .get('identity')
      ?.get('class')
      ?.valueChanges.pipe(distinctUntilChanged(), takeUntil(this.destroy$))
      .subscribe((name: string) => this.applyClassPreset(name));

    this.form
      .get('identity')
      ?.get('race')
      ?.valueChanges.pipe(distinctUntilChanged(), takeUntil(this.destroy$))
      .subscribe((name: string) => {
        this.form.get('combat')?.get('speed')?.setValue(raceSpeed(name), { emitEvent: false });
      });

    this.form
      .get('identity')
      ?.get('level')
      ?.valueChanges.pipe(distinctUntilChanged(), takeUntil(this.destroy$))
      .subscribe((level: number) => {
        this.form
          .get('combat')
          ?.get('hitDiceTotal')
          ?.setValue(Math.max(1, Number(level) || 1), { emitEvent: false });
      });

    this.recomputeDerived();
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

  // ── Derived values ────────────────────────────────────

  modifier(attr: string): number {
    return abilityModifier(Number(this.attributesValue[attr]) || 10);
  }

  abilityShort(attr: string): string {
    return this.abilities.find((a) => a.key === attr)?.short ?? attr.toUpperCase();
  }

  private get attributesValue(): Record<string, number> {
    return (this.form.get('attributes')?.value as Record<string, number>) ?? {};
  }

  private get proficiencyBonusValue(): number {
    return Number(this.form.get('combat')?.get('proficiencyBonus')?.value) || 2;
  }

  skillBonus(index: number): number {
    const row = this.skillRow(index);
    if (!row) return 0;
    return effectiveSkillBonus(row, this.attributesValue, this.proficiencyBonusValue);
  }

  savingBonus(attr: string): number {
    const proficient = Boolean(this.form.get('savingThrows')?.get(attr)?.value);
    return savingThrowBonus(
      attr as AttributeKey,
      proficient,
      this.attributesValue,
      this.proficiencyBonusValue,
    );
  }

  passivePerceptionValue(): number {
    return passivePerception(
      this.attributesValue,
      this.proficiencyBonusValue,
      this.findSkillRow('Percepção'),
    );
  }

  private skillRow(index: number): SkillEntry | null {
    const group = this.skills.at(index);
    if (!group) return null;
    return group.value as SkillEntry;
  }

  private findSkillRow(name: string): SkillEntry | null {
    const match = this.skills.controls.find(
      (c) => String(c.get('name')?.value).toLowerCase() === name.toLowerCase(),
    );
    return match ? (match.value as SkillEntry) : null;
  }

  isOverridden(field: DerivedField): boolean {
    return Boolean(this.form.get('overrides')?.get(field)?.value);
  }

  toggleOverride(field: DerivedField): void {
    const control = this.form.get('overrides')?.get(field);
    if (!control) return;
    control.setValue(!control.value, { emitEvent: false });
    this.recomputeDerived();
  }

  toggleSkillOverride(index: number): void {
    const control = this.skills.at(index)?.get('bonusOverride');
    if (!control) return;
    if (control.value === null) {
      control.setValue(this.skillBonus(index), { emitEvent: false });
    } else {
      control.setValue(null, { emitEvent: false });
    }
    this.recomputeDerived();
  }

  private clearExpertiseWithoutProficiency(): void {
    for (const control of this.skills.controls) {
      if (!control.get('proficient')?.value && control.get('expertise')?.value) {
        control.get('expertise')?.setValue(false, { emitEvent: false });
      }
    }
  }

  private recomputeDerived(): void {
    const overrides = (this.form.get('overrides')?.value as Record<string, boolean>) ?? {};
    const attributes = this.attributesValue;
    const level = Number(this.form.get('identity')?.get('level')?.value) || 1;
    const combat = this.form.get('combat');
    const spellcasting = this.form.get('spellcasting');

    if (!overrides['proficiencyBonus']) {
      combat?.get('proficiencyBonus')?.setValue(proficiencyBonusForLevel(level), { emitEvent: false });
    }
    const profBonus = this.proficiencyBonusValue;

    if (!overrides['initiative']) {
      combat
        ?.get('initiative')
        ?.setValue(abilityModifier(Number(attributes['des']) || 10), { emitEvent: false });
    }

    if (!overrides['passivePerception']) {
      combat
        ?.get('passivePerception')
        ?.setValue(passivePerception(attributes, profBonus, this.findSkillRow('Percepção')), {
          emitEvent: false,
        });
    }

    const ability = spellcasting?.get('ability')?.value as AttributeKey | '' | undefined;
    if (ability) {
      const mod = abilityModifier(Number(attributes[ability]) || 10);
      if (!overrides['spellSaveDc']) {
        spellcasting?.get('saveDc')?.setValue(computeSpellSaveDc(mod, profBonus), { emitEvent: false });
      }
      if (!overrides['spellAttackBonus']) {
        spellcasting
          ?.get('attackBonus')
          ?.setValue(computeSpellAttackBonus(mod, profBonus), { emitEvent: false });
      }
    }
  }

  private applyClassPreset(name: string): void {
    const def = findClass(name);
    if (!def) return;
    this.form.get('combat')?.get('hitDice')?.setValue(def.hitDice, { emitEvent: false });
    const saves = this.form.get('savingThrows');
    for (const key of this.attributeKeys) {
      saves?.get(key)?.setValue(def.savingThrows.includes(key), { emitEvent: false });
    }
    const spellcasting = this.form.get('spellcasting');
    spellcasting?.get('ability')?.setValue(def.spellcastingAbility ?? '', { emitEvent: false });
    spellcasting?.get('active')?.setValue(def.spellcasting !== 'none', { emitEvent: false });
    this.recomputeDerived();
  }

  // ── Attribute roll ────────────────────────────────────

  onRollAttributes(): void {
    this.rolledAll = rollAttributeSet();
    this.rolledPool = [...this.rolledAll];
    this.assigned = {};
    const attributes = this.form.get('attributes');
    for (const attr of this.attributeKeys) {
      attributes?.get(attr)?.setValue(null, { emitEvent: false });
    }
    this.recomputeDerived();
  }

  onAssign(attr: string, value: number | null): void {
    if (value === null || value === undefined) return;
    const previous = this.assigned[attr];
    if (previous === value) return;
    const pool = previous === undefined ? [...this.rolledPool] : [...this.rolledPool, previous];
    const index = pool.indexOf(value);
    if (index === -1) return;
    pool.splice(index, 1);
    pool.sort((a, b) => b - a);
    this.rolledPool = pool;
    this.assigned = { ...this.assigned, [attr]: value };
    this.form.get('attributes')?.get(attr)?.setValue(value);
    this.recomputeDerived();
  }

  optionsFor(attr: string): { value: number; disabled: boolean }[] {
    const remaining = new Map<number, number>();
    for (const value of this.rolledPool) {
      remaining.set(value, (remaining.get(value) ?? 0) + 1);
    }
    const current = this.assigned[attr];
    const distinct = [...new Set(this.rolledAll)].sort((a, b) => b - a);
    return distinct.map((value) => ({
      value,
      disabled: value !== current && (remaining.get(value) ?? 0) === 0,
    }));
  }

  onClearRoll(): void {
    this.rolledAll = [];
    this.rolledPool = [];
    this.assigned = {};
    const attributes = this.form.get('attributes');
    for (const attr of this.attributeKeys) {
      const control = attributes?.get(attr);
      if (control && (control.value === null || control.value === '')) {
        control.setValue(10, { emitEvent: false });
      }
    }
    this.recomputeDerived();
  }

  // ── Dynamic list mutations ────────────────────────────

  addSkill(): void {
    this.skills.push(
      this.fb.group({
        name: ['', Validators.required],
        ability: ['for'],
        proficient: [false],
        expertise: [false],
        bonusOverride: [null],
      }),
    );
  }

  removeSkill(index: number): void {
    this.skills.removeAt(index);
  }

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

  addAttack(): void {
    this.attacks.push(
      this.fb.group({ name: [''], attackBonus: [''], damage: [''], damageType: [''] }),
    );
  }

  removeAttack(index: number): void {
    this.attacks.removeAt(index);
  }

  addSpell(): void {
    this.spells.push(this.fb.group({ name: [''], level: [0], prepared: [false] }));
  }

  removeSpell(index: number): void {
    this.spells.removeAt(index);
  }

  addSlot(): void {
    this.slots.push(this.fb.group({ level: [1], total: [0], used: [0] }));
  }

  removeSlot(index: number): void {
    this.slots.removeAt(index);
  }

  addProficiency(): void {
    this.proficiencies.push(this.fb.control(''));
  }

  removeProficiency(index: number): void {
    this.proficiencies.removeAt(index);
  }

  addTool(): void {
    this.tools.push(this.fb.control(''));
  }

  removeTool(index: number): void {
    this.tools.removeAt(index);
  }

  addLanguage(): void {
    this.languages.push(this.fb.control(''));
  }

  removeLanguage(index: number): void {
    this.languages.removeAt(index);
  }

  addFeature(): void {
    this.features.push(this.fb.control(''));
  }

  removeFeature(index: number): void {
    this.features.removeAt(index);
  }
}