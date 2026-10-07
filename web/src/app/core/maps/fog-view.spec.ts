import { describe, expect, it, vi } from 'vitest';

import type { PackedLayers } from './layers';
import { FogView } from './fog-view';
import { visionResponse } from './vision-testing';

const layers = (wall: number): PackedLayers => ({
  gridColumns: 2,
  gridRows: 1,
  difficultTerrain: new Uint8Array(),
  wall: Uint8Array.of(wall),
  cover: new Uint8Array(),
});

describe('FogView', () => {
  it('reads the vision and the layers together and says ready', async () => {
    const view = new FogView(
      async () => visionResponse(['Bd']),
      async () => layers(1),
    );
    expect(view.status()).toBe('idle');
    const opening = view.open('m1');
    expect(view.status()).toBe('loading');
    await opening;
    expect(view.status()).toBe('ready');
    expect(view.vision()?.columns).toBe(2);
    expect(view.layers().walls).toEqual([{ col: 0, row: 0 }]);
  });

  it('asks as the character the master chose, and again as another one', async () => {
    const loadVision = vi.fn(async (_map: string, _as: string | null) => visionResponse(['BB']));
    const view = new FogView(loadVision, async () => layers(0));
    await view.open('m1', 'toren');
    await view.open('m1', 'pensantus');
    expect(loadVision.mock.calls.map((c) => c[1])).toEqual(['toren', 'pensantus']);
  });

  const tile = (tx: number, ty: number, revision: number) => ({
    $typeName: 'meurpg.maps.v1.MapTile' as const,
    tx,
    ty,
    revision,
  });

  it('keeps the same vision while its revision and every tile are the same, so nothing is drawn again', async () => {
    const view = new FogView(
      async () =>
        visionResponse(['BB'], {
          revision: 4,
          tilesPath: '/t/',
          tileSquares: 16,
          tiles: [tile(0, 0, 2)],
        }),
      async () => layers(0),
    );
    await view.open('m1');
    const first = view.vision();
    await view.refresh();
    expect(view.vision()).toBe(first);
  });

  it('takes the new tiles when a map has a new image with the same states: same revision and count, another tile revision', async () => {
    const loadVision = vi
      .fn()
      .mockResolvedValueOnce(
        visionResponse(['BB'], {
          revision: 4,
          tilesPath: '/t/',
          tileSquares: 16,
          tiles: [tile(0, 0, 2)],
        }),
      )
      .mockResolvedValueOnce(
        visionResponse(['BB'], {
          revision: 4,
          tilesPath: '/t/',
          tileSquares: 16,
          tiles: [tile(0, 0, 7)],
        }),
      );
    const view = new FogView(loadVision, async () => layers(0));
    await view.open('m1');
    await view.refresh();
    expect(view.vision()?.tiles[0].revision).toBe(7);
  });

  it('says why a read failed, by the code: the character is gone, the map has no grid, the server is away', async () => {
    const { ConnectError } = await import('@connectrpc/connect');
    const read = async (code: number) => {
      const view = new FogView(
        async () => Promise.reject(new ConnectError('x', code)),
        async () => layers(0),
      );
      await view.open('m1', 'toren');
      return view.error();
    };
    expect(await read(5)).toBe('gone');
    expect(await read(9)).toBe('no-grid');
    expect(await read(14)).toBe('offline');
    expect(await read(13)).toBe('failed');
  });

  it('takes a new vision when the revision changes', async () => {
    const loadVision = vi
      .fn()
      .mockResolvedValueOnce(visionResponse(['dd'], { revision: 1 }))
      .mockResolvedValueOnce(visionResponse(['BB'], { revision: 2 }));
    const view = new FogView(loadVision, async () => layers(0));
    await view.open('m1');
    await view.refresh();
    expect(view.vision()?.revision).toBe(2);
  });

  it('never lets a stale read overwrite a newer one', async () => {
    let release: (r: ReturnType<typeof visionResponse>) => void = () => undefined;
    const slow = new Promise<ReturnType<typeof visionResponse>>((resolve) => (release = resolve));
    const loadVision = vi
      .fn()
      .mockReturnValueOnce(slow)
      .mockResolvedValueOnce(visionResponse(['BB'], { revision: 9 }));
    const view = new FogView(loadVision, async () => layers(0));
    const first = view.open('m1');
    await view.refresh();
    release(visionResponse(['dd'], { revision: 1 }));
    await first;
    expect(view.vision()?.revision).toBe(9);
  });

  it('keeps what is on screen when a later read fails, and says error when the first one does', async () => {
    const loadVision = vi
      .fn()
      .mockResolvedValueOnce(visionResponse(['BB']))
      .mockRejectedValue(new Error('down'));
    const view = new FogView(loadVision, async () => layers(0));
    await view.open('m1');
    await view.refresh();
    expect(view.status()).toBe('ready');
    expect(view.vision()).not.toBeNull();

    const broken = new FogView(
      async () => Promise.reject(new Error('down')),
      async () => layers(0),
    );
    await broken.open('m1');
    expect(broken.status()).toBe('error');
  });

  it('closes for no map', async () => {
    const view = new FogView(
      async () => visionResponse(['BB']),
      async () => layers(0),
    );
    await view.open('m1');
    await view.open(null);
    expect(view.status()).toBe('idle');
    expect(view.vision()).toBeNull();
  });
});
