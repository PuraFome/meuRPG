import { describe, expect, it } from 'vitest';

import { NO_LAYERS, decodeLayers, hasLayers, hasPainted, lightCount } from './layers';

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

  it('reads the master\'s painted light by level (1 Escuro, 2 Penumbra, 3 Claro), two bits a square', () => {
    // 3 x 2 grid: square 0 is Claro (3), square 1 Penumbra (2), square 4 Escuro (1).
    const layers = decodeLayers({
      gridColumns: 3,
      gridRows: 2,
      difficultTerrain: new Uint8Array(),
      wall: new Uint8Array(),
      cover: new Uint8Array(),
      light: Uint8Array.of(0b0000_1011, 0b0000_0001),
    });
    expect(layers.light?.bright).toEqual([{ col: 0, row: 0 }]);
    expect(layers.light?.dim).toEqual([{ col: 1, row: 0 }]);
    expect(layers.light?.dark).toEqual([{ col: 1, row: 1 }]);
    expect(lightCount(layers)).toBe(3);
  });

  it('has no light for a player (the layer comes empty) and counts painted light as painted', () => {
    const none = decodeLayers({ gridColumns: 3, gridRows: 2, difficultTerrain: new Uint8Array(), wall: new Uint8Array(), cover: new Uint8Array(), light: new Uint8Array() });
    expect(none.light).toBeUndefined();
    expect(hasPainted(none)).toBe(false);
    const lit = decodeLayers({ gridColumns: 3, gridRows: 2, difficultTerrain: new Uint8Array(), wall: new Uint8Array(), cover: new Uint8Array(), light: Uint8Array.of(0b0000_0011) });
    expect(hasLayers(lit)).toBe(false);
    expect(hasPainted(lit)).toBe(true);
  });
});
