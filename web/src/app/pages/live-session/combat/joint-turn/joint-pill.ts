import { Component, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/** The pill of the member's turn, "Turno conjunto com Brisa" (E8-01): the word
 * and a link icon in a 1,5px accent outline, announced when the turn opens. */
@Component({
  selector: 'app-joint-pill',
  imports: [MatIconModule],
  template: `<mat-icon aria-hidden="true">link</mat-icon>{{ text() }}`,
  styles: `
    :host {
      display: inline-flex;
      align-items: center;
      align-self: flex-start;
      gap: 6px;
      box-sizing: border-box;
      min-height: 28px;
      padding: 0 12px 0 8px;
      border: 1.5px solid var(--mr-accent);
      border-radius: var(--mr-radius-pill);
      color: var(--mr-accent-text);
      font-size: 14px;
      font-weight: 700;
      line-height: 18px;
    }

    .mat-icon {
      flex: none;
      width: 18px;
      height: 18px;
      font-size: 18px;
    }
  `,
  host: { role: 'status' },
})
export class JointPill {
  readonly text = input.required<string>();
}
