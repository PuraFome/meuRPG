import { describe, expect, it, vi } from 'vitest';

import { LayersState } from './layers-state';
import type { PackedLayers } from './layers';

function packed(wallSquare: number): PackedLayers {
  return {
    gridColumns: 4,
    gridRows: 2,
    difficultTerrain: new Uint8Array(),
    wall: Uint8Array.of(1 << wallSquare),
    cover: new Uint8Array(),
  };
}

describe('LayersState', () => {
  it('reads a map once per revision', async () => {
    const load = vi.fn(async () => packed(0));
    const state = new LayersState(load);
    await state.open('m1', 3);
    await state.open('m1', 3);
    expect(load).toHaveBeenCalledTimes(1);
    expect(state.layers().walls).toEqual([{ col: 0, row: 0 }]);
    await state.open('m1', 4);
    expect(load).toHaveBeenCalledTimes(2);
  });

  it('never lets a stale read overwrite a newer one', async () => {
    let release: (p: PackedLayers) => void = () => undefined;
    const slow = new Promise<PackedLayers>((resolve) => (release = resolve));
    const load = vi.fn().mockReturnValueOnce(slow).mockResolvedValueOnce(packed(5));
    const state = new LayersState(load);
    const first = state.open('m1', 1);
    await state.open('m1', 2);
    release(packed(0));
    await first;
    expect(state.layers().walls).toEqual([{ col: 1, row: 1 }]);
  });

  it('keeps what is on screen when a read fails, and tries again on the next change', async () => {
    const load = vi.fn().mockResolvedValueOnce(packed(0)).mockRejectedValueOnce(new Error('down')).mockResolvedValueOnce(packed(2));
    const state = new LayersState(load);
    await state.open('m1', 1);
    await state.open('m1', 2);
    expect(state.layers().walls).toEqual([{ col: 0, row: 0 }]);
    await state.open('m1', 2);
    expect(state.layers().walls).toEqual([{ col: 2, row: 0 }]);
  });

  it('clears for no map', async () => {
    const state = new LayersState(async () => packed(0));
    await state.open('m1', 1);
    await state.open(null, 0);
    expect(state.layers().walls).toEqual([]);
  });
});
