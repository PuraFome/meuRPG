import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { PendingSend } from '../../../core/conversation/conversation-state';

const LEAD = 'Não foi enviada.';

/**
 * The reader's own line while it goes, and after it did not: it stays on screen as sent, and a
 * danger notice says why with "Tentar de novo", which sends it again with the same key so a line
 * that did get through is not stored twice.
 */
@Component({
  selector: 'li[app-pending-line]',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatIconModule],
  host: { class: 'pending', '[attr.aria-busy]': "pending().status === 'sending' ? 'true' : null" },
  template: `
    <span class="line__label">{{ label() }}</span>
    <p class="line__bubble" [class.line__bubble--sending]="pending().status === 'sending'">{{ pending().text }}</p>
    @if (pending().status === 'failed') {
      <div class="notice" role="alert">
        <mat-icon aria-hidden="true">error</mat-icon>
        <span><strong>{{ lead }}</strong> {{ reason() }}</span>
        <button type="button" class="notice__retry" (click)="retry.emit()">Tentar de novo</button>
      </div>
    }
  `,
  styleUrl: './pending-line.scss',
})
export class PendingLine {
  readonly pending = input.required<PendingSend>();
  /** "Pensantus · você". */
  readonly label = input.required<string>();
  readonly retry = output<void>();

  protected readonly lead = LEAD;
  protected readonly reason = computed(() =>
    this.pending().error.replace(LEAD, '').trim(),
  );
}
