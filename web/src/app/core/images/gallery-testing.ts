import { create } from '@bufbuild/protobuf';

import {
  type GalleryImage,
  GalleryImageSchema,
  type GalleryUsage,
  GalleryUsageSchema,
} from '../../../gen/meurpg/maps/v1/gallery_pb';
import type { GalleryListing } from './gallery-client';
import type { UploadOptions } from './image-uploader';
import { UploadFailed } from './upload-errors';

/**
 * Builders and stand-ins for the gallery's specs (never imported by the
 * app itself, so never bundled): a `GalleryImage` as the server sends it,
 * its usage, and fakes for `GalleryClient` and `ImageUploader`.
 */
export function galleryImage(
  id: string,
  name: string,
  partial: Partial<Omit<GalleryImage, '$typeName'>> = {},
): GalleryImage {
  return create(GalleryImageSchema, {
    id,
    campaignId: 'camp-1',
    name,
    contentType: 'image/jpeg',
    width: 1920,
    height: 1080,
    byteSize: Math.round(1.2 * 1024 * 1024),
    url: `/images/${id}`,
    thumbnailUrl: `/images/${id}/thumb`,
    ...partial,
  });
}

export function galleryUsage(
  images: readonly GalleryImage[] = [],
  partial: Partial<Omit<GalleryUsage, '$typeName'>> = {},
): GalleryUsage {
  return create(GalleryUsageSchema, {
    imageCount: images.length,
    maxImages: 300,
    byteCount: images.reduce((sum, i) => sum + i.byteSize, 0),
    maxBytes: 500 * 1024 * 1024,
    maxImageBytes: 10 * 1024 * 1024,
    ...partial,
  });
}

/** Text with the no-break spaces the formatters put between a number and
 * its unit read as plain spaces, so expectations stay readable. */
export function plain(text: string | null | undefined): string {
  return (text ?? '').replace(/\u00a0/g, ' ');
}

/** The five images of the Mirathel artboards, newest first. */
export function mirathelImages(): GalleryImage[] {
  return [
    galleryImage('img-covil', 'Covil dos goblins', { width: 2000, height: 1400 }),
    galleryImage('img-taverna', 'Taverna do Javali'),
    galleryImage('img-capitao', 'Capitão Goblin', { width: 800, height: 1000 }),
    galleryImage('img-torre', 'Planta da torre', {
      width: 1600,
      height: 1600,
      contentType: 'image/png',
    }),
    galleryImage('img-mapa', 'Mapa de Mirathel', { width: 2400, height: 1600 }),
  ];
}

/** A `GalleryClient` stand-in: set the results, read the calls. Rename
 * and delete results are functions, so a rejection is only created when
 * the call happens (never an unhandled one). */
export class FakeGalleryClient {
  listResult: Promise<GalleryListing> = Promise.resolve({ images: [], usage: galleryUsage() });
  renameResult: (name: string) => Promise<GalleryImage> = (name) =>
    Promise.resolve(galleryImage('img', name));
  deleteResult: () => Promise<void> = () => Promise.resolve();
  readonly calls: string[][] = [];

  list(campaignId: string): Promise<GalleryListing> {
    this.calls.push(['list', campaignId]);
    return this.listResult;
  }

  rename(campaignId: string, imageId: string, name: string): Promise<GalleryImage> {
    this.calls.push(['rename', campaignId, imageId, name]);
    return this.renameResult(name);
  }

  delete(campaignId: string, imageId: string): Promise<void> {
    this.calls.push(['delete', campaignId, imageId]);
    return this.deleteResult();
  }
}

/** One upload an `ImageUploader` stand-in received; the test settles it. */
export interface PendingUpload {
  readonly campaignId: string;
  readonly file: File;
  readonly options: UploadOptions;
  resolve(image: GalleryImage): void;
  reject(err: unknown): void;
}

/** An `ImageUploader` stand-in that keeps each upload pending until the
 * test resolves or rejects it. Aborting rejects with `CANCELED`, as the
 * real one does. */
export class FakeImageUploader {
  readonly pending: PendingUpload[] = [];

  upload(campaignId: string, file: File, options: UploadOptions = {}): Promise<GalleryImage> {
    return new Promise((resolve, reject) => {
      options.signal?.addEventListener('abort', () => reject(new UploadFailed('CANCELED')));
      this.pending.push({ campaignId, file, options, resolve, reject });
    });
  }
}
