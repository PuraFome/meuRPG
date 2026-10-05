import { Component, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/**
 * "Na forma de Lobo" (MR-037, E9-11 state 3): the band of the turn card while the druid is a beast. A paw
 * and words, never only a colour. Under it, what changes: no spells, the beast's armor class and its speed.
 */
@Component({
  selector: 'app-wild-band',
  imports: [MatIconModule],
  template: `
    <mat-icon class="band__icon" aria-hidden="true">pets</mat-icon>
    <span class="band__text">
      <b class="band__title">Na forma de {{ beast() }}</b>
      @if (detail()) {
        <span class="band__sub">{{ detail() }}</span>
      }
    </span>
  `,
  styles: `
    :host {
      display: flex;
      align-items: center;
      gap: 12px;
      box-sizing: border-box;
      padding: 10px 14px;
      border: 1px solid var(--mr-control-line);
      border-radius: var(--mr-radius-md);
      background: var(--mr-surface);
    }

    .band__icon {
      flex: none;
      width: 24px;
      height: 24px;
      font-size: 24px;
      color: var(--mr-ink);
    }

    .band__text {
      display: flex;
      flex-direction: column;
      min-width: 0;
    }

    .band__title {
      font-family: var(--mr-font-display);
      font-size: 19px;
      line-height: 23px;
    }

    .band__sub {
      font-size: 14px;
      line-height: 19px;
      color: var(--mr-ink-muted);
    }
  `,
})
export class WildBand {
  /** The beast's Portuguese name ("Lobo"). */
  readonly beast = input.required<string>();
  /** "Sem magias · CA 13 · 12,0 m". */
  readonly detail = input('');
}
