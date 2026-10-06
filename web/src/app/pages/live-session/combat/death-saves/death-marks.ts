import { Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { marks } from '../../../../core/combat/death-saves';

/**
 * The marks of the death saves (E6-13, E6-30): three tries of one kind, as
 * discs. A success that happened is a disc with a check, a failure a disc with
 * a cross, a try that did not happen a hollow ring: a mark is a shape, never a
 * colour alone. Decorative: the words around it ("1 de 3", "Morrendo · 3 falhas")
 * carry the count, so the marks are hidden from assistive tech.
 */
@Component({
  selector: 'app-death-marks',
  imports: [MatIconModule],
  template: `
    @for (on of dots(); track $index) {
      <span class="mark" [class.mark--on]="on" [class.mark--fail]="kind() === 'failure'">
        @if (on) {
          <mat-icon>{{ kind() === 'success' ? 'check' : 'close' }}</mat-icon>
        }
      </span>
    }
  `,
  styles: `
    :host {
      display: inline-flex;
      gap: calc(var(--dm) * 0.25);
    }

    .mark {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      box-sizing: border-box;
      width: var(--dm);
      height: var(--dm);
      border: 2px solid var(--mr-ink-muted);
      border-radius: 50%;
      color: var(--mr-surface);

      .mat-icon {
        width: calc(var(--dm) * 0.6);
        height: calc(var(--dm) * 0.6);
        font-size: calc(var(--dm) * 0.6);
      }
    }

    .mark--on {
      border-color: var(--mr-ink);
      background: var(--mr-ink);
    }

    // On the master's row of a character about to die (E6-30) the marks take
    // the colours too, besides their shapes.
    :host-context(.on-danger) .mark--on {
      border-color: var(--mr-success-ink);
      background: var(--mr-success-ink);
      color: var(--mr-danger-surface);
    }

    :host-context(.on-danger) .mark--on.mark--fail {
      border-color: var(--mr-danger-ink);
      background: var(--mr-danger-ink);
    }
  `,
  host: { 'aria-hidden': 'true', '[style.--dm.px]': 'size()' },
})
export class DeathMarks {
  readonly kind = input.required<'success' | 'failure'>();
  readonly count = input.required<number>();
  /** The disc's diameter in pixels: 40 on the player's page, 26 in the order. */
  readonly size = input(40);

  protected readonly dots = computed(() => marks(this.count()));
}

/** The two rows of marks on the master's order, side by side: successes then failures (E6-30). With the table's rule "só o dono e o mestre"
 * (RN-24) they say who else sees them, in the words and the eye-off icon of the player's own card (E10-04 state 6). */
@Component({
  selector: 'app-death-row',
  imports: [DeathMarks, MatIconModule],
  template: `
    @if (ownerOnly()) {
      <span class="cap">Testes contra a morte</span>
      <span class="pill"><mat-icon aria-hidden="true">visibility_off</mat-icon>Dono e mestre</span>
    }
    <span class="grp">
      <span class="lbl">Sucessos {{ successes() }} de 3</span>
      <app-death-marks kind="success" [count]="successes()" [size]="24" />
    </span>
    <span class="grp">
      <span class="lbl">Falhas {{ failures() }} de 3</span>
      <app-death-marks kind="failure" [count]="failures()" [size]="24" />
    </span>
  `,
  styles: `
    :host {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: 4px 12px;
      margin-top: 4px;
    }

    .grp {
      display: inline-flex;
      align-items: center;
      gap: 8px;
    }

    .lbl {
      font-size: 14px;
      color: var(--mr-ink-muted);
      white-space: nowrap;
    }

    .cap {
      flex: 1 0 100%;
      font-size: 14px;
      font-weight: 700;
    }

    .pill {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      box-sizing: border-box;
      height: 26px;
      padding: 0 10px 0 8px;
      border: 1px solid var(--mr-control-line);
      border-radius: var(--mr-radius-pill);
      color: var(--mr-ink-muted);
      font-size: 14px;
      font-weight: 700;
      line-height: 1;
      white-space: nowrap;
    }

    .pill .mat-icon {
      flex: none;
      width: 16px;
      height: 16px;
      font-size: 16px;
    }
  `,
})
export class DeathRow {
  readonly successes = input.required<number>();
  readonly failures = input.required<number>();
  /** Only the owner and the master see the marks. */
  readonly ownerOnly = input(false);
}
