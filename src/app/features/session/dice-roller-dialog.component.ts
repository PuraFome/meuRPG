import { Component, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { trigger, transition, style, animate } from '@angular/animations';
import { MatDialogModule, MatDialogRef } from '@angular/material/dialog';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';

interface DiceRollConfig {
  count: number;
  sides: number;
  modifier: number;
}

interface DiceRollResult {
  label: string;
  individual: number[];
  modifier: number;
  total: number;
  timestamp: Date;
}

@Component({
  selector: 'app-dice-roller-dialog',
  standalone: true,
  imports: [
    CommonModule,
    FormsModule,
    MatDialogModule,
    MatButtonModule,
    MatIconModule,
    MatFormFieldModule,
    MatInputModule,
  ],
  template: `
    <h2 mat-dialog-title class="dialog-title">
      <mat-icon>casino</mat-icon>
      Rolar Dados
    </h2>

    <mat-dialog-content class="dialog-content">
      <!-- Input -->
      <mat-form-field appearance="outline" class="notation-field">
        <mat-label>Notação (ex: 1d20, 3d6, 2d8+4)</mat-label>
        <input
          matInput
          [(ngModel)]="notation"
          placeholder="ex: 1d20"
          (ngModelChange)="parseNotation()"
          (keyup.enter)="roll()"
          [disabled]="isRolling"
          #notationInput
        />
        @if (notation) {
          <button
            matSuffix
            mat-icon-button
            aria-label="Limpar"
            (click)="clearNotation()"
            [disabled]="isRolling"
          >
            <mat-icon>close</mat-icon>
          </button>
        }
      </mat-form-field>

      <!-- Roll Button -->
      <div class="roll-action">
        <button
          mat-raised-button
          color="primary"
          class="roll-btn"
          (click)="roll()"
          [disabled]="!parsedNotation || isRolling"
        >
          {{ isRolling ? 'Rolando...' : 'Rolar' }}
          @if (isRolling) {
            <mat-icon class="spin-icon" iconPositionEnd>autorenew</mat-icon>
          } @else {
            <mat-icon iconPositionEnd>casino</mat-icon>
          }
        </button>
      </div>

      <!-- Current Roll Result -->
      @if (currentResult) {
        <div class="result-card">
          <div class="result-label">{{ currentResult.label }}</div>

          <div class="dice-grid">
            @for (die of currentResult.individual; track $index) {
              <div class="die-box" [class.high-roll]="die >= (parsedNotation?.sides ?? 20) * 0.9 && die > 1" [class.low-roll]="die === 1">
                <span class="die-value">{{ die }}</span>
                <span class="die-sides">d{{ parsedNotation?.sides }}</span>
              </div>
            }
          </div>

          <div class="result-total">
            @if (currentResult.modifier !== 0) {
              <span class="total-breakdown">
                {{ currentResult.individual.reduce((a, b) => a + b, 0) }}
                {{ currentResult.modifier > 0 ? '+' : '' }}{{ currentResult.modifier }}
                =
              </span>
            }
            <span class="total-value">{{ currentResult.total }}</span>
          </div>
        </div>
      }

      <!-- Roll History -->
      @if (rollHistory.length > 0) {
        <div class="history-section">
          <h3 class="history-title">
            <mat-icon>history</mat-icon>
            Histórico
          </h3>

          <div class="history-list">
            @for (roll of rollHistory; track $index) {
              <div class="history-item">
                <div class="history-meta">
                  <span class="history-label">{{ roll.label }}</span>
                  <span class="history-time">{{ roll.timestamp | date:'HH:mm:ss' }}</span>
                </div>
                <span class="history-total">{{ roll.total }}</span>
              </div>
            }
          </div>
        </div>
      }
    </mat-dialog-content>

    <mat-dialog-actions align="end">
      <button mat-button (click)="close()">Fechar</button>
    </mat-dialog-actions>
  `,
  styles: `
    :host {
      display: block;
    }

    .dialog-title {
      display: flex;
      align-items: center;
      gap: 8px;
      font-weight: 600;
    }

    .dialog-title mat-icon {
      font-size: 1.3rem;
      width: 1.3rem;
      height: 1.3rem;
    }

    .dialog-content {
      min-width: min(320px, 90vw);
      max-height: 60vh;
      overflow-y: auto;
    }

    /* ── Notation Input ─────────────────────── */

    .notation-field {
      width: 100%;
    }

    /* ── Roll Action ────────────────────────── */

    .roll-action {
      display: flex;
      justify-content: center;
      margin-bottom: 20px;
    }

    .roll-btn {
      min-width: 160px;
      display: flex;
      align-items: center;
      gap: 6px;
    }

    .spin-icon {
      animation: spin 1s linear infinite;
    }

    @keyframes spin {
      from { transform: rotate(0deg); }
      to { transform: rotate(360deg); }
    }

    /* ── Result Card ────────────────────────── */

    .result-card {
      background: rgba(255, 255, 255, 0.03);
      border: 1px solid rgba(255, 255, 255, 0.08);
      border-radius: 12px;
      padding: 20px;
      margin-bottom: 16px;
      animation: fadeSlideIn 0.3s ease;
    }

    @keyframes fadeSlideIn {
      from {
        opacity: 0;
        transform: translateY(8px);
      }
      to {
        opacity: 1;
        transform: translateY(0);
      }
    }

    .result-label {
      font-size: 0.8rem;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.5px;
      color: rgba(255, 255, 255, 0.4);
      margin-bottom: 16px;
    }

    .dice-grid {
      display: flex;
      flex-wrap: wrap;
      gap: 10px;
      justify-content: center;
      margin-bottom: 16px;
    }

    .die-box {
      width: 56px;
      height: 68px;
      border-radius: 10px;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      background: rgba(255, 255, 255, 0.04);
      border: 1px solid rgba(255, 255, 255, 0.08);
      transition: all 0.2s ease;
      animation: diePop 0.3s ease backwards;
    }

    .die-box:nth-child(1) { animation-delay: 0s; }
    .die-box:nth-child(2) { animation-delay: 0.05s; }
    .die-box:nth-child(3) { animation-delay: 0.1s; }
    .die-box:nth-child(4) { animation-delay: 0.15s; }
    .die-box:nth-child(5) { animation-delay: 0.2s; }
    .die-box:nth-child(6) { animation-delay: 0.25s; }
    .die-box:nth-child(7) { animation-delay: 0.3s; }
    .die-box:nth-child(8) { animation-delay: 0.35s; }

    @keyframes diePop {
      from {
        opacity: 0;
        transform: scale(0.8) translateY(10px);
      }
      to {
        opacity: 1;
        transform: scale(1) translateY(0);
      }
    }

    .die-box.high-roll {
      border-color: rgba(76, 175, 80, 0.5);
      background: rgba(76, 175, 80, 0.08);
    }

    .die-box.low-roll {
      border-color: rgba(244, 67, 54, 0.5);
      background: rgba(244, 67, 54, 0.08);
    }

    .die-value {
      font-size: 1.5rem;
      font-weight: 700;
      line-height: 1;
      color: #e0e0e0;
    }

    .die-sides {
      font-size: 0.65rem;
      opacity: 0.4;
      margin-top: 2px;
    }

    .result-total {
      text-align: center;
    }

    .total-breakdown {
      font-size: 0.85rem;
      opacity: 0.5;
      margin-right: 6px;
    }

    .total-value {
      font-size: 2rem;
      font-weight: 700;
      background: linear-gradient(135deg, #e0e0e0 0%, #b388ff 100%);
      -webkit-background-clip: text;
      -webkit-text-fill-color: transparent;
      background-clip: text;
    }

    /* ── History ────────────────────────────── */

    .history-section {
      margin-top: 8px;
    }

    .history-title {
      display: flex;
      align-items: center;
      gap: 6px;
      font-size: 0.85rem;
      font-weight: 600;
      color: rgba(255, 255, 255, 0.4);
      margin: 0 0 8px;
    }

    .history-title mat-icon {
      font-size: 1rem;
      width: 1rem;
      height: 1rem;
    }

    .history-list {
      display: flex;
      flex-direction: column;
      gap: 4px;
    }

    .history-item {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 8px 12px;
      border-radius: 6px;
      background: rgba(255, 255, 255, 0.02);
      transition: background 0.15s ease;
    }

    .history-item:hover {
      background: rgba(255, 255, 255, 0.04);
    }

    .history-meta {
      display: flex;
      align-items: center;
      gap: 8px;
    }

    .history-label {
      font-size: 0.85rem;
      font-weight: 500;
    }

    .history-time {
      font-size: 0.7rem;
      opacity: 0.35;
      font-family: monospace;
    }

    .history-total {
      font-size: 1rem;
      font-weight: 700;
      color: #b388ff;
    }
  `,
  animations: [
    trigger('fadeSlide', [
      transition(':enter', [
        style({ opacity: 0, transform: 'translateY(-12px)' }),
        animate('250ms ease-out', style({ opacity: 1, transform: 'translateY(0)' })),
      ]),
    ]),
  ],
  host: { '[@fadeSlide]': '' },
})
export class DiceRollerDialogComponent {
  private readonly dialogRef = inject(MatDialogRef<DiceRollerDialogComponent>);

  notation = '';
  parsedNotation: DiceRollConfig | null = null;
  isRolling = false;
  currentResult: DiceRollResult | null = null;
  rollHistory: DiceRollResult[] = [];

  constructor() {
    // Auto-parse as user types
    this.parseNotation();
  }

  parseNotation(): void {
    const trimmed = this.notation.trim();
    if (!trimmed) {
      this.parsedNotation = null;
      return;
    }

    const match = trimmed.match(/^(\d+)[dD](\d+)(?:([+-])(\d+))?$/);
    if (!match) {
      this.parsedNotation = null;
      return;
    }

    const count = parseInt(match[1], 10);
    const sides = parseInt(match[2], 10);
    const modSign = match[3] === '+' ? 1 : match[3] === '-' ? -1 : 0;
    const modValue = match[4] ? parseInt(match[4], 10) : 0;

    if (count < 1 || count > 100 || sides < 2 || sides > 1000) {
      this.parsedNotation = null;
      return;
    }

    this.parsedNotation = { count, sides, modifier: modSign * modValue };
  }

  clearNotation(): void {
    this.notation = '';
    this.parsedNotation = null;
  }

  roll(): void {
    if (!this.parsedNotation || this.isRolling) return;

    const config = this.parsedNotation;
    this.isRolling = true;

    // Generate actual results
    const actualRolls: number[] = [];
    for (let i = 0; i < config.count; i++) {
      actualRolls.push(Math.floor(Math.random() * config.sides) + 1);
    }

    // Animate: cycle random values for ~800ms
    const animationDuration = 800;
    const intervalMs = 80;
    const steps = animationDuration / intervalMs;
    let step = 0;

    const animRolls: number[][] = [];
    for (let i = 0; i < steps; i++) {
      const frame: number[] = [];
      for (let j = 0; j < config.count; j++) {
        frame.push(Math.floor(Math.random() * config.sides) + 1);
      }
      animRolls.push(frame);
    }

    const interval = setInterval(() => {
      if (step < steps) {
        // Show intermediate values
        this.currentResult = {
          label: `${config.count}d${config.sides}${config.modifier !== 0 ? (config.modifier > 0 ? '+' : '') + config.modifier : ''}`,
          individual: animRolls[step],
          modifier: config.modifier,
          total: animRolls[step].reduce((a, b) => a + b, 0) + config.modifier,
          timestamp: new Date(),
        };
        step++;
      } else {
        // Show final result
        clearInterval(interval);
        const total = actualRolls.reduce((a, b) => a + b, 0) + config.modifier;
        const result: DiceRollResult = {
          label: `${config.count}d${config.sides}${config.modifier !== 0 ? (config.modifier > 0 ? '+' : '') + config.modifier : ''}`,
          individual: actualRolls,
          modifier: config.modifier,
          total,
          timestamp: new Date(),
        };

        this.currentResult = result;
        this.rollHistory.unshift(result);
        this.isRolling = false;
      }
    }, intervalMs);
  }

  close(): void {
    this.dialogRef.close();
  }
}
