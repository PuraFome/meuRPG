import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { ReturnPlace } from '../../../core/revivify/revivify-flow';

/**
 * "Você voltou à vida" (the revived character's own player): a polite status that does not move the focus. It names who
 * cast Revivificar only when the table's log says it, and says where the character returns in the initiative order and
 * when it next acts only from a combat the player sees. It never shows another player's numbers or the 10-round window.
 * The live region is always in the page, so the text that appears in it is announced.
 */
@Component({
  selector: 'app-revived-notice',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatIconModule],
  template: `
    <div class="live" role="status" aria-live="polite">
      @if (on()) {
        <div class="mr-notice mr-notice--success">
          <mat-icon aria-hidden="true">favorite</mat-icon>
          <div class="text">
            <p>
              <strong>Você voltou à vida.</strong>
              {{ caster() ? caster() + ' usou Revivificar em você. ' : '' }}Está com 1 PV.
            </p>
            @if (place(); as p) {
              <p>
                Você volta à ordem de iniciativa onde estava{{ p.between ? ' (' + p.between + ')' : '' }}. O seu próximo
                turno é na rodada {{ p.round }}.
              </p>
            }
          </div>
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
      display: flex;
      flex: 1;
      flex-direction: column;
      gap: 6px;
      min-width: 0;

      p {
        margin: 0;
      }
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
export class RevivedNotice {
  /** The notice is on: the stream said this player's character lives again. */
  readonly on = input(false);
  /** Who cast the spell, when the log says; empty otherwise. */
  readonly caster = input('');
  /** Where the character returns and when it acts, only from a combat the player sees. */
  readonly place = input<ReturnPlace | null>(null);
  readonly dismissed = output<void>();
}
