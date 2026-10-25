import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  afterNextRender,
  computed,
  input,
  output,
  signal,
  viewChild,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';

import type { ActionOption } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { spendPreview, spendRange } from '../../../../core/combat/action-spend';
import { CountStepper } from '../../../../shared/count-stepper/count-stepper';

/**
 * "Quantos?" (MR-025): the question a table feature's action asks when the player may choose how many of its resource to
 * spend (`cost_max` above the cost). It sits under the action's row, in place like the frenzy question: a stepper from the
 * least the action costs to the smaller of `cost_max` and the uses left, a live line of what the number will cost and give
 * ("Gasta 3 · cura 3d6", for display only: the server decides and refuses a number outside the range), and "Usar" /
 * "Cancelar". Focus goes to the question when it opens; Esc cancels.
 */
@Component({
  selector: 'app-spend-ask',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [CountStepper, MatButtonModule],
  template: `
    <div class="ask" role="group" [attr.aria-label]="'Quantos? ' + name()">
      <p #question class="ask__q" tabindex="-1">Quantos?</p>
      <div class="ask__step">
        <app-count-stepper
          noun="uso"
          minusLabel="Gastar um a menos"
          plusLabel="Gastar um a mais"
          valueLabel="Gastar {n}"
          [value]="n()"
          [min]="range().min"
          [max]="range().max"
          (valueChange)="picked.set($event)"
        />
        <span class="ask__range">de {{ range().min }} a {{ range().max }}</span>
      </div>
      <p class="ask__preview" role="status" aria-live="polite">{{ preview() }}</p>
      <div class="ask__buttons">
        <button
          mat-flat-button
          type="button"
          [disabled]="busy()"
          [attr.aria-label]="'Usar ' + name() + ' gastando ' + n()"
          (click)="confirmed.emit(n())"
        >
          Usar
        </button>
        <button mat-stroked-button type="button" [disabled]="busy()" (click)="cancelled.emit()">
          Cancelar
        </button>
      </div>
    </div>
  `,
  styles: `
    :host {
      display: block;
    }

    .ask {
      display: flex;
      flex-direction: column;
      gap: 8px;
      padding: 12px 16px;
      border-top: 1px dashed var(--mr-line);
    }

    .ask__q {
      margin: 0;
      font-weight: 700;
      font-size: 16px;
      line-height: 24px;

      &:focus-visible {
        outline: 3px solid var(--mr-focus);
        outline-offset: 2px;
      }
    }

    .ask__step {
      display: flex;
      align-items: center;
      flex-wrap: wrap;
      gap: 12px;
    }

    .ask__range,
    .ask__preview {
      margin: 0;
      font-size: 15px;
      line-height: 20px;
      color: var(--mr-ink-muted);
    }

    .ask__preview {
      color: var(--mr-ink);
      font-weight: 600;
    }

    .ask__buttons {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
    }

    .ask__buttons button {
      min-height: 44px;
    }
  `,
  host: { '(keydown.escape)': 'cancelled.emit()' },
})
export class SpendAsk {
  /** The action's option from `GetTurnOptions`: its cost, its range and what it gives. */
  readonly option = input.required<ActionOption>();
  readonly busy = input(false);
  readonly confirmed = output<number>();
  readonly cancelled = output<void>();

  private readonly question = viewChild.required<ElementRef<HTMLElement>>('question');
  protected readonly name = computed(() => this.option().action?.namePt ?? '');
  protected readonly range = computed(() => spendRange(this.option()) ?? { min: 1, max: 1 });
  protected readonly picked = signal<number | null>(null);
  /** The number shown: the one picked, held inside the range, else the least. */
  protected readonly n = computed(() => {
    const { min, max } = this.range();
    return Math.min(Math.max(this.picked() ?? min, min), max);
  });
  protected readonly preview = computed(() => spendPreview(this.option(), this.n()));

  constructor() {
    afterNextRender(() => this.question().nativeElement.focus({ preventScroll: false }));
  }
}
