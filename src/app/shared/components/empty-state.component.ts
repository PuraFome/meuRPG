import { Component, EventEmitter, Input, Output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

@Component({
  selector: 'app-empty-state',
  standalone: true,
  imports: [MatIconModule, MatButtonModule],
  template: `
    <div class="empty-state">
      <mat-icon class="empty-icon">{{ icon }}</mat-icon>
      <p class="empty-message">{{ message }}</p>
      @if (actionLabel) {
        <button
          mat-raised-button
          color="primary"
          (click)="action.emit()"
        >
          {{ actionLabel }}
        </button>
      }
    </div>
  `,
  styles: [
    `
      .empty-state {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        padding: 48px 16px;
        text-align: center;
      }
      .empty-icon {
        font-size: 3rem;
        width: 3rem;
        height: 3rem;
        margin-bottom: 16px;
        opacity: 0.4;
      }
      .empty-message {
        font-size: 1rem;
        margin: 0 0 16px;
        opacity: 0.6;
        line-height: 1.5;
        max-width: 320px;
      }
    `,
  ],
})
export class EmptyStateComponent {
  @Input() icon = 'inbox';
  @Input() message = '';
  @Input() actionLabel?: string;
  @Output() action = new EventEmitter<void>();
}
