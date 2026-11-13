import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/**
 * "Na forma de Lobo · ⧗ 52 min": the one chip of the turn band while the druid is a beast (584-01, 584-02). Whoever may read the
 * time (the master and the owner) gets it after the name; the others read the name alone. The time is the server's words
 * ("52 min restantes", "Restam 4 rodadas"): the chip only drops the lead or the tail that its hourglass already says.
 */
@Component({
  selector: 'app-form-chip',
  imports: [MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <mat-icon class="chip__icon" aria-hidden="true">pets</mat-icon>
    <b>Na forma de {{ beast() }}</b>
    @if (short(); as t) {
      <span class="chip__time"><mat-icon class="chip__hour" aria-hidden="true">hourglass_empty</mat-icon>{{ t }}</span>
    }
  `,
  styles: `
    :host {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      box-sizing: border-box;
      min-height: 28px;
      padding: 2px 10px;
      border: 1px solid var(--mr-control-line);
      border-radius: 999px;
      background: var(--mr-surface);
      font-size: var(--mr-font-13);
      line-height: 18px;
      color: var(--mr-ink);
    }

    .chip__icon {
      flex: none;
      width: 16px;
      height: 16px;
      font-size: var(--mr-font-16);
    }

    .chip__time {
      display: inline-flex;
      align-items: center;
      gap: 2px;
      font-weight: 700;

      &::before {
        content: '·';
        margin-right: 4px;
        font-weight: 400;
      }
    }

    .chip__hour {
      width: 14px;
      height: 14px;
      font-size: var(--mr-font-14);
    }
  `,
})
export class FormChip {
  /** The beast's Portuguese name ("Lobo"). */
  readonly beast = input.required<string>();
  /** The time left as the server writes it; empty for a reader who may not have it. */
  readonly time = input('');

  protected readonly short = computed(() =>
    this.time()
      .replace(/ restantes$/, '')
      .replace(/^Restam /, ''),
  );
}
