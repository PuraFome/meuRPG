import { Component, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/**
 * Where a member's part of a joint turn stands (E8-01): "Ainda age" with an
 * open circle, or "Encerrou" with a check. A word and an icon, never a colour
 * alone; the pill is for the master, the member and everyone at the table who
 * is told about the group (RN-20).
 */
@Component({
  selector: 'app-part-state',
  imports: [MatIconModule],
  template: `<mat-icon aria-hidden="true">{{ ended() ? 'check_circle' : 'radio_button_unchecked' }}</mat-icon
    >{{ ended() ? 'Encerrou' : 'Ainda age' }}`,
  styles: `
    :host {
      display: inline-flex;
      flex: none;
      align-items: center;
      gap: 4px;
      box-sizing: border-box;
      height: 24px;
      padding: 0 10px 0 6px;
      border: 1.5px solid var(--mr-accent);
      border-radius: var(--mr-radius-pill);
      color: var(--mr-accent-text);
      font-size: 13px;
      font-weight: 700;
      line-height: 1;
      white-space: nowrap;
    }

    :host(.ended) {
      border-color: var(--mr-ink);
      color: var(--mr-ink);
    }

    .mat-icon {
      width: 16px;
      height: 16px;
      font-size: 16px;
    }
  `,
  host: { '[class.ended]': 'ended()' },
})
export class PartState {
  readonly ended = input(false);
}
