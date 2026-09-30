import {
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  computed,
  inject,
  input,
  output,
  signal,
  viewChild,
} from '@angular/core';
import { FormControl, ReactiveFormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';

import type { GalleryImage } from '../../../../gen/meurpg/maps/v1/gallery_pb';
import { GalleryClient } from '../../../core/images/gallery-client';
import {
  IMAGE_NAME_MAX,
  deleteRefusal,
  imageNameError,
  renameErrorMessage,
} from '../../../core/images/gallery-copy';
import { imageMeta } from '../../../core/images/image-format';

type Mode = 'idle' | 'rename' | 'saving' | 'confirm' | 'deleting';

let nextId = 0;

/**
 * One image of the gallery (E5-20, E5-21): the 480px thumbnail,
 * cover-cropped; the name; "2000 × 1400 px, 1,5 MB"; and its actions.
 *
 * - Actions: on a screen with a mouse, "Ver / Renomear / Apagar" cover the
 *   lower 48px of the thumbnail on hover or keyboard focus, so nothing
 *   shifts; on a touch screen (`hover: none`), "Renomear / Apagar" are
 *   always there under the card, and tapping the thumbnail is "Ver".
 * - Renaming happens in place: the name becomes a field with "Salvar nome"
 *   and "Cancelar" (Esc cancels too).
 * - Deleting confirms in place (docs/design.md: an action that can't be
 *   undone asks on the same screen): "Apagar <nome>? Não dá para desfazer."
 *   with "Apagar imagem", focused, and "Cancelar".
 *
 * The card calls `GalleryClient` itself and tells the page what changed
 * (`renamed`, `deleted`); the page owns the list and the focus after a
 * delete. The line "Usada em …" of the artboards waits for the maps slice:
 * `GalleryImage` does not say where an image is used yet.
 */
@Component({
  selector: 'app-gallery-card',
  imports: [
    MatButtonModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    ReactiveFormsModule,
  ],
  templateUrl: './gallery-card.html',
  styleUrl: './gallery-card.scss',
})
export class GalleryCard {
  private readonly gallery = inject(GalleryClient);
  private readonly injector = inject(Injector);

  readonly image = input.required<GalleryImage>();
  readonly campaignId = input.required<string>();
  /** Below the fold, the thumbnail loads lazily. */
  readonly eager = input(false);

  readonly view = output<void>();
  readonly renamed = output<GalleryImage>();
  readonly deleted = output<GalleryImage>();

  protected readonly mode = signal<Mode>('idle');
  protected readonly error = signal<string | null>(null);
  protected readonly meta = computed(() => imageMeta(this.image()));
  protected readonly nameMax = IMAGE_NAME_MAX;
  protected readonly nameControl = new FormControl('', { nonNullable: true });
  protected readonly nameError = signal<string | null>(null);

  protected readonly ids = {
    name: `gallery-card-${++nextId}`,
    confirm: `gallery-card-confirm-${nextId}`,
  };

  // `read: ElementRef`: on a Material button (or a matInput) the template
  // reference is the directive, not the element.
  private readonly thumb = viewChild.required<ElementRef<HTMLButtonElement>>('thumb');
  private readonly renameButton = viewChild('renameButton', {
    read: ElementRef<HTMLButtonElement>,
  });
  private readonly nameInput = viewChild('nameInput', { read: ElementRef<HTMLInputElement> });
  private readonly confirmButton = viewChild('confirmButton', {
    read: ElementRef<HTMLButtonElement>,
  });

  /** Focus back on the thumbnail (after the lightbox closes on it). */
  focusView(): void {
    this.thumb().nativeElement.focus();
  }

  startRename(): void {
    this.error.set(null);
    this.nameError.set(null);
    this.nameControl.setValue(this.image().name);
    this.mode.set('rename');
    this.afterRender(() => {
      const input = this.nameInput()?.nativeElement;
      input?.focus();
      input?.select();
    });
  }

  askDelete(): void {
    this.error.set(null);
    this.mode.set('confirm');
    this.afterRender(() => this.confirmButton()?.nativeElement.focus());
  }

  protected cancelRename(): void {
    this.mode.set('idle');
    this.nameError.set(null);
    this.afterRender(() => this.focusAfterAction());
  }

  protected cancelDelete(): void {
    this.mode.set('idle');
    this.afterRender(() => this.focusAfterAction());
  }

  protected async saveName(): Promise<void> {
    if (this.mode() !== 'rename') {
      return;
    }
    const name = this.nameControl.value;
    const invalid = imageNameError(name);
    if (invalid) {
      this.showNameError(invalid);
      return;
    }
    if (name.trim() === this.image().name) {
      this.cancelRename();
      return;
    }
    this.mode.set('saving');
    try {
      const image = await this.gallery.rename(this.campaignId(), this.image().id, name.trim());
      this.mode.set('idle');
      this.renamed.emit(image);
      this.afterRender(() => this.focusAfterAction());
    } catch (err) {
      this.mode.set('rename');
      this.afterRender(() => this.showNameError(renameErrorMessage(err)));
    }
  }

  protected async confirmDelete(): Promise<void> {
    this.mode.set('deleting');
    try {
      await this.gallery.delete(this.campaignId(), this.image().id);
      this.deleted.emit(this.image());
    } catch (err) {
      const refusal = deleteRefusal(err);
      if (refusal.gone) {
        // Already deleted (another tab): same outcome for this screen.
        this.deleted.emit(this.image());
        return;
      }
      this.mode.set('idle');
      this.error.set(refusal.message);
      this.afterRender(() => this.focusAfterAction());
    }
  }

  /** Shows the message as the field's `mat-error` (red outline, read with
   * the field) until the next keystroke, and puts focus back in it. */
  private showNameError(message: string): void {
    this.nameError.set(message);
    this.nameControl.setErrors({ name: true });
    this.nameControl.markAsTouched();
    this.nameInput()?.nativeElement.focus();
  }

  protected onNameKeydown(event: KeyboardEvent): void {
    if (event.key === 'Escape') {
      event.preventDefault();
      event.stopPropagation();
      this.cancelRename();
    }
  }

  /** Where focus goes when a rename or a delete ends without the card
   * leaving: "Renomear" when it is showing, else the thumbnail. */
  private focusAfterAction(): void {
    const rename = this.renameButton()?.nativeElement;
    if (rename && rename.offsetParent !== null) {
      rename.focus();
    } else {
      this.focusView();
    }
  }

  private afterRender(fn: () => void): void {
    afterNextRender(fn, { injector: this.injector });
  }
}
