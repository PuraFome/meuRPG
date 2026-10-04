import { Component, ElementRef, afterNextRender, input, output, viewChild } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

/**
 * A question that takes the place of a line (E7-09, E8-14): "Remover o marco
 * Salvar o mercador?", "Desfazer o marco…?". A warm notice, never a dialog:
 * the whole question is scrolled into view below the sticky app bar and the
 * focus lands on "Voltar", the safe answer; Esc is "Voltar" too. The two
 * answers are outlined and the same size. The host puts the focus back on the
 * button that opened it when it steps back.
 */
@Component({
  selector: 'app-milestone-ask',
  imports: [MatButtonModule, MatIconModule],
  template: `
    <div #box class="ask mr-notice mr-notice--warning" role="alertdialog" [attr.aria-label]="title()" (keydown.escape)="back.emit()">
      <mat-icon aria-hidden="true">warning</mat-icon>
      <div class="ask__body">
        <p class="ask__title">{{ title() }}</p>
        <p class="ask__text">{{ text() }}</p>
        @if (error()) {
          <p class="ask__error" role="alert">{{ error() }}</p>
        }
        <div class="ask__btns">
          <button #voltar mat-stroked-button type="button" class="ask__btn" (click)="back.emit()">Voltar</button>
          <button mat-stroked-button type="button" class="ask__btn ask__go" [attr.aria-disabled]="busy()" (click)="go.emit()">
            {{ goLabel() }}
          </button>
        </div>
      </div>
    </div>
  `,
  styleUrl: './milestone-ask.scss',
})
export class MilestoneAsk {
  readonly title = input.required<string>();
  readonly text = input.required<string>();
  /** The button that does it: "Remover", "Desfazer marco". */
  readonly goLabel = input.required<string>();
  readonly error = input('');
  readonly busy = input(false);

  readonly back = output<void>();
  readonly go = output<void>();

  private readonly box = viewChild.required<ElementRef<HTMLElement>>('box');
  private readonly voltar = viewChild.required('voltar', { read: ElementRef<HTMLButtonElement> });

  constructor() {
    afterNextRender(() => {
      this.box().nativeElement.scrollIntoView?.({ block: 'start' }); // jsdom has none
      this.voltar().nativeElement.focus({ preventScroll: true });
    });
  }
}
