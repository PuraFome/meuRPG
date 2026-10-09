import { ChangeDetectionStrategy, Component, input } from '@angular/core';

import type { CounterRow } from '../../core/puzzles/puzzle-format';

/**
 * The counters a puzzle with limits keeps on screen (MR-038, E10-12 states 6, 8 and 10): "Tentativas restantes 2 de 3", "Jogadas 7 de 10",
 * "Tempo 4:48 de 5:00". A label on the left and the value, in bold, on the right; a counter that has run out says so in words
 * ("acabou") as well as in its colour.
 */
@Component({
  selector: 'app-limit-counters',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (rows().length > 0) {
      <dl class="rows">
        @for (row of rows(); track row.key) {
          <div class="row" [class.row--spent]="row.spent">
            <dt>{{ row.label }}</dt>&ngsp;<dd>{{ row.value }}@if (row.spent) { <span class="spent"> · acabou</span> }</dd>
          </div>
        }
      </dl>
    }
  `,
  styles: `
    :host {
      display: block;
    }

    .rows {
      display: flex;
      flex-direction: column;
      gap: 2px;
      margin: 0;
    }

    .row {
      display: flex;
      justify-content: space-between;
      gap: var(--mr-space-3);
      font-size: 16px;
      line-height: 22px;
    }

    dt {
      margin: 0;
    }

    dd {
      margin: 0;
      font-weight: 700;
      text-align: right;
      font-variant-numeric: lining-nums tabular-nums;
    }

    .spent {
      font-weight: 400;
    }

    .row--spent dd {
      color: var(--mr-danger-ink);
    }
  `,
})
export class LimitCounters {
  readonly rows = input.required<readonly CounterRow[]>();
}
