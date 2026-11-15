import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { FieldNote } from './field-note';

let nextId = 0;

export interface ToggleChip {
  readonly value: string;
  readonly label: string;
  /** Shut: it stays on screen, dashed, and cannot be chosen; `lockNote` says why. */
  readonly locked?: boolean;
}

/**
 * A closed list shown as chips to switch on and off (the weapon's properties, board 580-11). A chosen chip shows a check; a
 * shut one a lock and a dashed edge, and the one line under the chips (`lockNote`) is its reason, read with each shut chip.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-toggle-chips',
  imports: [FieldNote, MatIconModule],
  template: `
    <div class="tchips" role="group" [attr.aria-label]="label()" [attr.data-field]="path() || null">
      @for (c of chips(); track c.value) {
        <button
          type="button"
          class="tchip"
          [class.tchip--on]="isOn(c.value)"
          [class.tchip--locked]="c.locked"
          [attr.aria-pressed]="isOn(c.value)"
          [attr.aria-disabled]="c.locked ? 'true' : null"
          [attr.aria-describedby]="c.locked && lockNote() ? noteId : null"
          (click)="toggle(c)"
        >
          @if (c.locked) {
            <mat-icon aria-hidden="true">lock</mat-icon>
          } @else if (isOn(c.value)) {
            <mat-icon aria-hidden="true">check</mat-icon>
          }
          {{ c.label }}
        </button>
      }
    </div>
    @if (lockNote()) {
      <p class="tchips__note" [id]="noteId"><mat-icon aria-hidden="true">lock</mat-icon>{{ lockNote() }}</p>
    }
    <app-field-note [issues]="issues()" [hint]="hint()" />
  `,
  styles: `
    :host {
      display: block;
    }

    .tchips {
      display: flex;
      flex-wrap: wrap;
      gap: var(--mr-space-2);
    }

    .tchip {
      display: inline-flex;
      align-items: center;
      gap: 4px;
      min-height: 44px;
      padding: 0 var(--mr-space-3);
      border: 1px solid var(--mr-control-line);
      border-radius: var(--mr-radius-pill);
      background: var(--mr-surface);
      color: var(--mr-ink);
      font: inherit;
      cursor: pointer;
    }

    .tchip mat-icon {
      width: 18px;
      height: 18px;
      font-size: 18px;
    }

    .tchip--on {
      border-color: var(--mr-accent);
      background: var(--mr-accent-soft);
      color: var(--mr-accent-strong);
      font-weight: 700;
    }

    .tchip:focus-visible {
      outline: 2px solid var(--mr-accent);
      outline-offset: 2px;
    }

    .tchip--locked {
      border-style: dashed;
      color: var(--mr-ink-muted);
      border-width: 1px;
      cursor: not-allowed;
    }

    .tchips__note mat-icon {
      flex: none;
      width: 16px;
      height: 16px;
      font-size: 16px;
    }

    .tchips__note {
      display: flex;
      align-items: center;
      gap: 6px;
      margin: var(--mr-space-2) 0 0;
      font-size: var(--mr-font-14);
      color: var(--mr-ink-muted);
    }
  `,
})
export class ToggleChips {
  readonly label = input.required<string>();
  readonly chips = input.required<readonly ToggleChip[]>();
  readonly values = input<readonly string[]>([]);
  /** The one line that says why each shut chip is shut. */
  readonly lockNote = input('');
  readonly path = input('');
  readonly issues = input<readonly string[]>([]);
  readonly hint = input('');
  readonly valuesChange = output<string[]>();

  protected readonly noteId = `tchips-note-${nextId++}`;

  protected isOn(value: string): boolean {
    return this.values().includes(value);
  }

  protected toggle(chip: ToggleChip): void {
    if (chip.locked) {
      return;
    }
    this.valuesChange.emit(
      this.isOn(chip.value)
        ? this.values().filter((v) => v !== chip.value)
        : [...this.values(), chip.value],
    );
  }
}
