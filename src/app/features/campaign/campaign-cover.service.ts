import { Injectable, inject, signal } from '@angular/core';
import { IndexedDbFileRepository } from '../../core';
import type { CampaignFolder } from '../../core';

/**
 * Caches object URLs for campaign cover blobs stored in IndexedDB.
 * The tree list uses per-folder thumbnails (id `${coverFileId}-thumb`);
 * the campaign header uses the full-size blob (id `coverFileId`).
 */
@Injectable({
  providedIn: 'root',
})
export class CampaignCoverService {
  private readonly fileRepo = inject(IndexedDbFileRepository);

  /** Bumped whenever thumbnails are (re)loaded so templates re-render. */
  readonly thumbVersion = signal(0);

  private readonly thumbUrls = new Map<string, string>();
  private readonly fullUrls = new Map<string, string>();
  private readonly thumbSources = new Map<string, string | null>();

  thumbUrl(folderId: string): string | undefined {
    return this.thumbUrls.get(folderId);
  }

  /** Load/refresh thumbnails for the given folders (no-op when unchanged). */
  async syncThumbnails(folders: CampaignFolder[]): Promise<void> {
    for (const folder of folders) {
      const coverFileId = folder.coverFileId ?? null;
      if (this.thumbSources.get(folder.id) === coverFileId) continue;

      this.revokeThumb(folder.id);
      this.thumbSources.set(folder.id, coverFileId);

      if (coverFileId) {
        const record = await this.fileRepo.get(`${coverFileId}-thumb`);
        if (record) {
          this.thumbUrls.set(folder.id, URL.createObjectURL(record.data));
        }
      }
    }
    this.thumbVersion.update((v) => v + 1);
  }

  /** Load the full-size cover blob and return a cached object URL. */
  async loadFull(coverFileId: string): Promise<string | null> {
    const cached = this.fullUrls.get(coverFileId);
    if (cached) return cached;

    const record = await this.fileRepo.get(coverFileId);
    if (!record) return null;

    const url = URL.createObjectURL(record.data);
    this.fullUrls.set(coverFileId, url);
    return url;
  }

  revokeThumb(folderId: string): void {
    const url = this.thumbUrls.get(folderId);
    if (url) URL.revokeObjectURL(url);
    this.thumbUrls.delete(folderId);
  }

  revokeFull(coverFileId: string): void {
    const url = this.fullUrls.get(coverFileId);
    if (url) URL.revokeObjectURL(url);
    this.fullUrls.delete(coverFileId);
  }

  revokeAll(): void {
    for (const url of this.thumbUrls.values()) URL.revokeObjectURL(url);
    for (const url of this.fullUrls.values()) URL.revokeObjectURL(url);
    this.thumbUrls.clear();
    this.fullUrls.clear();
    this.thumbSources.clear();
    this.thumbVersion.set(0);
  }
}
