import { Injectable } from '@angular/core';
import { fromJson, type JsonValue } from '@bufbuild/protobuf';

import { type GalleryImage, GalleryImageSchema } from '../../../gen/meurpg/maps/v1/gallery_pb';
import { UploadFailed, uploadFailureFromResponse } from './upload-errors';

export interface UploadOptions {
  /** Called as the file goes out, with 0 to 1. At 1 the server is still
   * working (checking, encoding again, making the thumbnail). */
  readonly onProgress?: (fraction: number) => void;
  /** Aborting it cancels the upload; the promise rejects with
   * `UploadFailed('CANCELED')`. */
  readonly signal?: AbortSignal;
}

/**
 * Sends one image to `POST /uploads/images` (gallery.proto), the one route
 * of the app that is not Connect, because its body is a file.
 *
 * `XMLHttpRequest`, not `fetch`: only XHR reports upload progress, which the
 * gallery's progress bar needs. There is no `HttpClient` in the app (every
 * other call is Connect), and adding it for one request would be heavier
 * than these lines. The request is same-origin, so the session cookie goes
 * with it, and the browser's own `Origin`/`Sec-Fetch-Site` headers satisfy
 * the server's CSRF guard (docs/architecture.md#csrf).
 *
 * The form's fields go in the order the server requires: `campaign_id`,
 * then `file`. The browser sets the multipart `Content-Type` with its
 * boundary: setting it here would break the body.
 *
 * `providedIn: 'root'` is safe for the bundle: only lazy pages import it,
 * so it and the generated gallery code land in their chunk.
 */
@Injectable({ providedIn: 'root' })
export class ImageUploader {
  upload(campaignId: string, file: File, options: UploadOptions = {}): Promise<GalleryImage> {
    return new Promise((resolve, reject) => {
      const { onProgress, signal } = options;
      if (signal?.aborted) {
        reject(new UploadFailed('CANCELED'));
        return;
      }

      const xhr = new XMLHttpRequest();
      xhr.open('POST', '/uploads/images');
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
        if (xhr.status === 201) {
          try {
            const json = JSON.parse(xhr.responseText) as JsonValue;
            resolve(fromJson(GalleryImageSchema, json, { ignoreUnknownFields: true }));
          } catch {
            reject(new UploadFailed('UNKNOWN'));
          }
          return;
        }
        reject(new UploadFailed(uploadFailureFromResponse(xhr.status, xhr.responseText)));
      });
      // A dropped connection, or the server closing it before reading the
      // whole body.
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

      const form = new FormData();
      form.append('campaign_id', campaignId);
      form.append('file', file, file.name);
      xhr.send(form);
    });
  }
}
