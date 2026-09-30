import { Component, computed, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { UploadItem } from '../../../core/images/upload-queue';

/**
 * One file on its way to the gallery (E5-20: "Enviando ruinas.jpg… 60%"),
 * where its image will appear: the first cells of the gallery grid, or of
 * the picker's (`compact`). A determinate bar (`role="progressbar"`) and
 * "Cancelar envio".
 *
 * The percentage is not a live region (it would talk over everything);
 * the page announces when the image arrives. No preview of the file: the
 * app's CSP allows images from its own origin only (`img-src 'self'`),
 * never a `blob:` URL.
 */
@Component({
  selector: 'app-upload-progress',
  imports: [MatButtonModule, MatIconModule],
  templateUrl: './upload-progress.html',
  styleUrl: './upload-progress.scss',
  host: { '[class.compact]': 'compact()' },
})
export class UploadProgress {
  readonly item = input.required<UploadItem>();
  /** The picker's smaller tile (E5-31) instead of the gallery's card. */
  readonly compact = input(false);
  readonly cancel = output<void>();

  /** The line over the bar: the words, which may be cut short with "…"
   * on a narrow card, and the percentage, which never is. */
  protected readonly label = computed(() => {
    const { fileName, status, percent } = this.item();
    switch (status) {
      case 'queued':
        return { text: `Na fila: ${fileName}`, percent: null };
      case 'processing':
        return { text: `Preparando ${fileName}…`, percent: null };
      default:
        return { text: `Enviando ${fileName}…`, percent: `${percent}%` };
    }
  });
}
