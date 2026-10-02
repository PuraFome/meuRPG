import { Component, input, model } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/**
 * The editor's switch "Revelado aos jogadores" (README-B): not filled with
 * the accent, because only the primary button is. On: an `ink` track, a
 * `surface` handle and a check. Off: an outlined `control-line` track with
 * a small handle. A real `role="switch"` button, named by its label and
 * described by the hint.
 */
@Component({
  selector: 'app-reveal-switch',
  imports: [MatIconModule],
  template: `
    <div class="sw">
      <span class="sw__text">
        <span class="sw__label" id="sw-label">{{ label() }}</span>
        <span class="sw__hint" id="sw-hint">{{ hint() }}</span>
      </span>
      <button
        type="button"
        role="switch"
        class="sw__track"
        [class.sw__track--on]="checked()"
        [attr.aria-checked]="checked()"
        aria-labelledby="sw-label"
        aria-describedby="sw-hint"
        (click)="checked.set(!checked())"
      >
        <span class="sw__handle">
          @if (checked()) {
            <mat-icon aria-hidden="true">check</mat-icon>
          }
        </span>
      </button>
    </div>
  `,
  styleUrl: './reveal-switch.scss',
})
export class RevealSwitch {
  readonly label = input.required<string>();
  readonly hint = input('');
  readonly checked = model(false);
}
