import { Component, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/**
 * "Da mesa", "Arquivada" and "Desligada para os jogadores"  beside the name of an entry of the table's own
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
      <span class="mr-tag"><mat-icon aria-hidden="true">inventory_2</mat-icon>{{ archivedWord() }}</span>
    }
    @if (off()) {
      <span class="mr-tag"><mat-icon aria-hidden="true">visibility_off</mat-icon>Desligada para os jogadores</span>
    }
  `,
  styles: `
    :host {
      display: inline-flex;
      flex-wrap: wrap;
      gap: var(--mr-space-1);
      vertical-align: middle;
    }

    // The tag keeps its words on one line, in the open list too.
    .mr-tag {
      white-space: nowrap;
    }

    // Inside a select's option Material gives every icon a right margin of its own: the tag's icon sits against its word.
    .mr-tag .mat-icon {
      margin: 0;
    }
  `,
})
export class TableMark {
  readonly fromTable = input(false);
  readonly archived = input(false);
  /** The master switched it off for the players: only the master can still pick it, and the sheet that has it keeps it. */
  readonly off = input(false);
  /** "Arquivada", or "Arquivado" for a word that is masculine (an antecedente). */
  readonly masculine = input(false);
  protected archivedWord(): string {
    return this.masculine() ? 'Arquivado' : 'Arquivada';
  }
}
