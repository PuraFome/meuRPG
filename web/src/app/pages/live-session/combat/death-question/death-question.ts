import { Component, ElementRef, Injector, afterNextRender, computed, effect, inject, input, output, viewChild } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { Combatant } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { article } from '../../../../core/combat/combat-log';

/**
 * "Brisa falhou três vezes no teste contra a morte. Confirmar a morte?" (RN-03,
 * E6-30): the master's question, asked in place at the top of the card for each
 * character at three failures (only he is told they are dying). Confirming marks
 * the character dead for good (the sheet is kept, the player may create another
 * character) and takes it out of the order; it cannot be undone, which the text
 * says. "Ainda não" puts the question away (the order's row keeps a "Confirmar a
 * morte" button), because a cure before the confirmation brings the character
 * back. The two buttons are outlined and the same height; focus lands on the
 * safe one, "Ainda não".
 */
@Component({
  selector: 'app-death-question',
  imports: [MatButtonModule, MatIconModule],
  template: `
    @for (c of asking(); track c.id) {
      <div #box class="ask mr-notice mr-notice--danger" role="alertdialog" [attr.aria-label]="'Confirmar a morte ' + (article(c.label) === 'a' ? 'da ' : 'do ') + c.label">
        <mat-icon aria-hidden="true">warning</mat-icon>
        <div class="ask__body">
          <p class="ask__text">
            <strong>{{ c.label }} falhou três vezes no teste contra a morte.</strong>
            Confirmar a morte? A ficha fica guardada, e o jogador pode criar outro personagem. Isso não se desfaz.
            Se alguém curar {{ the(c.label) }} antes, {{ article(c.label) === 'a' ? 'ela' : 'ele' }} volta.
          </p>
          <div class="ask__btns">
            <button mat-stroked-button type="button" class="ask__btn ask__go" [disabled]="busy()" (click)="confirm.emit(c.id)">
              Confirmar a morte
            </button>
            <button #safe mat-stroked-button type="button" class="ask__btn" (click)="later.emit(c.id)">Ainda não</button>
          </div>
        </div>
      </div>
    }
  `,
  styles: `
    :host {
      display: contents;
    }

    .ask {
      align-items: flex-start;
      // Under the sticky app bar when it scrolls into view.
      scroll-margin-top: 80px;
    }

    .ask__body {
      display: flex;
      flex: 1;
      flex-direction: column;
      gap: 12px;
      min-width: 0;
    }

    .ask__text {
      margin: 0;
    }

    // A phone: the two answers stacked, full width and equal. From a tablet up, side by side.
    .ask__btns {
      display: grid;
      grid-template-columns: minmax(0, 1fr);
      gap: 10px;

      @media (min-width: 768px) {
        grid-template-columns: repeat(2, 200px);
      }
    }

    .ask__btn {
      --mat-button-outlined-container-height: 48px;
      --mat-button-outlined-label-text-color: var(--mr-ink);
      --mat-button-outlined-outline-color: var(--mr-control-line);
      width: 100%;
      white-space: nowrap;

      @media (min-width: 768px) {
        --mat-button-outlined-container-height: 44px;
      }
    }

    .ask__go {
      --mat-button-outlined-label-text-color: var(--mr-danger-ink);
      --mat-button-outlined-outline-color: var(--mr-danger-ink);
    }
  `,
})
export class DeathQuestion {
  private readonly injector = inject(Injector);

  /** The characters at three failures, and the ones the master put away. */
  readonly dying = input.required<readonly Combatant[]>();
  readonly dismissed = input<ReadonlySet<string>>(new Set());
  readonly busy = input(false);

  readonly confirm = output<string>();
  readonly later = output<string>();

  private readonly safe = viewChild('safe', { read: ElementRef<HTMLButtonElement> });
  private readonly box = viewChild('box', { read: ElementRef<HTMLElement> });
  protected readonly asking = computed(() => this.dying().filter((c) => !this.dismissed().has(c.id)));
  protected readonly article = article;

  constructor() {
    // The question opens with the focus on the safe button.
    let shown = 0;
    effect(() => {
      const n = this.asking().length;
      if (n > shown) {
        afterNextRender(
          () => {
            // The whole question under the app bar, then the focus on the safe button.
            this.box()?.nativeElement.scrollIntoView({ block: 'start' });
            this.safe()?.nativeElement.focus({ preventScroll: true });
          },
          { injector: this.injector },
        );
      }
      shown = n;
    });
  }

  protected the(label: string): string {
    return `${article(label)} ${label}`;
  }
}
