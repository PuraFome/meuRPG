import { Component, ElementRef, afterNextRender, input, output, viewChild } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

let nextId = 0;

/**
 * The frame of every dialog of the document screens (E5-29): a native modal
 * `<dialog>` with an icon, a title (Alegreya 24), an optional line under it,
 * "Fechar", the content, and a footer slot (`dialogFooter`).
 *
 * The page puts it in the DOM only while it is open (`@if`), so the content
 * inside loads on open and the dialog opens itself after its first render.
 * Esc and a click on the scrim close it (`closed`); the page then takes it
 * out and gives focus back to what opened it. Together with the gallery's
 * lightbox, it is the only place with a shadow (docs/design.md).
 */
@Component({
  selector: 'app-doc-dialog',
  imports: [MatIconModule],
  templateUrl: './doc-dialog.html',
  styleUrl: './doc-dialog.scss',
})
export class DocDialog {
  readonly heading = input.required<string>();
  readonly subtitle = input('');
  /** A Material Symbols name drawn before the subtitle (the map's "visibility"). */
  readonly subtitleIcon = input('');
  readonly icon = input('');
  /** `wide` for the map, the default for lists and confirmations. */
  readonly size = input<'wide' | 'narrow'>('narrow');
  readonly closed = output<void>();

  protected readonly titleId = `doc-dialog-title-${nextId++}`;
  private readonly dialog = viewChild.required<ElementRef<HTMLDialogElement>>('dialog');
  private readonly closeButton = viewChild.required<ElementRef<HTMLButtonElement>>('closeButton');

  constructor() {
    afterNextRender(() => {
      const dialog = this.dialog().nativeElement;
      if (typeof dialog.showModal === 'function') {
        dialog.showModal();
      } else {
        // jsdom (the unit tests) has no showModal.
        dialog.setAttribute('open', '');
      }
      // Focus on "Fechar" unless something inside took it (an `autofocus`).
      if (!dialog.contains(document.activeElement) || document.activeElement === dialog) {
        this.closeButton().nativeElement.focus();
      }
    });
  }

  protected close(): void {
    const dialog = this.dialog().nativeElement;
    if (typeof dialog.close === 'function' && dialog.open) {
      dialog.close(); // its `close` event emits `closed`
    } else {
      dialog.removeAttribute('open');
      this.closed.emit();
    }
  }

  /** A click on the scrim lands on the `<dialog>` itself. */
  protected onClick(event: MouseEvent): void {
    if (event.target === this.dialog().nativeElement) {
      this.close();
    }
  }
}
