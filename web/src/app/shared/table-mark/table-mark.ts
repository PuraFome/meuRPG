import { Component, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/**
 * "Da mesa" (and "Arquivada" for the master) beside the name of an entry of the table's own
 * content (RN-23): the same tag the "Magias" page uses, a word and an icon, never colour alone.
 * Inline, so it sits inside a select option, a list row or a sentence.
 */
@Component({
  selector: 'app-table-mark',
  imports: [MatIconModule],
  template: `
    @if (fromTable()) {
      <span class="mr-tag"><mat-icon aria-hidden="true">menu_book</mat-icon>Da mesa</span>
    }
    @if (archived()) {
      <span class="mr-tag"><mat-icon aria-hidden="true">inventory_2</mat-icon>Arquivada</span>
    }
  `,
  styles: `
    :host {
      display: inline-flex;
      flex-wrap: wrap;
      gap: var(--mr-space-1);
      vertical-align: middle;
    }
  `,
})
export class TableMark {
  readonly fromTable = input(false);
  readonly archived = input(false);
}
