import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { MapLayer } from '../../../gen/meurpg/maps/v1/maps_pb';
import { Code, ConnectError } from '@connectrpc/connect';

import { MAX_PAINT_BATCH, PaintQueue, type PaintTarget, isTransient } from './paint-queue';

interface Call {
  mapId: string;
  layer: MapLayer;
  value: number;
  n: number;
}

describe('PaintQueue', () => {
  let calls: Call[];
  let fail: unknown;
  let queue: PaintQueue;

  beforeEach(() => {
    vi.useFakeTimers();
    calls = [];
    fail = null;
    queue = new PaintQueue(async (target, layer, value, squares) => {
      if (fail) {
        throw fail;
      }
      calls.push({ mapId: target.mapId, layer, value, n: squares.length });
    }, 100);
  });

  afterEach(() => vi.useRealTimers());

  const A: PaintTarget = { campaignId: 'c', mapId: 'a' };
  const B: PaintTarget = { campaignId: 'c', mapId: 'b' };
  const sq = (n: number) => Array.from({ length: n }, (_, i) => ({ col: i % 200, row: Math.floor(i / 200) }));

  it('is "Tudo salvo" while nothing waits', () => {
    expect(queue.status()).toBe('saved');
    expect(queue.busy).toBe(false);
  });

  it('joins the squares of a drag in one call and sends it a moment later', async () => {
    queue.add(A, MapLayer.WALL, 1, [{ col: 0, row: 0 }]);
    queue.add(A, MapLayer.WALL, 1, [{ col: 1, row: 0 }]);
    queue.add(A, MapLayer.WALL, 1, [{ col: 1, row: 0 }, { col: 2, row: 0 }]);
    expect(queue.status()).toBe('saving');
    expect(calls).toEqual([]);
    await vi.advanceTimersByTimeAsync(150);
    expect(calls).toEqual([{ mapId: 'a', layer: MapLayer.WALL, value: 1, n: 3 }]);
    expect(queue.status()).toBe('saved');
  });

  it('sends at once on flush, at the end of a stroke', async () => {
    queue.add(A, MapLayer.DIFFICULT_TERRAIN, 1, [{ col: 0, row: 0 }]);
    await queue.flush();
    expect(calls).toHaveLength(1);
    expect(queue.status()).toBe('saved');
  });

  it('keeps the order of the strokes: a wall, then its erasure, are two calls in that order (last write wins)', async () => {
    queue.add(A, MapLayer.WALL, 1, [{ col: 0, row: 0 }]);
    queue.add(A, MapLayer.WALL, 0, [{ col: 0, row: 0 }]);
    queue.add(A, MapLayer.COVER, 2, [{ col: 1, row: 0 }]);
    await queue.flush();
    expect(calls.map((c) => `${c.layer}:${c.value}`)).toEqual([`${MapLayer.WALL}:1`, `${MapLayer.WALL}:0`, `${MapLayer.COVER}:2`]);
  });

  it('splits a long stroke at 400 squares a call', async () => {
    queue.add(A, MapLayer.WALL, 1, sq(MAX_PAINT_BATCH * 2 + 50));
    await queue.flush();
    expect(calls.map((c) => c.n)).toEqual([400, 400, 50]);
  });

  it('keeps a batch the server did not answer, says so, and sends it again on retry', async () => {
    fail = new ConnectError('down', Code.Unavailable);
    queue.add(A, MapLayer.WALL, 1, sq(3));
    await queue.flush();
    expect(queue.status()).toBe('error');
    expect(queue.failure()).toBeInstanceOf(ConnectError);
    expect(queue.retryable()).toBe(true);
    expect(queue.busy).toBe(true);
    fail = null;
    await queue.retry();
    expect(calls).toEqual([{ mapId: 'a', layer: MapLayer.WALL, value: 1, n: 3 }]);
    expect(queue.status()).toBe('saved');
  });

  it('puts a stroke made while a batch is on its way in a batch of its own', async () => {
    queue.add(A, MapLayer.WALL, 1, sq(2));
    const first = queue.flush();
    queue.add(A, MapLayer.WALL, 1, [{ col: 9, row: 9 }]);
    await first;
    await queue.flush();
    expect(calls.map((c) => c.n)).toEqual([2, 1]);
  });

  it('sends a stroke to the map it was made on, even when the editor has moved to another', async () => {
    queue.add(A, MapLayer.WALL, 1, sq(2));
    queue.add(B, MapLayer.WALL, 1, sq(1));
    await queue.flush();
    expect(calls.map((c) => `${c.mapId}:${c.n}`)).toEqual(['a:2', 'b:1']);
  });

  it('drops what waits on a refusal no retry fixes, and says the editor must read the layers again', async () => {
    fail = new ConnectError('gone', Code.NotFound);
    queue.add(A, MapLayer.WALL, 1, sq(2));
    queue.add(A, MapLayer.WALL, 0, sq(1));
    await queue.flush();
    expect(queue.status()).toBe('error');
    expect(queue.retryable()).toBe(false);
    expect(queue.busy).toBe(false);
    expect(queue.refused()).toBe(1);
  });

  it('forgets what waits when the grid changes (clear)', async () => {
    queue.add(A, MapLayer.WALL, 1, sq(2));
    queue.clear();
    expect(queue.busy).toBe(false);
    expect(queue.status()).toBe('saved');
    await queue.flush();
    expect(calls).toEqual([]);
  });

  it('runs a callback when the queue goes quiet (the layers are read again then)', async () => {
    const seen: string[] = [];
    queue.add(A, MapLayer.WALL, 1, sq(1));
    queue.whenIdle(() => seen.push('idle'));
    expect(seen).toEqual([]);
    await queue.flush();
    expect(seen).toEqual(['idle']);
    queue.whenIdle(() => seen.push('now'));
    expect(seen).toEqual(['idle', 'now']);
  });

  it('calls only the failures a retry can fix transient', () => {
    expect(isTransient(new ConnectError('x', Code.Unavailable))).toBe(true);
    expect(isTransient(new ConnectError('x', Code.DeadlineExceeded))).toBe(true);
    expect(isTransient(new ConnectError('x', Code.Aborted))).toBe(true);
    expect(isTransient(new TypeError('Failed to fetch'))).toBe(true);
    expect(isTransient(new ConnectError('x', Code.NotFound))).toBe(false);
    expect(isTransient(new ConnectError('x', Code.PermissionDenied))).toBe(false);
    expect(isTransient(new ConnectError('x', Code.FailedPrecondition))).toBe(false);
  });
});
