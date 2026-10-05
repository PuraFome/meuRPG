import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { Toast } from '../../core/traps/toast-queue';

/**
 * The toasts of the session page (E9-08 G, E9-09 4): a green notice with an icon, a bold first phrase and a ✕
 * of 44 px, in a polite live region that is always in the page, so a screen reader hears each one when
 * it arrives. Presentational: `ToastQueue` decides when one comes and goes (8 seconds).
 */
@Component({
  selector: 'app-live-toast',
  imports: [MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div role="status" aria-live="polite" class="lt">
      @for (t of toasts(); track t.id) {
        <div class="mr-notice mr-notice--success lt__toast">
          <mat-icon aria-hidden="true">{{ t.icon }}</mat-icon>
          <p class="lt__text">
            <strong>{{ t.title }}</strong>
            @if (t.text) {
              <span class="lt__sub">{{ t.text }}</span>
            }
          </p>
          <button type="button" class="lt__close" aria-label="Dispensar o aviso" (click)="dismissed.emit(t.id)">
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

    .lt {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-2);
    }

    .lt__toast {
      align-items: flex-start;
      max-width: 640px;
    }

    .lt__toast:last-child {
      margin-bottom: var(--mr-space-4);
    }

    .lt__text {
      flex: 1 1 auto;
      min-width: 0;
    }

    .lt__sub {
      display: block;
    }

    // The ✕: a 44px target at the corner, easy to hit and never in the way.
    .lt__close {
      display: inline-flex;
      flex: none;
      align-items: center;
      justify-content: center;
      width: 44px;
      height: 44px;
      margin: -10px -12px -10px 0;
      padding: 0;
      border: 0;
      background: none;
      color: inherit;
      cursor: pointer;
    }

    .lt__close .mat-icon {
      width: 24px;
      height: 24px;
      font-size: 24px;
    }
  `,
})
export class LiveToast {
  readonly toasts = input.required<readonly Toast[]>();
  readonly dismissed = output<number>();
}
