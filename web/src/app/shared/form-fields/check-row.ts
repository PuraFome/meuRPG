import { Component, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/**
 * A checkbox with its words beside it, the whole row a 44 px target: a real checkbox under a 24 px box that shows a check
 * (the accent only when it is checked, and the check itself says so). `<ng-content>` is anything after the words, such as the
 * "Da mesa" tag.
 */
@Component({
  selector: 'app-check-row',
  imports: [MatIconModule],
  template: `
    <label class="row">
      <input type="checkbox" class="mr-visually-hidden" [checked]="checked()" [disabled]="disabled()" [attr.data-field]="path() || null" (change)="toggled.emit(!checked())" />
      <span class="box" aria-hidden="true"><mat-icon>check</mat-icon></span>
      <span class="label">{{ label() }}</span>
      <ng-content />
    </label>
  `,
  styles: `
    :host {
      display: block;
    }

    .row {
      display: flex;
      align-items: center;
      gap: var(--mr-space-3);
      min-height: 44px;
      cursor: pointer;
    }

    .box {
      flex: none;
      display: flex;
      align-items: center;
      justify-content: center;
      box-sizing: border-box;
      width: 24px;
      height: 24px;
      border: 2px solid var(--mr-control-line);
      border-radius: 5px;
      background: var(--mr-surface);
      color: transparent;

      .mat-icon {
        width: 18px;
        height: 18px;
        font-size: 18px;
      }
    }

    input:checked + .box {
      border-color: var(--mr-accent);
      background: var(--mr-accent);
      color: var(--mr-on-accent);
    }

    input:focus-visible + .box {
      outline: 2px solid var(--mr-focus);
      outline-offset: 2px;
    }

    .label {
      font-size: 17px;
    }
  `,
})
export class CheckRow {
  readonly label = input.required<string>();
  readonly checked = input.required<boolean>();
  readonly disabled = input(false);
  readonly path = input('');
  readonly toggled = output<boolean>();
}
