import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/**
 * "O mestre deu Inspiração a você. Use antes de rolar um d20 seu." (the player whose character was just given it, board 581, states
 * item 3): a polite status that does not move the focus; the other players read nothing (RN-10). The live region is always in the
 * page, so the text that appears in it is announced.
 */
@Component({
  selector: 'app-favor-notice',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatIconModule],
  template: `
    <div class="live" role="status" aria-live="polite">
      @if (on()) {
        <div class="mr-notice mr-notice--success">
          <mat-icon class="mr-icon-filled" aria-hidden="true">star</mat-icon>
          <p class="text">O mestre deu Inspiração a você. Use antes de rolar um d20 seu.</p>
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
export class FavorNotice {
  /** The stream said this player's character was given it. */
  readonly on = input(false);
  readonly dismissed = output<void>();
}
