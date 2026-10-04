import { Component, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { ChangeRowsList } from '../change-rows/change-rows';
import { LevelUpSession } from '../level-up-session';

/**
 * Step "Resumo" of the guided level-up (MR-040, E8-15): every change of the level, before →
 * after, as the server derived it; the line that the rest of the sheet does not change and
 * stays locked; and, quietly, what the master adds in the sheet editor (the features whose
 * choice the flow does not cover). "Confirmar o nível N" lives in the page's footer, the
 * screen's one filled button.
 */
@Component({
  selector: 'app-summary-step',
  imports: [ChangeRowsList, MatIconModule],
  template: `
    <section class="mr-panel" aria-labelledby="summary-title">
      <h2 class="mr-panel__title" id="summary-title">O que muda</h2>
      <p class="lead">Confira antes de confirmar. Depois, só o mestre muda a ficha.</p>
      <app-change-rows [rows]="s().rows()" />
    </section>
    <p class="rest">
      <mat-icon aria-hidden="true">lock</mat-icon>
      <span>O resto da ficha não muda e continua travado.</span>
    </p>
    @if (s().masterAdds()) {
      <p class="rest rest--quiet">
        <mat-icon aria-hidden="true">edit_note</mat-icon>
        <span>O mestre acrescenta pelo editor: {{ s().masterAdds() }}.</span>
      </p>
    }
  `,
  styles: `
    :host {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-4);

      // A summary reads label to value: not wider than a line can be followed.
      // From 1100px the step has its own column, which it fills like the
      // other steps, next to "O resto da ficha".
      @media (min-width: 768px) and (max-width: 1099.98px) {
        max-width: 640px;
      }
    }

    .lead {
      margin: 0 0 var(--mr-space-2);
      font-size: 15px;
      line-height: 20px;
      color: var(--mr-ink-muted);
    }

    .rest {
      display: flex;
      align-items: flex-start;
      gap: var(--mr-space-3);
      margin: 0;
      font-size: 15px;
      line-height: 20px;

      &--quiet {
        color: var(--mr-ink-muted);
      }

      .mat-icon {
        flex: none;
        width: 20px;
        height: 20px;
        font-size: 20px;
      }
    }
  `,
})
export class SummaryStep {
  readonly s = input.required<LevelUpSession>();
}
