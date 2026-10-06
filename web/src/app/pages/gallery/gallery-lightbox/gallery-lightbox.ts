import {
  Component,
  ElementRef,
  Injector,
  type OnChanges,
  afterNextRender,
  computed,
  inject,
  input,
  output,
  signal,
  viewChild,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { GalleryImage } from '../../../../gen/meurpg/maps/v1/gallery_pb';
import { imageMeta } from '../../../core/images/image-format';

/** How far a finger must travel sideways to turn the page (phone). */
const SWIPE_PX = 50;

/**
 * "Ver imagem" (E5-20, lower half): the full image, `/images/<id>`, never
 * the thumbnail, shown whole on the `ground` colour, with its name,
 * "Imagem anterior" / "2 de 5" / "Próxima imagem", and "Renomear",
 * "Apagar" and "Fechar".
 *
 * A native modal `<dialog>`: the browser makes the rest of the page inert
 * (focus stays inside), Esc closes it, and it sits above everything
 * without the CDK overlay. ← and → turn the page, and on a phone it is
 * full-screen and a swipe turns the page too. Turning past the last image
 * goes back to the first, so a focused button never disappears. The page
 * returns focus to the card of the image that was open when it closes.
 */
@Component({
  selector: 'app-gallery-lightbox',
  imports: [MatButtonModule, MatIconModule],
  templateUrl: './gallery-lightbox.html',
  styleUrl: './gallery-lightbox.scss',
})
export class GalleryLightbox implements OnChanges {
  readonly images = input.required<readonly GalleryImage[]>();
  /** The open image's position in `images`, or `null` when closed. */
  readonly index = input<number | null>(null);

  readonly indexChange = output<number>();
  readonly closed = output<void>();
  readonly rename = output<GalleryImage>();
  readonly remove = output<GalleryImage>();
  /** "Pedir um ajuste" on a generated image: the page opens the generate dialog on it. */
  readonly adjust = output<GalleryImage>();

  protected readonly image = computed(() => {
    const i = this.index();
    return i === null ? null : (this.images()[i] ?? null);
  });
  protected readonly meta = computed(() => {
    const img = this.image();
    return img ? imageMeta(img) : '';
  });
  /** The image whose full file has loaded; until then its thumbnail shows. */
  protected readonly loadedId = signal<string | null>(null);
  protected readonly position = computed(
    () => `${(this.index() ?? 0) + 1} de ${this.images().length}`,
  );

  private readonly dialog = viewChild.required<ElementRef<HTMLDialogElement>>('dialog');
  private readonly closeButton = viewChild<ElementRef<HTMLButtonElement>>('closeButton');
  private readonly injector = inject(Injector);
  private swipeStartX: number | null = null;

  // Opens and closes the native dialog as `index` comes and goes, after the
  // DOM has the image, so focus can land on "Fechar". (A lifecycle hook and
  // `afterNextRender` rather than an effect: both are already in the app's
  // initial bundle, `afterRenderEffect` is not.)
  ngOnChanges(): void {
    afterNextRender(() => this.sync(), { injector: this.injector });
  }

  private sync(): void {
    const open = this.image() !== null;
    const dialog = this.dialog().nativeElement;
    if (open && !dialog.open) {
      if (typeof dialog.showModal === 'function') {
        dialog.showModal();
      } else {
        // jsdom (the unit tests) has no showModal.
        dialog.setAttribute('open', '');
      }
      this.closeButton()?.nativeElement.focus();
    } else if (!open && dialog.open) {
      this.close();
    }
  }

  /** Closes the dialog; its `close` event tells the page (`closed`). */
  close(): void {
    const dialog = this.dialog().nativeElement;
    if (typeof dialog.close === 'function') {
      if (dialog.open) {
        dialog.close();
      }
    } else {
      dialog.removeAttribute('open');
      this.closed.emit();
    }
  }

  protected previous(): void {
    this.go(-1);
  }

  protected next(): void {
    this.go(1);
  }

  protected onKeydown(event: KeyboardEvent): void {
    if (event.key === 'ArrowLeft') {
      event.preventDefault();
      this.previous();
    } else if (event.key === 'ArrowRight') {
      event.preventDefault();
      this.next();
    }
  }

  /** A click on the scrim lands on the `<dialog>` itself, outside the
   * panel: it closes, like Esc. */
  protected onDialogClick(event: MouseEvent): void {
    if (event.target === this.dialog().nativeElement) {
      this.close();
    }
  }

  protected onPointerDown(event: PointerEvent): void {
    this.swipeStartX = event.pointerType === 'mouse' ? null : event.clientX;
  }

  protected onPointerUp(event: PointerEvent): void {
    if (this.swipeStartX === null) {
      return;
    }
    const dx = event.clientX - this.swipeStartX;
    this.swipeStartX = null;
    if (dx > SWIPE_PX) {
      this.previous();
    } else if (dx < -SWIPE_PX) {
      this.next();
    }
  }

  private go(step: number): void {
    const count = this.images().length;
    const i = this.index();
    if (i === null || count < 2) {
      return;
    }
    this.indexChange.emit((i + step + count) % count);
  }
}
