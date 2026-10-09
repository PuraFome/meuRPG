import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  effect,
  input,
  output,
  viewChild,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

/**
 * The master's question when he ends Ajuda on a combatant (PM-03a, state 4): in the row, in a box with the danger frame
 * because the cut in the hit points is not undone. It says the sum in words, offers "Encerrar Ajuda" (the outlined
 * irreversible action, not the filled accent one) and "Cancelar", and the focus opens on "Cancelar", the safe answer.
 * The two buttons stack on a phone (48px each).
 */
@Component({
  selector: 'app-end-aid-question',
  imports: [MatButtonModule, MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="box" role="alertdialog" [attr.aria-label]="question()">
      <mat-icon aria-hidden="true">favorite_border</mat-icon>
      <div class="ask">
        <p class="ask__q">{{ question() }}</p>
        <p class="ask__text">{{ text() }}</p>
        <div class="ask__buttons">
          <button mat-stroked-button type="button" class="danger" (click)="confirm.emit()">
            Encerrar Ajuda
          </button>
          <button #cancelButton mat-stroked-button type="button" (click)="declined.emit()">
            Cancelar
          </button>
        </div>
      </div>
    </div>
  `,
  styles: `
    :host {
      display: block;
    }

    .box {
      display: flex;
      align-items: flex-start;
      gap: var(--mr-space-3);
      margin-top: var(--mr-space-2);
      padding: var(--mr-space-3) var(--mr-space-4);
      border: 2px solid var(--mr-danger-ink);
      border-radius: var(--mr-radius-lg);
      background: var(--mr-surface);
      color: var(--mr-ink);

      .mat-icon {
        flex: none;
        width: 20px;
        height: 20px;
        margin-top: 3px;
        font-size: 20px;
        color: var(--mr-danger-ink);
      }
    }

    .ask {
      flex: 1;
      min-width: 0;
    }

    .ask__q {
      margin: 0;
      font-family: var(--mr-font-display);
      font-size: 20px;
      font-weight: 700;
      line-height: 26px;
    }

    .ask__text {
      margin: 2px 0 0;
      font-size: 15px;
      line-height: 20px;
      color: var(--mr-ink-muted);
    }

    .ask__buttons {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-2);
      margin-top: var(--mr-space-3);

      @media (min-width: 768px) {
        flex-direction: row;
      }
    }

    button {
      --mat-button-outlined-container-height: 48px;
      --mat-button-outlined-label-text-color: var(--mr-ink);
      min-width: 140px;
      font-family: var(--mr-font-sans);
    }

    .danger {
      --mat-button-outlined-label-text-color: var(--mr-danger-ink);
      --mat-button-outlined-outline-color: var(--mr-danger-ink);
    }
  `,
})
export class EndAidQuestion {
  /** "Encerrar a Ajuda de Sálvia?". */
  readonly question = input.required<string>();
  /** What happens to the hit points, in words. */
  readonly text = input.required<string>();
  /** "Cancelar": nothing is changed. */
  readonly declined = output<void>();
  readonly confirm = output<void>();
  private readonly cancelButton = viewChild('cancelButton', {
    read: ElementRef<HTMLButtonElement>,
  });

  constructor() {
    // Opening the question puts the focus on the safe button, with its ring.
    effect(() => this.cancelButton()?.nativeElement.focus({ focusVisible: true } as FocusOptions));
  }
}
