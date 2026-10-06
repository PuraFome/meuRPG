import { Component, ElementRef, Injector, afterNextRender, inject, input, output, signal, viewChild } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { GalleryImage } from '../../../../gen/meurpg/maps/v1/gallery_pb';
import { GalleryPicker } from '../../../shared/gallery-picker/gallery-picker';
import { DocDialog } from '../doc-dialog/doc-dialog';

/**
 * "Imagem da galeria" (the editor toolbar): the shared gallery picker in a
 * dialog. "Inserir imagem" hands the chosen image to the editor, which
 * writes `![nome](imagem:<id>)` on a paragraph of its own.
 */
@Component({
  selector: 'app-image-picker-dialog',
  imports: [DocDialog, GalleryPicker, MatButtonModule, MatIconModule],
  templateUrl: './image-picker-dialog.html',
  styles: `
    .ask {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-2);
      margin-top: var(--mr-space-4);
    }

    .ask__title {
      align-self: flex-start;
      margin: 0;
      font-family: var(--mr-font-display);
      font-size: 20px;
      font-weight: 700;
      line-height: 26px;
    }

    .ask__text {
      display: flex;
      align-items: flex-start;
      gap: 6px;
      margin: 0;
      color: var(--mr-warning-ink);
      font-size: 15px;
      line-height: 21px;

      .mat-icon {
        flex: none;
        width: 20px;
        height: 20px;
        font-size: 20px;
      }
    }
  `,
})
export class ImagePickerDialog {
  readonly campaignId = input.required<string>();
  readonly picked = output<GalleryImage>();
  readonly closed = output<void>();

  protected readonly imageId = signal<string | null>(null);
  private readonly injector = inject(Injector);
  private readonly firstAnswer = viewChild('firstAnswer', { read: ElementRef<HTMLElement> });
  private readonly image = signal<GalleryImage | null>(null);

  protected choose(image: GalleryImage): void {
    this.image.set(image);
    this.asking.set(false);
  }

  /** The question over a picture of the whole map: the players read the document (RN-10). */
  protected readonly asking = signal(false);

  protected insert(): void {
    const image = this.image();
    if (!image) {
      return;
    }
    // A textured map shows the rooms the players have not found: ask first, in place.
    if (image.showsWholeMap && !this.asking()) {
      this.asking.set(true);
      // "Voltar" has the focus: nothing is inserted before the second press.
      afterNextRender(() => this.firstAnswer()?.nativeElement.focus(), { injector: this.injector });
      return;
    }
    this.picked.emit(image);
  }

  protected back(): void {
    this.asking.set(false);
  }
}
