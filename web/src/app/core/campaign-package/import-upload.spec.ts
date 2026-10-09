import { UploadFailed } from '../images/upload-errors';
import { FakePartUploader } from './campaign-package-testing';
import {
  PART_RETRIES,
  type PartsPlan,
  type PartsProgress,
  abortableSleep,
  isRetryablePart,
  partBlob,
  sendParts,
} from './import-upload';

/** A 35-byte file in parts of 10: four parts, the last one 5 bytes. */
function plan(received: number[] = []): PartsPlan {
  return {
    importId: 'imp-1',
    file: new Blob(['x'.repeat(35)]),
    partSize: 10,
    partCount: 4,
    received,
  };
}

describe('sendParts', () => {
  let uploader: FakePartUploader;
  let controller: AbortController;
  let progress: PartsProgress[];
  let pauses: number[];
  const sleep = (ms: number) => {
    pauses.push(ms);
    return Promise.resolve();
  };

  const run = (p: PartsPlan) =>
    sendParts(p, {
      uploader,
      signal: controller.signal,
      onProgress: (x) => progress.push(x),
      sleep,
    });

  beforeEach(() => {
    uploader = new FakePartUploader();
    controller = new AbortController();
    progress = [];
    pauses = [];
  });

  it('sends the parts one after another, in order, with the bytes of each', async () => {
    await run(plan());
    expect(uploader.partNumbers).toEqual([1, 2, 3, 4]);
    expect(uploader.sent.map((s) => s.size)).toEqual([10, 10, 10, 5]);
    expect(uploader.sent.every((s) => s.importId === 'imp-1')).toBe(true);
  });

  it('never has two parts in flight', async () => {
    let inFlight = 0;
    let most = 0;
    uploader.behave = async () => {
      inFlight++;
      most = Math.max(most, inFlight);
      await Promise.resolve();
      inFlight--;
    };
    await run(plan());
    expect(most).toBe(1);
  });

  it('skips the parts the server already holds: this is the resume', async () => {
    await run(plan([1, 2, 4]));
    expect(uploader.partNumbers).toEqual([3]);
  });

  it('sends nothing when every part is already there', async () => {
    await run(plan([1, 2, 3, 4]));
    expect(uploader.sent).toEqual([]);
  });

  it('reports the part being sent and the share of the bytes the server holds', async () => {
    await run(plan([1]));
    expect(progress[0]).toEqual({ part: 2, partCount: 4, fraction: 10 / 35 });
    const last = progress[progress.length - 1];
    expect(last.fraction).toBe(1);
    // Half-way through part 2 (the fake reports 0.5), part 1 counted from the start.
    expect(progress.some((p) => p.part === 2 && p.fraction === 15 / 35)).toBe(true);
    expect(progress.every((p) => p.fraction >= 0 && p.fraction <= 1)).toBe(true);
  });

  it('sends a failed part again, with a growing pause, and goes on', async () => {
    uploader.behave = (call, attempt) =>
      call.part === 2 && attempt < PART_RETRIES
        ? Promise.reject(new UploadFailed('NETWORK'))
        : Promise.resolve();
    await run(plan());
    expect(uploader.partNumbers).toEqual([1, 2, 2, 2, 3, 4]);
    expect(pauses).toEqual([800, 1600]);
  });

  it('gives up after two retries with the failure of the last try', async () => {
    uploader.behave = () => Promise.reject(new UploadFailed('UNAVAILABLE'));
    await expect(run(plan())).rejects.toEqual(new UploadFailed('UNAVAILABLE'));
    expect(uploader.partNumbers).toEqual([1, 1, 1]);
    expect(pauses).toHaveLength(PART_RETRIES);
  });

  it('does not send the parts after the one that failed for good', async () => {
    uploader.behave = (call) =>
      call.part === 2 ? Promise.reject(new UploadFailed('NETWORK')) : Promise.resolve();
    await expect(run(plan())).rejects.toMatchObject({ kind: 'NETWORK' });
    expect(uploader.partNumbers).toEqual([1, 2, 2, 2]);
  });

  it('does not retry a failure a new try cannot fix', async () => {
    uploader.behave = () => Promise.reject(new UploadFailed('PERMISSION_DENIED'));
    await expect(run(plan())).rejects.toMatchObject({ kind: 'PERMISSION_DENIED' });
    expect(uploader.partNumbers).toEqual([1]);
    expect(pauses).toEqual([]);
  });

  it('stops at once when cancelled during a part, with no retry', async () => {
    uploader.behave = (call) => {
      if (call.part === 2) {
        controller.abort();
        return Promise.reject(new UploadFailed('CANCELED'));
      }
      return Promise.resolve();
    };
    await expect(run(plan())).rejects.toMatchObject({ kind: 'CANCELED' });
    expect(uploader.partNumbers).toEqual([1, 2]);
    expect(pauses).toEqual([]);
  });

  it('does not start a part once the signal is aborted, even when only skipping', async () => {
    controller.abort();
    await expect(run(plan([1, 2, 3, 4]))).rejects.toMatchObject({ kind: 'CANCELED' });
    expect(uploader.sent).toEqual([]);
  });

  it('turns a failure that is not an UploadFailed into UNKNOWN', async () => {
    uploader.behave = () => Promise.reject(new Error('boom'));
    await expect(run(plan())).rejects.toMatchObject({ kind: 'UNKNOWN' });
    expect(uploader.partNumbers).toEqual([1, 1, 1]);
  });
});

describe('partBlob', () => {
  it('slices partSize bytes from the part number, the last one the rest', () => {
    const p = plan();
    expect(partBlob(p, 1).size).toBe(10);
    expect(partBlob(p, 3).size).toBe(10);
    expect(partBlob(p, 4).size).toBe(5);
  });
});

describe('isRetryablePart', () => {
  it('retries what a dropped connection or a busy server causes, not a refusal', () => {
    for (const kind of ['NETWORK', 'UNAVAILABLE', 'UNKNOWN', 'RATE_LIMITED', 'CORRUPT'] as const) {
      expect(isRetryablePart(kind), kind).toBe(true);
    }
    for (const kind of [
      'CANCELED',
      'UNAUTHENTICATED',
      'PERMISSION_DENIED',
      'NOT_FOUND',
      'TOO_LARGE',
      'QUOTA',
      'MALFORMED_REQUEST',
    ] as const) {
      expect(isRetryablePart(kind), kind).toBe(false);
    }
  });
});

describe('abortableSleep', () => {
  afterEach(() => vi.useRealTimers());

  it('waits the time, on a fake clock', async () => {
    vi.useFakeTimers();
    const done = vi.fn();
    void abortableSleep(800, new AbortController().signal).then(done);
    await vi.advanceTimersByTimeAsync(799);
    expect(done).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(1);
    expect(done).toHaveBeenCalled();
  });

  it('rejects with CANCELED as soon as the signal aborts, and when it already has', async () => {
    vi.useFakeTimers();
    const controller = new AbortController();
    const waiting = abortableSleep(800, controller.signal);
    controller.abort();
    await expect(waiting).rejects.toMatchObject({ kind: 'CANCELED' });
    await expect(abortableSleep(800, controller.signal)).rejects.toMatchObject({
      kind: 'CANCELED',
    });
  });
});
