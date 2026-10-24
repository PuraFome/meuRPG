import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

/**
 * What a rail panel says while it has nothing to show (C6-10): `loading` (a ring and "Lendo a conversa…", a polite status),
 * `error` (a danger notice with "Tentar de novo", an alert) and `empty` (an icon and the sentence). The words come from the
 * screen; this is the look, 120 px tall at least, centred.
 */
@Component({
  selector: 'app-rail-state',
  imports: [MatButtonModule, MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @switch (kind()) {
      @case ('loading') {
        <span class="rs__ring" aria-hidden="true"></span>
        <p class="rs__text" role="status">{{ text() }}</p>
      }
      @case ('error') {
        <div class="mr-notice mr-notice--danger rs__notice" role="alert">
          <mat-icon aria-hidden="true">error</mat-icon>
          <p>
            <strong>{{ text() }}</strong>
            @if (more()) {
              {{ ' ' + more() }}
            }
          </p>
        </div>
        <button matButton type="button" class="rs__retry" (click)="retry.emit()">
          <mat-icon aria-hidden="true">refresh</mat-icon>Tentar de novo
        </button>
      }
      @default {
        <mat-icon class="rs__icon" aria-hidden="true">{{ icon() }}</mat-icon>
        <p class="rs__text">{{ text() }}</p>
      }
    }
  `,
  styleUrl: './rail-state.scss',
})
export class RailState {
  readonly kind = input.required<'loading' | 'error' | 'empty'>();
  /** The sentence ("Lendo a conversa…"; for an error its bold first part, "Não deu para carregar a conversa."). */
  readonly text = input.required<string>();
  /** An error's second part ("Confira a conexão."). */
  readonly more = input('');
  /** The empty state's icon. */
  readonly icon = input('chat_bubble_outline');
  readonly retry = output<void>();
}
