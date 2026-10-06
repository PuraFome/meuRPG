import { Component, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/**
 * The line under a field: what the server refused, each reason with the alert icon in the danger ink (never colour alone),
 * or, when nothing is wrong, the hint. The field points `aria-describedby` at `id`.
 */
@Component({
  selector: 'app-field-note',
  imports: [MatIconModule],
  template: `
    @if (issues().length > 0) {
      <!-- One id for the whole group: the field points aria-describedby at it, and every message is read. -->
      <div [id]="id()">
        @for (m of issues(); track m) {
          <p class="note note--issue">
            <mat-icon aria-hidden="true">warning</mat-icon>
            <span>{{ m }}</span>
          </p>
        }
      </div>
    } @else if (hint()) {
      <p class="note" [id]="id()">{{ hint() }}</p>
    }
  `,
  styles: `
    :host {
      display: block;
    }

    .note {
      display: flex;
      align-items: flex-start;
      gap: 6px;
      margin: 4px 0 0;
      padding: 0 4px;
      font-size: 14px;
      line-height: 18px;
      color: var(--mr-ink-muted);
    }

    .note--issue {
      color: var(--mr-danger-ink);
    }

    .mat-icon {
      flex: none;
      width: 16px;
      height: 16px;
      font-size: 16px;
      margin-top: 1px;
    }
  `,
})
export class FieldNote {
  readonly id = input('');
  readonly issues = input<readonly string[]>([]);
  readonly hint = input('');
}
