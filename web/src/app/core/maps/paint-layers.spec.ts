import { describe, expect, it } from 'vitest';

import { MapLayer } from '../../../gen/meurpg/maps/v1/maps_pb';
import { PaintedLayers } from './paint-layers';

// The byte layouts are rules/grid's (TestLayerEncoding, TestLightAndCoverLayerEncoding): a one-bit layer is bit n % 8 of
// byte n / 8, a two-bit layer is bits 2 * (n % 4) and one more of byte n / 4, both row-major.
function packed(columns: number, rows: number, extra: Partial<{ wall: Uint8Array; difficultTerrain: Uint8Array; cover: Uint8Array; light: Uint8Array; doors: Uint8Array }> = {}) {
  return { gridColumns: columns, gridRows: rows, difficultTerrain: new Uint8Array(), wall: new Uint8Array(), cover: new Uint8Array(), ...extra };
}

describe('PaintedLayers', () => {
  it('starts from what the server sent, and an empty layer is nothing painted', () => {
    const layers = new PaintedLayers();
    layers.load(packed(5, 3, { difficultTerrain: Uint8Array.of(0b1000_0001, 0b0100_0000) }));
    expect(layers.layers().terrain).toEqual([
      { col: 0, row: 0 },
      { col: 2, row: 1 },
      { col: 4, row: 2 },
    ]);
    expect(layers.layers().walls).toEqual([]);
    expect(layers.value(MapLayer.DIFFICULT_TERRAIN, 2, 1)).toBe(1);
    expect(layers.value(MapLayer.WALL, 2, 1)).toBe(0);
  });

  it('paints a wall at once, and says which squares changed', () => {
    const layers = new PaintedLayers();
    layers.load(packed(4, 4));
    const changed = layers.paint(MapLayer.WALL, 1, [
      { col: 1, row: 1 },
      { col: 2, row: 1 },
    ]);
    expect(changed).toEqual([
      { col: 1, row: 1 },
      { col: 2, row: 1 },
    ]);
    expect(layers.layers().walls).toEqual([
      { col: 1, row: 1 },
      { col: 2, row: 1 },
    ]);
  });

  it('leaves out the squares that already had the value, so a batch repeating them costs nothing', () => {
    const layers = new PaintedLayers();
    layers.load(packed(4, 4));
    layers.paint(MapLayer.WALL, 1, [{ col: 0, row: 0 }]);
    const again = layers.paint(MapLayer.WALL, 1, [
      { col: 0, row: 0 },
      { col: 3, row: 3 },
    ]);
    expect(again).toEqual([{ col: 3, row: 3 }]);
  });

  it('erases with value 0', () => {
    const layers = new PaintedLayers();
    layers.load(packed(4, 4, { wall: Uint8Array.of(0b0000_0010, 0) }));
    expect(layers.layers().walls).toEqual([{ col: 1, row: 0 }]);
    layers.paint(MapLayer.WALL, 0, [{ col: 1, row: 0 }]);
    expect(layers.layers().walls).toEqual([]);
  });

  it('keeps cover as two bits: half and three-quarters side by side', () => {
    const layers = new PaintedLayers();
    layers.load(packed(3, 2));
    layers.paint(MapLayer.COVER, 1, [{ col: 1, row: 0 }]);
    layers.paint(MapLayer.COVER, 2, [{ col: 2, row: 1 }]);
    expect(layers.layers().half).toEqual([{ col: 1, row: 0 }]);
    expect(layers.layers().threeQuarters).toEqual([{ col: 2, row: 1 }]);
    // Painting half over three-quarters replaces it.
    layers.paint(MapLayer.COVER, 1, [{ col: 2, row: 1 }]);
    expect(layers.layers().threeQuarters).toEqual([]);
    expect(layers.layers().half).toHaveLength(2);
  });

  it('keeps the painted light by level, for the master only', () => {
    const layers = new PaintedLayers();
    layers.load(packed(3, 2));
    layers.paint(MapLayer.LIGHT, 3, [{ col: 0, row: 0 }]);
    layers.paint(MapLayer.LIGHT, 2, [{ col: 1, row: 0 }]);
    layers.paint(MapLayer.LIGHT, 1, [{ col: 2, row: 0 }]);
    expect(layers.layers().light).toEqual({ bright: [{ col: 0, row: 0 }], dim: [{ col: 1, row: 0 }], dark: [{ col: 2, row: 0 }] });
    layers.paint(MapLayer.LIGHT, 0, [{ col: 1, row: 0 }]);
    expect(layers.layers().light?.dim).toEqual([]);
  });

  it('ignores a square outside the grid', () => {
    const layers = new PaintedLayers();
    layers.load(packed(2, 2));
    expect(layers.paint(MapLayer.WALL, 1, [{ col: 5, row: 0 }, { col: -1, row: 0 }, { col: 0, row: 2 }])).toEqual([]);
  });

  it('writes the same bytes the server reads: a 200 x 400 grid is 10 kB a one-bit layer', () => {
    const layers = new PaintedLayers();
    layers.load(packed(200, 400));
    layers.paint(MapLayer.WALL, 1, [{ col: 199, row: 399 }]);
    expect(layers.layers().walls).toEqual([{ col: 199, row: 399 }]);
  });

  it('clears everything for a map with no grid', () => {
    const layers = new PaintedLayers();
    layers.load(packed(3, 2, { wall: Uint8Array.of(1) }));
    layers.clear();
    expect(layers.layers().walls).toEqual([]);
    expect(layers.layers().columns).toBe(0);
  });
});

describe('PaintedLayers: the doors', () => {
  it('reads and writes four bits a square, and decodes them into doors', () => {
    const layers = new PaintedLayers();
    layers.load(packed(3, 2));
    expect(layers.paint(MapLayer.DOORS, 3, [{ col: 1, row: 1 }])).toEqual([{ col: 1, row: 1 }]);
    expect(layers.paint(MapLayer.DOORS, 5, [{ col: 0, row: 0 }])).toEqual([{ col: 0, row: 0 }]);
    expect(layers.value(MapLayer.DOORS, 1, 1)).toBe(3);
    expect(layers.value(MapLayer.DOORS, 0, 0)).toBe(5);
    expect(layers.layers().doors?.map((d) => [d.col, d.row, d.state])).toEqual([
      [0, 0, 5],
      [1, 1, 3],
    ]);
    // Painting the same value again changes nothing, and 0 clears the square.
    expect(layers.paint(MapLayer.DOORS, 3, [{ col: 1, row: 1 }])).toEqual([]);
    layers.paint(MapLayer.DOORS, 0, [{ col: 1, row: 1 }]);
    expect(layers.layers().doors?.map((d) => d.state)).toEqual([5]);
  });

  it('starts from the doors the server sent', () => {
    const layers = new PaintedLayers();
    // 3 x 2 grid: square 1 is the high nibble of byte 0 (closed), square 4 the low nibble of byte 2 (barred).
    layers.load(packed(3, 2, { doors: Uint8Array.of(0x20, 0x00, 0x04) }));
    expect(layers.value(MapLayer.DOORS, 1, 0)).toBe(2);
    expect(layers.value(MapLayer.DOORS, 1, 1)).toBe(4);
    expect(layers.value(MapLayer.DOORS, 0, 0)).toBe(0);
  });
});
