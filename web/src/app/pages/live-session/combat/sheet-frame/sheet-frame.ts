import { Component, ElementRef, input, output, viewChild } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/**
 * The frame of the combat's sheets and dialogs that scroll inside
 * themselves (the cast sheet, Escudo, "Condições…", Retomar o Fôlego):
 * the grab bar on a phone, the title and what it is about, the close
 * button, a body that scrolls and a footer with the action buttons that
 * never scrolls away, so a tall list on a 667px phone still leaves
 * "Conjurar" in reach. The title is the first focus (`tabindex="-1"`,
 * see `openSheet`) and the name of the dialog (`titleId`).
 */
@Component({
  selector: 'app-sheet-frame',
  imports: [MatIconModule],
  template: `
    <div class="frame" [class.frame--phone]="phone()">
      @if (phone()) {
        <span class="frame__handle" aria-hidden="true"></span>
      }
      <div class="frame__head">
        @if (icon()) {
          <mat-icon class="frame__icon" aria-hidden="true">{{ icon() }}</mat-icon>
        }
        <div class="frame__titles">
          <h2 class="frame__title" [id]="titleId()" tabindex="-1">{{ title() }}</h2>
          @if (subtitle()) {
            <p class="frame__sub">{{ subtitle() }}</p>
          }
        </div>
        @if (closable()) {
          <button type="button" class="frame__close" aria-label="Fechar" (click)="closed.emit()">
            <mat-icon aria-hidden="true">close</mat-icon>
          </button>
        }
      </div>
      <div #body class="frame__body"><ng-content /></div>
      <div class="frame__foot"><ng-content select="[foot]" /></div>
    </div>
  `,
  styleUrl: './sheet-frame.scss',
  host: { style: 'display: block' },
})
export class SheetFrame {
  readonly title = input.required<string>();
  readonly subtitle = input('');
  readonly titleId = input('sheet-t');
  /** A leading icon before the title (Escudo's shield). */
  readonly icon = input('');
  /** The phone's bottom sheet: it draws its grab bar. */
  readonly phone = input(false);
  /** An alert dialog that must be answered has no close button. */
  readonly closable = input(true);

  readonly closed = output<void>();

  private readonly body = viewChild.required<ElementRef<HTMLElement>>('body');

  /** An error opens at the top of the scrolling body, where it is seen. */
  scrollToTop(): void {
    this.body().nativeElement.scrollTop = 0;
  }
}
