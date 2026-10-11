import { ChangeDetectionStrategy, Component, inject, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { TurnAlert } from '../../../core/combat/turn-alert';

/**
 * "O mestre está chamando você." (the master's "Chamar a atenção", seen by the called player): a polite status near the top
 * of the session, in and out of combat. It shakes with the same half second as the turn panel (a still outline under reduced
 * motion), and a tap on the close button dismisses it; the page also dismisses it after a few seconds. The live region is
 * always in the page, so the text that appears in it is announced; it does not move the focus.
 */
@Component({
  selector: 'app-calling-notice',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatIconModule],
  template: `
    <div class="live" role="status" aria-live="polite">
      @if (on()) {
        <div
          class="mr-notice mr-notice--warning"
          [class.mr-nudge]="alert.shaking()"
          [class.mr-nudge--still]="alert.highlighted()"
        >
          <mat-icon aria-hidden="true">notifications_active</mat-icon>
          <p class="text"><strong>O mestre está chamando você.</strong></p>
          <button type="button" class="close" aria-label="Dispensar o aviso" (click)="dismissed.emit()">
            <mat-icon aria-hidden="true">close</mat-icon>
          </button>
        </div>
      }
    </div>
  `,
  styles: `
    :host {
      display: block;
    }

    .live:empty {
      display: none;
    }

    .text {
      flex: 1;
      min-width: 0;
      margin: 0;
    }

    .close {
      display: inline-flex;
      flex: none;
      align-items: center;
      justify-content: center;
      width: 44px;
      height: 44px;
      margin: -12px -14px -12px 0;
      border: 0;
      background: transparent;
      color: inherit;
      cursor: pointer;
    }
  `,
})
export class CallingNotice {
  protected readonly alert = inject(TurnAlert);
  /** The notice is on: the stream said the master is calling this player. */
  readonly on = input(false);
  readonly dismissed = output<void>();
}
