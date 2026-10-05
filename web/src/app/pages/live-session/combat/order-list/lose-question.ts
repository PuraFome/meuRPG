import { Component, ElementRef, effect, input, output, viewChild } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

/**
 * The master's question when a caster loses concentration (MR-037, E9-12 state 5): a warning box with its icon, the
 * question in bold, what happens in the normal weight, and the two answers the same width ("Voltar", then the one that
 * dismisses the creatures), stacked at 320 px. The focus goes to "Voltar", with its ring.
 */
@Component({
  selector: 'app-lose-question',
  imports: [MatButtonModule, MatIconModule],
  template: `
    <div class="box mr-notice mr-notice--warning" role="alertdialog" [attr.aria-label]="question()">
      <mat-icon aria-hidden="true">warning</mat-icon>
      <div class="ask">
        <p class="ask__q">{{ question() }}</p>
        <p class="ask__text">{{ text() }}</p>
        <div class="ask__buttons">
          <button #back mat-stroked-button type="button" (click)="cancel.emit()">Voltar</button>
          <button mat-stroked-button type="button" class="danger" (click)="confirm.emit()">{{ answer() }}</button>
        </div>
      </div>
    </div>
  `,
  styles: `
    :host {
      display: block;
    }

    .box {
      align-items: flex-start;
      margin-top: 8px;
      padding: 12px 14px;
    }

    .ask {
      flex: 1;
      min-width: 0;
    }

    .ask__q {
      font-weight: 700;
    }

    .ask__text {
      margin: 2px 0 0;
      font-size: 15px;
      font-weight: 400;
      line-height: 20px;
    }

    .ask__buttons {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 200px));
      gap: 8px;
      margin-top: 10px;

      @media (max-width: 359.98px) {
        grid-template-columns: minmax(0, 1fr);
      }
    }

    button {
      --mat-button-outlined-container-height: 48px;
      --mat-button-outlined-label-text-color: var(--mr-ink);
      width: 100%;
    }

    .danger {
      --mat-button-outlined-label-text-color: var(--mr-danger-ink);
      --mat-button-outlined-outline-color: var(--mr-danger-ink);
    }
  `,
})
export class LoseQuestion {
  readonly question = input.required<string>();
  readonly text = input.required<string>();
  readonly answer = input.required<string>();
  readonly cancel = output<void>();
  readonly confirm = output<void>();
  private readonly back = viewChild('back', { read: ElementRef<HTMLButtonElement> });

  constructor() {
    // Opening the question puts the focus on the safe button, with its ring.
    effect(() => this.back()?.nativeElement.focus({ focusVisible: true } as FocusOptions));
  }
}

/** The solid "Concentração" pill by a name in the master's order: an hourglass and the word, on one line. */
@Component({
  selector: 'app-conc-pill',
  imports: [MatIconModule],
  template: `<mat-icon aria-hidden="true">hourglass_empty</mat-icon>Concentração`,
  styles: `
    :host {
      display: inline-flex;
      align-items: center;
      gap: 4px;
      box-sizing: border-box;
      height: 22px;
      padding: 0 8px 0 6px;
      border: 1.5px solid var(--mr-ink);
      border-radius: var(--mr-radius-pill);
      font-size: 13px;
      font-weight: 700;
      line-height: 1;
      white-space: nowrap;
      color: var(--mr-ink);
    }

    .mat-icon {
      width: 14px;
      height: 14px;
      font-size: 14px;
    }

    // A 320 px phone: the name's column is narrow beside "Dano/Cura", so the pill is the word alone.
    @media (max-width: 359.98px) {
      :host {
        padding: 0 7px;
        font-size: 12px;
      }

      .mat-icon {
        display: none;
      }
    }
  `,
})
export class ConcPill {}
