import { Component, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { LevelUpDone } from '../../../core/levelup/levelup-flow';

/**
 * "Pensantus subiu para o nível 4. O mestre foi avisado." (MR-040): the status the sheet shows after
 * the guided level-up, a polite live region the player dismisses. It is not stored: it lives in the
 * navigation the level-up page made, so a reload of the sheet does not bring it back.
 */
@Component({
  selector: 'app-level-up-done',
  imports: [MatIconModule],
  template: `
    <div class="mr-notice mr-notice--success done" role="status">
      <mat-icon aria-hidden="true">check</mat-icon>
      <p><strong>{{ done().name }} subiu para o nível {{ done().level }}.</strong> O mestre foi avisado.</p>
      <button type="button" class="done__close" aria-label="Dispensar o aviso" (click)="dismissed.emit()">
        <mat-icon aria-hidden="true">close</mat-icon>
      </button>
    </div>
  `,
  styles: `
    :host {
      display: block;
    }

    .done {
      align-items: center;
    }

    .done p {
      flex: 1 1 auto;
      text-wrap: balance;
    }

    .done__close {
      flex: none;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: 44px;
      height: 44px;
      margin: -8px -10px -8px 0;
      padding: 0;
      border: 0;
      border-radius: 50%;
      background: none;
      color: inherit;
      cursor: pointer;
    }
  `,
})
export class LevelUpDoneNotice {
  readonly done = input.required<LevelUpDone>();
  readonly dismissed = output<void>();
}
