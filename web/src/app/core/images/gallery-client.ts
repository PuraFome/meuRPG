import { Injectable, inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';

import {
  type GalleryImage,
  GalleryService,
  type GalleryUsage,
} from '../../../gen/meurpg/maps/v1/gallery_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';
import { DEFAULT_LIMITS } from './upload-errors';

export interface GalleryListing {
  /** Newest first, as the server sends them. */
  readonly images: GalleryImage[];
  readonly usage: GalleryUsage;
}

/**
 * Thin wrapper around the generated `GalleryService` client (MR-019), in the
 * same shape as `CampaignsService`. Master-only on the server: a player gets
 * `permission_denied`, a non-member `not_found` (gallery.proto). Callers
 * map those codes to Portuguese themselves.
 *
 * `providedIn: 'root'` rather than a route-level port: the gallery page,
 * the campaign page's Galeria panel and the gallery picker (the map form,
 * the document editor and "Mostrar imagem" later) all need it, and only
 * lazy code imports this file, so the generated gallery client stays out
 * of the initial bundle. Tests replace it with `{ provide: GalleryClient,
 * useValue: fake }`.
 */
@Injectable({ providedIn: 'root' })
export class GalleryClient {
  private readonly client = createClient(GalleryService, inject(CONNECT_TRANSPORT));

  async list(campaignId: string): Promise<GalleryListing> {
    const res = await this.client.listGalleryImages({ campaignId });
    return {
      images: res.images,
      // The server always sends `usage`; the fallback only keeps the
      // type honest.
      usage: res.usage ?? {
        $typeName: 'meurpg.maps.v1.GalleryUsage',
        imageCount: res.images.length,
        byteCount: res.images.reduce((sum, i) => sum + i.byteSize, 0),
        ...DEFAULT_LIMITS,
      },
    };
  }

  async rename(campaignId: string, imageId: string, name: string): Promise<GalleryImage> {
    const res = await this.client.renameGalleryImage({ campaignId, imageId, name });
    if (!res.image) {
      throw new Error('RenameGalleryImage answered without the image');
    }
    return res.image;
  }

  async delete(campaignId: string, imageId: string): Promise<void> {
    await this.client.deleteGalleryImage({ campaignId, imageId });
  }
}
