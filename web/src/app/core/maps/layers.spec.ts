import { describe, expect, it } from 'vitest';

import { NO_LAYERS, decodeLayers, hasLayers } from './layers';

// The fixtures are the byte layouts rules/grid's tests pin down
// (TestLayerEncoding, TestLightAndCoverLayerEncoding).
describe('decodeLayers', () => {
  it('reads a one-bit layer, least significant bit first', () => {
    // 5 x 3 grid: squares 0, 7 and 14 set, as TestLayerEncoding writes them.
    const layers = decodeLayers({
      gridColumns: 5,
      gridRows: 3,
      difficultTerrain: Uint8Array.of(0b1000_0001, 0b0100_0000),
      wall: new Uint8Array(),
      cover: new Uint8Array(),
    });
    expect(layers.terrain).toEqual([
      { col: 0, row: 0 },
      { col: 2, row: 1 },
      { col: 4, row: 2 },
    ]);
    expect(layers.walls).toEqual([]);
    expect(layers.columns).toBe(5);
    expect(layers.rows).toBe(3);
  });

  it('reads the two-bit cover layer: 1 is half, 2 is three-quarters', () => {
    // 3 x 2 grid, TestLightAndCoverLayerEncoding: half at (1,0), three-quarters at (2,1).
    const layers = decodeLayers({
      gridColumns: 3,
      gridRows: 2,
      difficultTerrain: new Uint8Array(),
      wall: Uint8Array.of(0b0010_0000),
      cover: Uint8Array.of(0b0000_0100, 0b0000_1000),
    });
    expect(layers.half).toEqual([{ col: 1, row: 0 }]);
    expect(layers.threeQuarters).toEqual([{ col: 2, row: 1 }]);
    expect(layers.walls).toEqual([{ col: 2, row: 1 }]);
  });

  it('treats an empty layer as nothing painted and a map without a grid as no layers', () => {
    expect(
      hasLayers(
        decodeLayers({
          gridColumns: 4,
          gridRows: 4,
          difficultTerrain: new Uint8Array(),
          wall: new Uint8Array(),
          cover: new Uint8Array(),
        }),
      ),
    ).toBe(false);
    expect(
      decodeLayers({ gridColumns: 0, gridRows: 0, difficultTerrain: new Uint8Array(), wall: new Uint8Array(), cover: new Uint8Array() }),
    ).toBe(NO_LAYERS);
  });
});
