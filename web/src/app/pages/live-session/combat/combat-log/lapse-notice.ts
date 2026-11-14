import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

/**
 * The notice above the log for a concentration that ended with the caster's next hit still waiting ("A sua concentração
 * acabou. O Clarão na Lâmina acabou sem golpe."). The status region is always there, so a screen reader hears the words
 * when they come; the words are the notice's own (icon and text, never colour alone), and it stays until it is touched.
 */
@Component({
  selector: 'app-lapse-notice',
  imports: [MatButtonModule, MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="live" role="status" aria-live="polite">
      @if (text()) {
        <div class="mr-notice mr-notice--warning notice" data-testid="lapse-notice">
          <div class="mr-notice__said">
            <mat-icon aria-hidden="true">hourglass_empty</mat-icon>
            <p>{{ text() }}</p>
          </div>
          <button matButton type="button" class="notice__ok" (click)="dismiss.emit()">
            Entendi
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

    .notice {
      flex-wrap: wrap;
      align-items: center;

      p {
        margin: 0;
      }
    }

    .notice__ok {
      --mat-button-text-label-text-color: var(--mr-accent-text);
      margin-right: -12px;
    }
  `,
})
export class LapseNoticeCard {
  readonly text = input('');
  readonly dismiss = output<void>();
}
