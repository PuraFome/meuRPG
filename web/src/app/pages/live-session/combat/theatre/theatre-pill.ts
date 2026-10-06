import { Component } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { THEATRE_PILL } from '../../../../core/combat/theatre';

/**
 * "Teatro da mente": the pill that says a combat is played without a map (E10-04 state 2, review C23). A neutral
 * outline with the place-off icon and the word, never colour alone. It rides in the combat bar of the master, under
 * the name of whoever is next.
 */
@Component({
  selector: 'app-theatre-pill',
  imports: [MatIconModule],
  template: `<mat-icon aria-hidden="true">location_off</mat-icon>{{ word }}`,
  styles: `
    :host {
      display: inline-flex;
      align-items: center;
      align-self: flex-start;
      gap: 6px;
      box-sizing: border-box;
      min-height: 26px;
      padding: 0 12px 0 8px;
      border: 1px solid var(--mr-control-line);
      border-radius: var(--mr-radius-pill);
      color: var(--mr-ink-muted);
      font-size: 14px;
      font-weight: 700;
      line-height: 18px;
      white-space: nowrap;
    }

    .mat-icon {
      flex: none;
      width: 18px;
      height: 18px;
      font-size: 18px;
    }
  `,
})
export class TheatrePill {
  protected readonly word = THEATRE_PILL;
}
