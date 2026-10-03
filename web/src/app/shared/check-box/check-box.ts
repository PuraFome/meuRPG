import { Component, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/**
 * The app's checkbox for a list of choices (who joins the combat, who gets
 * the XP): a 24px box in a 44px target, with a real `<input type="checkbox">`
 * underneath and the box drawn over it. `label` is what a screen reader
 * hears ("Incluir Pensantus"); the person's name next to it is the row's
 * own text. The parent owns the state: `toggle` says the person tapped.
 */
@Component({
  selector: 'app-check-box',
  imports: [MatIconModule],
  template: `
    <label class="check">
      <input
        type="checkbox"
        class="box"
        [checked]="checked()"
        [disabled]="disabled()"
        (change)="toggle.emit()"
      />
      <span class="mark" aria-hidden="true"><mat-icon>check</mat-icon></span>
      <span class="mr-visually-hidden">{{ label() }}</span>
    </label>
  `,
  styles: `
    :host {
      display: inline-flex;
    }

    .check {
      position: relative;
      display: flex;
      align-items: center;
      justify-content: center;
      width: 24px;
      height: 24px;
    }

    .box {
      position: absolute;
      inset: -10px;
      margin: 0;
      opacity: 0;
      cursor: pointer;
    }

    .mark {
      display: flex;
      align-items: center;
      justify-content: center;
      box-sizing: border-box;
      width: 24px;
      height: 24px;
      border: 2px solid var(--mr-control-line);
      border-radius: 5px;
      color: transparent;
      pointer-events: none;

      .mat-icon {
        width: 18px;
        height: 18px;
        font-size: 18px;
      }
    }

    .box:checked + .mark {
      border-color: var(--mr-ink);
      background: var(--mr-ink);
      color: var(--mr-surface);
    }

    .box:focus-visible + .mark {
      outline: 2px solid var(--mr-focus);
      outline-offset: 2px;
    }
  `,
})
export class CheckBox {
  readonly checked = input(false);
  readonly disabled = input(false);
  /** What a screen reader hears for the box. */
  readonly label = input.required<string>();

  readonly toggle = output<void>();
}
