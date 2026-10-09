import type { UploadFailureKind } from '../images/upload-errors';
import { UploadFailed } from '../images/upload-errors';

/** A failed part is sent again up to this many times before the failure card shows. */
export const PART_RETRIES = 2;

/** The wait before retry number 1, 2…: this many milliseconds times the retry number. */
const BACKOFF_STEP_MS = 800;

/** Failures a new try cannot fix: the person, the session or the upload itself is the problem. */
const FINAL_FAILURES: ReadonlySet<UploadFailureKind> = new Set([
  'CANCELED',
  'UNAUTHENTICATED',
  'PERMISSION_DENIED',
  'NOT_FOUND',
  'TOO_LARGE',
  'MALFORMED_REQUEST',
  'QUOTA',
]);

/** Whether a failed part is worth sending again: a dropped connection, a busy or restarting server. */
export function isRetryablePart(kind: UploadFailureKind): boolean {
  return !FINAL_FAILURES.has(kind);
}

/** What the server told `BeginCampaignImport`, as the sender needs it. */
export interface PartsPlan {
  readonly importId: string;
  /** The chosen file; each part is a `Blob.slice` of it. */
  readonly file: Pick<Blob, 'size' | 'slice'>;
  /** The size of every part but the last. */
  readonly partSize: number;
  readonly partCount: number;
  /** The numbers (from 1) of the parts the server already holds. They are skipped: this is the resume. */
  readonly received: readonly number[];
}

/** Where the upload is, for the screen: the part being sent and how much of the file is on the server. */
export interface PartsProgress {
  readonly part: number;
  readonly partCount: number;
  /** 0 to 1, by bytes: what the bar shows. */
  readonly fraction: number;
}

export interface PartUploader {
  upload(
    importId: string,
    partNumber: number,
    bytes: Blob,
    options: { onProgress?: (fraction: number) => void; signal?: AbortSignal },
  ): Promise<void>;
}

export interface SendPartsOptions {
  readonly uploader: PartUploader;
  readonly signal: AbortSignal;
  readonly onProgress: (progress: PartsProgress) => void;
  /** The pause between tries; a spec passes one that does not wait. Rejects when the signal aborts. */
  readonly sleep?: (ms: number, signal: AbortSignal) => Promise<void>;
}

/** Waits `ms`, or rejects with `UploadFailed('CANCELED')` as soon as the signal aborts. */
export function abortableSleep(ms: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal.aborted) {
      reject(new UploadFailed('CANCELED'));
      return;
    }
    const onAbort = () => {
      clearTimeout(timer);
      reject(new UploadFailed('CANCELED'));
    };
    const timer = setTimeout(() => {
      signal.removeEventListener('abort', onAbort);
      resolve();
    }, ms);
    signal.addEventListener('abort', onAbort, { once: true });
  });
}

/** The bytes of part `n` (from 1): `partSize` each, the last one the rest. */
export function partBlob(plan: Pick<PartsPlan, 'file' | 'partSize'>, partNumber: number): Blob {
  const start = (partNumber - 1) * plan.partSize;
  return plan.file.slice(start, Math.min(start + plan.partSize, plan.file.size));
}

/**
 * Sends the parts of an import one after another (never in parallel), skipping the numbers the server
 * already holds. A part that fails is sent again, up to `PART_RETRIES` times with a growing pause, unless the
 * failure is one a new try cannot fix; then the call rejects with the `UploadFailed` of the last try.
 * Aborting the signal stops at once with `UploadFailed('CANCELED')`.
 *
 * `fraction` counts every byte the server holds: the skipped parts from the start, the sent ones as they
 * go, and the part in flight by its own progress.
 */
export async function sendParts(plan: PartsPlan, options: SendPartsOptions): Promise<void> {
  const { uploader, signal, onProgress } = options;
  const sleep = options.sleep ?? abortableSleep;
  const held = new Set(plan.received);
  const total = Math.max(plan.file.size, 1);
  let sent = 0;
  for (let part = 1; part <= plan.partCount; part++) {
    if (signal.aborted) {
      throw new UploadFailed('CANCELED');
    }
    const bytes = partBlob(plan, part);
    if (held.has(part)) {
      sent += bytes.size;
      continue;
    }
    const report = (inPart: number) =>
      onProgress({
        part,
        partCount: plan.partCount,
        fraction: Math.min(1, (sent + inPart * bytes.size) / total),
      });
    report(0);
    await sendOnePart(plan.importId, part, bytes, { uploader, signal, sleep, report });
    sent += bytes.size;
    report(0);
  }
}

async function sendOnePart(
  importId: string,
  part: number,
  bytes: Blob,
  ctx: {
    uploader: PartUploader;
    signal: AbortSignal;
    sleep: (ms: number, signal: AbortSignal) => Promise<void>;
    report: (inPart: number) => void;
  },
): Promise<void> {
  for (let attempt = 0; ; attempt++) {
    try {
      await ctx.uploader.upload(importId, part, bytes, {
        onProgress: ctx.report,
        signal: ctx.signal,
      });
      return;
    } catch (err) {
      const kind = err instanceof UploadFailed ? err.kind : 'UNKNOWN';
      if (ctx.signal.aborted) {
        throw new UploadFailed('CANCELED');
      }
      if (attempt >= PART_RETRIES || !isRetryablePart(kind)) {
        throw err instanceof UploadFailed ? err : new UploadFailed('UNKNOWN');
      }
      ctx.report(0);
      await ctx.sleep(BACKOFF_STEP_MS * (attempt + 1), ctx.signal);
    }
  }
}
