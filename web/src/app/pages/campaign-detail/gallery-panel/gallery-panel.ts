import { Component, OnInit, computed, inject, input, signal } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';
import { Code } from '@connectrpc/connect';

import type { GalleryImage, GalleryUsage } from '../../../../gen/meurpg/maps/v1/gallery_pb';
import { describeConnectError } from '../../../core/connect/connect-errors';
import { GalleryClient } from '../../../core/images/gallery-client';
import { PanelAction } from '../panel-action/panel-action';
import { PanelHead } from '../panel-head/panel-head';

/** How many of the newest images the panel shows: two rows of three (board C6-02). */
const PREVIEW_COUNT = 6;

type PanelState =
  | { status: 'loading' }
  | { status: 'error'; message: string }
  | { status: 'ready'; images: readonly GalleryImage[]; usage: GalleryUsage };

/**
 * The master's "Galeria" panel on "Preparação" (E5-09, MR-019; board C6-02): "12 de 200
 * imagens" and "Enviar imagem" in the title row, the newest six images in two rows of three, and
 * "Ver todas as 12 imagens". Both actions open the gallery page, where images are sent. The tab
 * renders it for the master only: the server refuses the gallery to a player (RN-10).
 */
@Component({
  selector: 'app-gallery-panel',
  imports: [MatIconModule, PanelAction, PanelHead],
  templateUrl: './gallery-panel.html',
  styleUrl: './gallery-panel.scss',
})
export class GalleryPanel implements OnInit {
  private readonly gallery = inject(GalleryClient);

  readonly campaignId = input.required<string>();

  protected readonly state = signal<PanelState>({ status: 'loading' });
  protected readonly preview = computed(() => {
    const s = this.state();
    return s.status === 'ready' ? s.images.slice(0, PREVIEW_COUNT) : [];
  });
  /** "12 de 200 imagens", empty while the gallery is not here. */
  protected readonly count = computed(() => {
    const s = this.state();
    if (s.status !== 'ready') {
      return '';
    }
    const { imageCount, maxImages } = s.usage;
    return `${imageCount} de ${maxImages} ${maxImages === 1 ? 'imagem' : 'imagens'}`;
  });
  protected readonly total = computed(() => {
    const s = this.state();
    return s.status === 'ready' ? s.usage.imageCount : 0;
  });

  ngOnInit(): void {
    this.gallery.list(this.campaignId()).then(
      ({ images, usage }) => this.state.set({ status: 'ready', images, usage }),
      (err: unknown) =>
        this.state.set({
          status: 'error',
          message: describeConnectError(err, {
            [Code.PermissionDenied]: 'Só o mestre da campanha vê a galeria.',
          }),
        }),
    );
  }
}
