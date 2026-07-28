import { Component, Input } from '@angular/core';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';

@Component({
  selector: 'app-loading-spinner',
  standalone: true,
  imports: [MatProgressSpinnerModule],
  template: `
    @if (isLoading) {
      <div class="spinner-container">
        <mat-progress-spinner
          mode="indeterminate"
          diameter="48"
        />
        @if (message) {
          <p class="spinner-message">{{ message }}</p>
        }
      </div>
    }
  `,
  styles: [
    `
      .spinner-container {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        padding: 32px 16px;
      }
      .spinner-message {
        margin: 16px 0 0;
        font-size: 0.9rem;
        opacity: 0.6;
      }
    `,
  ],
})
export class LoadingSpinnerComponent {
  @Input() isLoading = false;
  @Input() message?: string;
}
