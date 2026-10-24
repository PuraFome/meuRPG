import {
  Component,
  DOCUMENT,
  DestroyRef,
  Injector,
  computed,
  inject,
  input,
  signal,
} from '@angular/core';
import { MatDialog, MatDialogRef } from '@angular/material/dialog';

import { PHONE_QUERY, mediaQuery } from '../../../shared/map-view/media-query';
import type { ShownImageVm } from '../../../core/play/live-session.types';
import { Fold } from '../../../shared/fold/fold';
import { ShownImageViewer } from '../shown-image-block/shown-image-viewer';

/**
 * "Imagens que o mestre deixou (2)" (E6-25b, MR-028, C5-02, C6-08): the images the master
 * left with the players, on the player's session page. A fold of one row on a phone,
 * open under "O mestre está mostrando" on a computer; inside it a grid of thumbnails, each one a
 * button named "Ver <nome> em tela cheia" (the same read-only view as the image on show). The border is a
 * `line`, not the accent frame: the frame is only for what is shown right
 * now. The block is hidden while the list is empty, and it is not announced
 * when it changes: the players asked for no interruptions, and the list is
 * theirs to look at. The view closes when its image is taken back.
 */
@Component({
  selector: 'app-left-images-block',
  imports: [Fold],
  templateUrl: './left-images-block.html',
  styleUrl: './left-images-block.scss',
})
export class LeftImagesBlock {
  private readonly dialog = inject(MatDialog);
  private readonly injector = inject(Injector);
  private readonly document = inject(DOCUMENT);

  /** The images left with the players, oldest first. */
  readonly images = input<readonly ShownImageVm[]>([]);

  protected readonly phone = mediaQuery(PHONE_QUERY);
  /** The image in the open view; the view closes when it leaves the list. */
  private readonly viewing = signal<string | null>(null);
  private readonly viewerData = computed(
    () => this.images().find((i) => i.id === this.viewing()) ?? null,
  );
  private viewer: MatDialogRef<ShownImageViewer> | null = null;

  constructor() {
    inject(DestroyRef).onDestroy(() => this.viewer?.close());
  }

  protected readonly title = computed(
    () => `Imagens que o mestre deixou (${this.images().length})`,
  );

  protected thumb(image: ShownImageVm): string {
    return `${image.url}/thumb`;
  }

  protected openFull(image: ShownImageVm): void {
    if (this.viewer) {
      return;
    }
    this.viewing.set(image.id);
    const phone = this.phone();
    this.viewer = this.dialog.open<ShownImageViewer, unknown>(ShownImageViewer, {
      data: this.viewerData,
      injector: this.injector,
      width: phone ? '100vw' : '90vw',
      maxWidth: phone ? '100vw' : '1100px',
      height: phone ? '100dvh' : '88dvh',
      maxHeight: phone ? '100dvh' : '88dvh',
      autoFocus: 'dialog',
    });
    this.viewer.afterClosed().subscribe((reason) => {
      this.viewer = null;
      this.viewing.set(null);
      // Taken back while looking: the row is gone, so focus goes to the map's heading.
      if (reason === 'stopped') {
        this.document.getElementById('session-map-heading')?.focus();
      }
    });
  }
}
