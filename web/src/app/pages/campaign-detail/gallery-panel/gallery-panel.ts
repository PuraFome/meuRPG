import { Component, OnInit, computed, inject, input, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { RouterLink } from '@angular/router';
import { Code } from '@connectrpc/connect';

import type { GalleryImage, GalleryUsage } from '../../../../gen/meurpg/maps/v1/gallery_pb';
import { describeConnectError } from '../../../core/connect/connect-errors';
import { GalleryClient } from '../../../core/images/gallery-client';
import { usageLine } from '../../../core/images/image-format';

/** How many of the newest images the panel shows (E5-09). */
const PREVIEW_COUNT = 5;

type PanelState =
  | { status: 'loading' }
  | { status: 'error'; message: string }
  | { status: 'ready'; images: readonly GalleryImage[]; usage: GalleryUsage };

/**
 * The master's "Galeria" panel on `/campaigns/:id` (E5-09, MR-019): the
 * newest images in a row (5 from a tablet up, 3 on a phone), "5 imagens,
 * 5,8 MB de 500 MB" and "Abrir galeria". `CampaignDetail` renders it for
 * the master only: the server refuses the gallery to a player (RN-10).
 */
@Component({
  selector: 'app-gallery-panel',
  imports: [MatButtonModule, MatIconModule, RouterLink],
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
  protected readonly usageText = computed(() => {
    const s = this.state();
    return s.status === 'ready' ? usageLine(s.usage, ', ') : '';
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
