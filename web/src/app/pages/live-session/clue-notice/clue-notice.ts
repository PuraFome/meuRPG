import { Component, computed, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { Note } from '../../../../gen/meurpg/notes/v1/notes_pb';

/**
 * "O mestre revelou uma pista para você" (E8-06): the notice a player gets on
 * the session page the moment a clue arrives, with "Abrir anotações" in words
 * (no waiting for the person to find the button) and a ✕ of 44px. It stays
 * until it is opened or dismissed: it never goes away by itself, so a person
 * looking at the table does not miss it. The live region is always in the
 * page, so a screen reader hears it when it fills.
 */
@Component({
  selector: 'app-clue-notice',
  imports: [MatButtonModule, MatIconModule],
  template: `
    <div role="status" aria-live="polite">
      @if (clues().length > 0) {
        <div class="mr-notice mr-notice--neutral cn">
          <mat-icon aria-hidden="true">search</mat-icon>
          <div class="cn__body">
            <p>
              <strong>{{ title() }}</strong>
              {{ where() }}
            </p>
            <button matButton type="button" class="cn__open" (click)="opened.emit()">Abrir anotações</button>
          </div>
          <button type="button" class="cn__close" aria-label="Dispensar o aviso" (click)="dismissed.emit()">
            <mat-icon aria-hidden="true">close</mat-icon>
          </button>
        </div>
      }
    </div>
  `,
  styleUrl: './clue-notice.scss',
})
export class ClueNotice {
  /** The clues that arrived and have not been opened yet. */
  readonly clues = input.required<readonly Note[]>();
  readonly opened = output<void>();
  readonly dismissed = output<void>();

  protected readonly title = computed(() => {
    const n = this.clues().length;
    return n === 1 ? 'O mestre revelou uma pista para você.' : `O mestre revelou ${n} pistas para você.`;
  });
  protected readonly where = computed(() => {
    const n = this.clues().length;
    const tagged = this.clues().every((c) => c.sceneName !== '');
    if (n === 1) {
      return tagged ? 'Ela está em Anotações, com a cena marcada.' : 'Ela está em Anotações.';
    }
    return tagged ? 'Elas estão em Anotações, com a cena marcada.' : 'Elas estão em Anotações.';
  });
}
