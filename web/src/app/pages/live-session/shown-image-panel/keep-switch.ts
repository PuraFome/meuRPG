import { Component, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/**
 * "Deixar com os jogadores" (E6-25, MR-028): the switch in the master's
 * "Imagem para os jogadores" panel. On, the image stays with the players
 * when the master stops showing it or swaps it. It is a real
 * `role="switch"` button named by its label and described by the hint, and
 * it never relies on colour alone: a check in the handle and the word
 * "Ligado" / "Desligado" say the state. The parent owns the state (the
 * server decides): this only asks.
 */
@Component({
  selector: 'app-keep-switch',
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
        aria-labelledby="keep-label"
        aria-describedby="keep-hint"
        (click)="toggle.emit(!checked())"
      >
        <span class="handle">
          @if (checked()) {
            <mat-icon aria-hidden="true">check</mat-icon>
          }
        </span>
      </button>
      <span class="text">
        <span class="label" id="keep-label">Deixar com os jogadores</span>
        <span class="hint" id="keep-hint">Fica com os jogadores ao parar ou trocar.</span>
      </span>
      <span class="state" [class.state--on]="checked()" aria-hidden="true">
        {{ checked() ? 'Ligado' : 'Desligado' }}
      </span>
    </div>
  `,
  styles: `
    .row {
      display: flex;
      align-items: center;
      gap: var(--mr-space-3);
      min-height: 56px;
      padding: 6px 0;
      border-block: 1px solid var(--mr-line);
    }

    // 52 x 28 track, with a 44px-tall target around it.
    .track {
      position: relative;
      flex: none;
      box-sizing: border-box;
      width: 52px;
      height: 28px;
      padding: 0;
      border: 2px solid var(--mr-control-line);
      border-radius: var(--mr-radius-pill);
      background: none;
      cursor: pointer;

      &::before {
        content: '';
        position: absolute;
        inset: -8px -2px;
      }

      &--on {
        border-color: var(--mr-accent);
        background: var(--mr-accent);
      }

      &:disabled {
        cursor: default;
        opacity: 0.6;
      }
    }

    .handle {
      position: absolute;
      top: 50%;
      left: 3px;
      display: flex;
      align-items: center;
      justify-content: center;
      width: 14px;
      height: 14px;
      margin-top: -7px;
      border-radius: var(--mr-radius-pill);
      background: var(--mr-control-line);

      .track--on & {
        left: 24px;
        width: 22px;
        height: 22px;
        margin-top: -11px;
        background: var(--mr-surface);
        color: var(--mr-accent-text);
      }

      .mat-icon {
        width: 16px;
        height: 16px;
        font-size: 16px;
      }
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

    .state {
      flex: none;
      font-size: 14px;
      font-weight: 700;
      color: var(--mr-ink-muted);

      &--on {
        color: var(--mr-accent-text);
      }
    }
  `,
})
export class KeepSwitch {
  readonly checked = input(false);
  readonly disabled = input(false);
  /** The new state the master asks for. */
  readonly toggle = output<boolean>();
}
