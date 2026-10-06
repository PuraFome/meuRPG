import { Component, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

let nextId = 0;

/**
 * "Escondidos no início" (E10-08 state 4, E10-09 state 7): a real `role="switch"` named by its label and described by
 * the hint, with the word "Ligado" / "Desligado" and a check in the handle, so the state never rests on colour alone. The
 * parent owns the state.
 */
@Component({
  selector: 'app-hidden-switch',
  imports: [MatIconModule],
  template: `
    <div class="row">
      <button
        type="button"
        role="switch"
        class="track"
        [class.track--on]="checked()"
        [attr.aria-checked]="checked()"
        [disabled]="disabled()"
        [attr.aria-labelledby]="id + '-l'"
        [attr.aria-describedby]="id + '-h'"
        (click)="toggle.emit(!checked())"
      >
        <span class="handle">
          @if (checked()) {
            <mat-icon aria-hidden="true">check</mat-icon>
          }
        </span>
      </button>
      <span class="word" aria-hidden="true">{{ checked() ? 'Ligado' : 'Desligado' }}</span>
      <span class="text">
        <span class="label" [id]="id + '-l'">{{ label() }}</span>
        <span class="hint" [id]="id + '-h'">{{ hint() }}</span>
      </span>
    </div>
  `,
  styles: `
    .row {
      display: flex;
      align-items: center;
      gap: var(--mr-space-3);
      min-height: 48px;
    }

    // 52 x 32 track (the shipped switch), with a 44px-tall target around it.
    .track {
      position: relative;
      flex: none;
      box-sizing: border-box;
      width: 52px;
      height: 32px;
      padding: 0;
      border: 2px solid var(--mr-control-line);
      border-radius: var(--mr-radius-pill);
      background: none;
      cursor: pointer;

      &::before {
        content: '';
        position: absolute;
        inset: -6px -2px;
      }

      &:focus-visible {
        outline: 3px solid var(--mr-focus);
        outline-offset: 3px;
      }

      &--on {
        border-color: var(--mr-ink);
        background: var(--mr-ink);
      }

      &:disabled {
        cursor: default;
        opacity: 0.6;
      }
    }

    .handle {
      position: absolute;
      top: 50%;
      left: 4px;
      display: flex;
      align-items: center;
      justify-content: center;
      width: 16px;
      height: 16px;
      margin-top: -8px;
      border-radius: var(--mr-radius-pill);
      background: var(--mr-control-line);

      .track--on & {
        left: 22px;
        width: 24px;
        height: 24px;
        margin-top: -12px;
        background: var(--mr-surface);
        color: var(--mr-ink);
      }

      .mat-icon {
        width: 18px;
        height: 18px;
        font-size: 18px;
      }
    }

    .word {
      flex: none;
      min-width: 54px;
      font-size: 14px;
      font-weight: 700;
    }

    .text {
      display: flex;
      flex: 1 1 auto;
      flex-direction: column;
      min-width: 0;
    }

    .label {
      font-size: 16px;
      font-weight: 700;
      line-height: 20px;
    }

    .hint {
      font-size: 14px;
      line-height: 19px;
      color: var(--mr-ink-muted);
    }
  `,
})
export class HiddenSwitch {
  protected readonly id = `hs-${nextId++}`;
  readonly label = input.required<string>();
  readonly hint = input('');
  readonly checked = input(false);
  readonly disabled = input(false);
  readonly toggle = output<boolean>();
}
