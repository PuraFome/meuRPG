import { Component, input, output, signal } from '@angular/core';
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
})
export class ImagePickerDialog {
  readonly campaignId = input.required<string>();
  readonly picked = output<GalleryImage>();
  readonly closed = output<void>();

  protected readonly imageId = signal<string | null>(null);
  private readonly image = signal<GalleryImage | null>(null);

  protected choose(image: GalleryImage): void {
    this.image.set(image);
  }

  protected insert(): void {
    const image = this.image();
    if (image) {
      this.picked.emit(image);
    }
  }
}
