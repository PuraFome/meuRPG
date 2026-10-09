import { Injectable } from '@angular/core';

import type { UploadOptions } from '../images/image-uploader';
import { UploadFailed, uploadFailureFromResponse } from '../images/upload-errors';

/** The route of a part of an import (campaignpackage.proto, "Importing"), `n` from 1. */
export function importPartPath(importId: string, partNumber: number): string {
  return `/uploads/campaign-imports/${encodeURIComponent(importId)}/parts/${partNumber}`;
}

/** What the part route answers when the part was taken. */
const STATUS_NO_CONTENT = 204;

/**
 * Sends one part of a campaign import to `PUT /uploads/campaign-imports/{id}/parts/{n}` (the raw bytes of the
 * part as the body), the second route of the app that is not Connect, after `ImageUploader`, and built the same
 * way: `XMLHttpRequest` for the upload progress, same-origin so the session cookie goes with it, and errors
 * mapped by the JSON `reason`/`code` or the HTTP status, never by the message (`uploadFailureFromResponse`).
 * Aborting the `signal` cancels the part: the promise rejects with `UploadFailed('CANCELED')`.
 *
 * A part sent twice replaces itself on the server, so a retry of this call is always safe.
 */
@Injectable({ providedIn: 'root' })
export class ImportPartUploader {
  upload(
    importId: string,
    partNumber: number,
    bytes: Blob,
    options: UploadOptions = {},
  ): Promise<void> {
    return new Promise((resolve, reject) => {
      const { onProgress, signal } = options;
      if (signal?.aborted) {
        reject(new UploadFailed('CANCELED'));
        return;
      }

      const xhr = new XMLHttpRequest();
      xhr.open('PUT', importPartPath(importId, partNumber));
      xhr.responseType = 'text';

      const onAbortSignal = () => xhr.abort();
      signal?.addEventListener('abort', onAbortSignal, { once: true });
      const done = () => signal?.removeEventListener('abort', onAbortSignal);

      xhr.upload.addEventListener('progress', (e) => {
        if (e.lengthComputable && e.total > 0) {
          onProgress?.(e.loaded / e.total);
        }
      });
      xhr.addEventListener('load', () => {
        done();
        if (xhr.status === STATUS_NO_CONTENT) {
          resolve();
          return;
        }
        reject(new UploadFailed(uploadFailureFromResponse(xhr.status, xhr.responseText)));
      });
      xhr.addEventListener('error', () => {
        done();
        reject(new UploadFailed('NETWORK'));
      });
      xhr.addEventListener('timeout', () => {
        done();
        reject(new UploadFailed('NETWORK'));
      });
      xhr.addEventListener('abort', () => {
        done();
        reject(new UploadFailed('CANCELED'));
      });

      xhr.send(bytes);
    });
  }
}
