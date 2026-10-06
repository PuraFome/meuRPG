import { ChangeDetectionStrategy, Component, OnInit, inject, input, output, signal } from '@angular/core';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';

import { type HintCheckDraft, DC_MAX, DC_MIN } from '../../../core/puzzles/puzzle-draft';
import { type CheckOption, SceneChecks } from '../../../core/maps/scene-actions';

/**
 * "Dica por teste de perícia" of every form (MR-038, RN-27, RN-18, E10-12 state 4): a player rolls a skill against the master's DC and, on
 * a pass, wins the next hint, for them alone. A skill and a DC, both or neither ("Nenhuma" is neither). The DC is the master's alone: a
 * player's phone says "Tentar uma dica · Investigação" and never a number. The hints themselves are the list above ("Dicas"): the check
 * needs at least one. The 18 skills come from the rules' content (`SceneChecks`), named in Portuguese.
 */
@Component({
  selector: 'app-hint-check-field',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatFormFieldModule, MatInputModule],
  template: `
    <section class="hc" aria-labelledby="hc-title">
      <h3 class="hc__title" id="hc-title">Dica por teste de perícia</h3>
      <p class="hc__help">Um jogador rola contra a sua CD; passando, ele recebe a próxima dica, só para ele. Os jogadores nunca veem a CD.</p>
      @if (failed()) {
        <p class="hc__bad" role="alert">Não deu para ler as perícias. Volte e tente de novo.</p>
      }
      <div class="hc__row">
        <mat-form-field appearance="outline" subscriptSizing="dynamic" class="hc__skill" [class.field-bad]="error()">
          <mat-label>Perícia</mat-label>
          <select matNativeControl [value]="check().skillKey" [attr.aria-invalid]="error() ? 'true' : null" (change)="pick($any($event.target).value)">
            <option value="" [selected]="check().skillKey === ''">Nenhuma</option>
            @for (s of skills(); track s.key) {
              <option [value]="s.key" [selected]="s.key === check().skillKey">{{ s.label }}</option>
            }
          </select>
        </mat-form-field>
        @if (check().skillKey !== '') {
          <mat-form-field appearance="outline" subscriptSizing="dynamic" class="hc__dc" [class.field-bad]="error()">
            <mat-label>CD</mat-label>
            <input matInput type="text" inputmode="numeric" autocomplete="off" [value]="check().dcText" [attr.aria-invalid]="error() ? 'true' : null" (input)="checkChange.emit({ ...check(), dcText: $any($event.target).value })" />
            <mat-hint>De {{ dcMin }} a {{ dcMax }}</mat-hint>
          </mat-form-field>
        }
      </div>
      @if (error()) {
        <p class="hc__bad field-error" role="alert">{{ error() }}</p>
      }
    </section>
  `,
  styles: `
    :host {
      display: block;
    }

    .hc__title {
      margin: 0 0 4px;
      font-size: 16px;
      font-weight: 700;
    }

    .hc__help {
      margin: 0 0 var(--mr-space-3);
      font-size: 14px;
      line-height: 20px;
      color: var(--mr-ink-muted);
    }

    .hc__bad {
      margin: var(--mr-space-2) 0 0;
      font-size: 14px;
      color: var(--mr-danger-ink);
    }

    .hc__row {
      display: grid;
      grid-template-columns: minmax(0, 1fr) 120px;
      gap: var(--mr-space-3);
      align-items: start;
    }

    .hc__row:has(.hc__dc) .hc__skill {
      grid-column: 1;
    }

    .hc__row:not(:has(.hc__dc)) {
      grid-template-columns: minmax(0, 1fr);
    }

    .hc__skill,
    .hc__dc {
      min-width: 0;
      scroll-margin-block: calc(64px + var(--mr-space-4));
    }

    .field-bad {
      --mat-form-field-outlined-outline-color: var(--mr-danger-ink);
      --mat-form-field-outlined-hover-outline-color: var(--mr-danger-ink);
      --mat-form-field-outlined-focus-outline-color: var(--mr-danger-ink);
    }
  `,
})
export class HintCheckField implements OnInit {
  private readonly checks = inject(SceneChecks);
  protected readonly dcMin = DC_MIN;
  protected readonly dcMax = DC_MAX;

  readonly campaignId = input.required<string>();
  readonly check = input.required<HintCheckDraft>();
  /** What is wrong with the check ("A CD vai de 1 a 30."), after a try to save. */
  readonly error = input('');
  readonly checkChange = output<HintCheckDraft>();

  protected readonly skills = signal<readonly CheckOption[]>([]);
  protected readonly failed = signal(false);

  ngOnInit(): void {
    this.checks.skills(this.campaignId()).then(
      (skills) => this.skills.set(skills),
      () => this.failed.set(true),
    );
  }

  protected pick(skillKey: string): void {
    // "Nenhuma" clears the DC too: both or neither.
    this.checkChange.emit(skillKey === '' ? { skillKey: '', dcText: '' } : { ...this.check(), skillKey });
  }
}
