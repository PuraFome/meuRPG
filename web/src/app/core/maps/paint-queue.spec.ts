import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { MapLayer } from '../../../gen/meurpg/maps/v1/maps_pb';
import { MAX_PAINT_BATCH, PaintQueue } from './paint-queue';

interface Call {
  layer: MapLayer;
  value: number;
  n: number;
}

describe('PaintQueue', () => {
  let calls: Call[];
  let fail: boolean;
  let queue: PaintQueue;

  beforeEach(() => {
    vi.useFakeTimers();
    calls = [];
    fail = false;
    queue = new PaintQueue(async (layer, value, squares) => {
      if (fail) {
        throw new Error('refused');
      }
      calls.push({ layer, value, n: squares.length });
    }, 100);
  });

  afterEach(() => vi.useRealTimers());

  const sq = (n: number) => Array.from({ length: n }, (_, i) => ({ col: i % 200, row: Math.floor(i / 200) }));

  it('is "Tudo salvo" while nothing waits', () => {
    expect(queue.status()).toBe('saved');
    expect(queue.busy).toBe(false);
  });

  it('joins the squares of a drag in one call and sends it a moment later', async () => {
    queue.add(MapLayer.WALL, 1, [{ col: 0, row: 0 }]);
    queue.add(MapLayer.WALL, 1, [{ col: 1, row: 0 }]);
    queue.add(MapLayer.WALL, 1, [{ col: 1, row: 0 }, { col: 2, row: 0 }]);
    expect(queue.status()).toBe('saving');
    expect(calls).toEqual([]);
    await vi.advanceTimersByTimeAsync(150);
    expect(calls).toEqual([{ layer: MapLayer.WALL, value: 1, n: 3 }]);
    expect(queue.status()).toBe('saved');
  });

  it('sends at once on flush, at the end of a stroke', async () => {
    queue.add(MapLayer.DIFFICULT_TERRAIN, 1, [{ col: 0, row: 0 }]);
    await queue.flush();
    expect(calls).toHaveLength(1);
    expect(queue.status()).toBe('saved');
  });

  it('keeps the order of the strokes: a wall, then its erasure, are two calls in that order (last write wins)', async () => {
    queue.add(MapLayer.WALL, 1, [{ col: 0, row: 0 }]);
    queue.add(MapLayer.WALL, 0, [{ col: 0, row: 0 }]);
    queue.add(MapLayer.COVER, 2, [{ col: 1, row: 0 }]);
    await queue.flush();
    expect(calls.map((c) => `${c.layer}:${c.value}`)).toEqual([`${MapLayer.WALL}:1`, `${MapLayer.WALL}:0`, `${MapLayer.COVER}:2`]);
  });

  it('splits a long stroke at 400 squares a call', async () => {
    queue.add(MapLayer.WALL, 1, sq(MAX_PAINT_BATCH * 2 + 50));
    await queue.flush();
    expect(calls.map((c) => c.n)).toEqual([400, 400, 50]);
  });

  it('keeps a refused batch, says so, and sends it again on retry', async () => {
    fail = true;
    queue.add(MapLayer.WALL, 1, sq(3));
    await queue.flush();
    expect(queue.status()).toBe('error');
    expect(queue.failure()).toBeInstanceOf(Error);
    expect(queue.busy).toBe(true);
    fail = false;
    await queue.retry();
    expect(calls).toEqual([{ layer: MapLayer.WALL, value: 1, n: 3 }]);
    expect(queue.status()).toBe('saved');
  });

  it('puts a stroke made while a batch is on its way in a batch of its own', async () => {
    queue.add(MapLayer.WALL, 1, sq(2));
    const first = queue.flush();
    queue.add(MapLayer.WALL, 1, [{ col: 9, row: 9 }]);
    await first;
    await queue.flush();
    expect(calls.map((c) => c.n)).toEqual([2, 1]);
  });
});
