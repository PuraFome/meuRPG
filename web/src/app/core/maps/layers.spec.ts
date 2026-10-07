import { describe, expect, it } from 'vitest';

import { NO_LAYERS, decodeLayers, doorCounts, hasLayers, hasPainted, lightCount } from './layers';

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
      decodeLayers({
        gridColumns: 0,
        gridRows: 0,
        difficultTerrain: new Uint8Array(),
        wall: new Uint8Array(),
        cover: new Uint8Array(),
      }),
    ).toBe(NO_LAYERS);
  });

  it("reads the master's painted light by level (1 Escuro, 2 Penumbra, 3 Claro), two bits a square", () => {
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
    const none = decodeLayers({
      gridColumns: 3,
      gridRows: 2,
      difficultTerrain: new Uint8Array(),
      wall: new Uint8Array(),
      cover: new Uint8Array(),
      light: new Uint8Array(),
    });
    expect(none.light).toBeUndefined();
    expect(hasPainted(none)).toBe(false);
    const lit = decodeLayers({
      gridColumns: 3,
      gridRows: 2,
      difficultTerrain: new Uint8Array(),
      wall: new Uint8Array(),
      cover: new Uint8Array(),
      light: Uint8Array.of(0b0000_0011),
    });
    expect(hasLayers(lit)).toBe(false);
    expect(hasPainted(lit)).toBe(true);
  });
});

// The doors layer is four bits a square (rules/grid's DoorLayer): square n is the low nibble of byte n / 2 when n is even and the high nibble when odd.
describe('decodeLayers: the doors (RN-26)', () => {
  const empty = { difficultTerrain: new Uint8Array(), cover: new Uint8Array() };

  it('reads a nibble a square: 1 open, 2 closed, 3 locked, 4 barred, 5 secret', () => {
    // 3 x 2 grid: square 0 closed (2), 1 open (1), 2 locked (3), 3 barred (4), 4 secret (5), 5 nothing.
    const layers = decodeLayers({
      ...empty,
      gridColumns: 3,
      gridRows: 2,
      wall: new Uint8Array(),
      doors: Uint8Array.of(0x12, 0x43, 0x05),
    });
    expect(layers.doors?.map((d) => [d.col, d.row, d.state])).toEqual([
      [0, 0, 2],
      [1, 0, 1],
      [2, 0, 3],
      [0, 1, 4],
      [1, 1, 5],
    ]);
    expect(doorCounts(layers)).toEqual({ 1: 1, 2: 1, 3: 1, 4: 1, 5: 1 });
    expect(hasLayers(layers)).toBe(true);
  });

  it('has no doors when the layer is empty, and a map with doors only still has layers to name', () => {
    const none = decodeLayers({
      ...empty,
      gridColumns: 3,
      gridRows: 2,
      wall: new Uint8Array(),
      doors: new Uint8Array(),
    });
    expect(none.doors).toBeUndefined();
    expect(hasLayers(none)).toBe(false);
    expect(doorCounts(none)).toEqual({ 1: 0, 2: 0, 3: 0, 4: 0, 5: 0 });
  });

  it('turns the bar by the floor on both sides, as planDoor does: floor above and below keeps it across (h), floor left and right turns it (v)', () => {
    // 3 x 3 grid, a closed door in the middle (square 4, the low nibble of byte 2).
    const door = Uint8Array.of(0, 0, 0x02, 0, 0);
    // Walls at (0,1) and (2,1), left and right of the door: floor above and below, the passage runs up and down.
    const sides = decodeLayers({
      ...empty,
      gridColumns: 3,
      gridRows: 3,
      wall: Uint8Array.of(0b0010_1000, 0),
      doors: door,
    });
    expect(sides.doors).toEqual([{ col: 1, row: 1, state: 2, axis: 'h' }]);
    // Walls at (1,0) and (1,2), above and below: floor left and right, the bar is turned.
    const ends = decodeLayers({
      ...empty,
      gridColumns: 3,
      gridRows: 3,
      wall: Uint8Array.of(0b1000_0010, 0),
      doors: door,
    });
    expect(ends.doors).toEqual([{ col: 1, row: 1, state: 2, axis: 'v' }]);
    // A corner (a wall on two sides) does not flip it: two doors of one corridor point the same way.
    const corner = decodeLayers({
      ...empty,
      gridColumns: 3,
      gridRows: 3,
      wall: Uint8Array.of(0b0000_1010, 0),
      doors: door,
    });
    expect(corner.doors?.[0].axis).toBe('h');
  });

  it("guards a player's view (RN-10): a locked door is drawn closed and a secret one is not drawn, even if the layer carried them", () => {
    // Squares 0 (locked), 1 (secret) and 2 (open) of a 3 x 1 grid.
    const packed = {
      ...empty,
      gridColumns: 3,
      gridRows: 1,
      wall: new Uint8Array(),
      doors: Uint8Array.of(0x53, 0x01),
    };
    expect(decodeLayers(packed).doors?.map((d) => d.state)).toEqual([3, 5, 1]);
    expect(decodeLayers(packed, true).doors?.map((d) => [d.col, d.state])).toEqual([
      [0, 2],
      [2, 1],
    ]);
  });
});
