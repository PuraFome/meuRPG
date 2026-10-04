import { Component, ViewEncapsulation, computed, input, output } from '@angular/core';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatSelectModule } from '@angular/material/select';

import { formatInt } from '../../core/format/text';

/** One choice of the select, with an optional count on the right. */
export interface SelectOption {
  readonly value: string;
  readonly label: string;
  readonly count?: number;
}

let nextId = 0;

/**
 * The notes' two "Cena" fields (E8-06, E8-07): the filter of the list ("Todas
 * as anotações · 4", each discovered scene with its count, "Sem cena") and the
 * scene tag of a note ("Sem cena" and the discovered scenes). An Angular
 * Material select (arrows, Enter and Esc work); rows of 48px, the chosen one
 * with a check and in bold. The parent owns the value and says what changes.
 */
@Component({
  selector: 'app-notes-select',
  imports: [MatFormFieldModule, MatSelectModule],
  template: `
    <mat-form-field appearance="outline" subscriptSizing="dynamic">
      <mat-label>{{ label() }}</mat-label>
      <mat-select
        panelClass="mr-notes-panel"
        disableOptionCentering
        [value]="value()"
        [attr.aria-describedby]="hint() ? id + '-hint' : null"
        (selectionChange)="valueChange.emit($event.value)"
      >
        <mat-select-trigger>{{ trigger() }}</mat-select-trigger>
        @for (o of options(); track o.value) {
          <mat-option [value]="o.value" class="notes-option">
            <span class="notes-option__row">
              <span class="notes-option__name">{{ o.label }}</span>
              @if (o.count !== undefined) {
                <span class="notes-option__count">{{ count(o.count) }}</span>
              }
            </span>
          </mat-option>
        }
      </mat-select>
    </mat-form-field>
    @if (hint(); as text) {
      <p class="notes-hint" [id]="id + '-hint'">{{ text }}</p>
    }
  `,
  // Not encapsulated: the select's panel is drawn in the overlay, outside this
  // component, and its rules (rows of 48px, the count on the right, the chosen
  // row in bold) live here, in the lazy chunk, instead of in the global styles.
  encapsulation: ViewEncapsulation.None,
  styles: `
    app-notes-select {
      display: block;
    }

    app-notes-select mat-form-field {
      width: 100%;
    }

    app-notes-select .notes-hint {
      margin: 4px 0 0 16px;
      font-size: 14px;
      line-height: 19px;
      color: var(--mr-ink-muted);
    }

    // Under the field, same edges, and opaque (the list behind never shows through).
    .mr-notes-panel.mat-mdc-select-panel {
      background: var(--mr-surface);
    }

    .mr-notes-panel.mat-mdc-select-panel .notes-option {
      min-height: 48px;
    }

    .mr-notes-panel .mdc-list-item__primary-text {
      flex: 1;
    }

    .mr-notes-panel .notes-option__row {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: var(--mr-space-3);
    }

    .mr-notes-panel .notes-option__name {
      overflow-wrap: anywhere;
    }

    .mr-notes-panel .notes-option__count {
      flex: none;
      font-size: 14px;
      color: var(--mr-ink-muted);
    }

    .mr-notes-panel .mdc-list-item--selected .notes-option__name {
      font-weight: 700;
    }
  `,
})
export class NotesSelect {
  readonly label = input.required<string>();
  readonly options = input.required<readonly SelectOption[]>();
  readonly value = input('');
  readonly hint = input('');
  readonly valueChange = output<string>();

  protected readonly id = `ns-${nextId++}`;
  protected readonly count = (n: number) => formatInt(n);
  /** The closed field: the choice's name and, for the filter, its count. */
  protected readonly trigger = computed(() => {
    const o = this.options().find((x) => x.value === this.value());
    return o ? (o.count === undefined ? o.label : `${o.label} · ${formatInt(o.count)}`) : '';
  });
}
