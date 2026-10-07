import { describe, expect, it } from 'vitest';

import { MapLayer } from '../../../gen/meurpg/maps/v1/maps_pb';
import { type MapLayers, NO_LAYERS } from './layers';
import {
  DEFAULT_SETTINGS,
  brushSquares,
  layerLines,
  lineSquares,
  paintHint,
  strokeOf,
} from './paint-tools';

describe('strokeOf', () => {
  it('paints 1 for terrain and walls', () => {
    expect(strokeOf({ ...DEFAULT_SETTINGS, tool: 'terrain' })).toEqual({
      layer: MapLayer.DIFFICULT_TERRAIN,
      value: 1,
    });
    expect(strokeOf({ ...DEFAULT_SETTINGS, tool: 'wall' })).toEqual({
      layer: MapLayer.WALL,
      value: 1,
    });
  });

  it('paints the chosen degree of cover and of light', () => {
    expect(strokeOf({ ...DEFAULT_SETTINGS, tool: 'cover', cover: 2 })).toEqual({
      layer: MapLayer.COVER,
      value: 2,
    });
    expect(strokeOf({ ...DEFAULT_SETTINGS, tool: 'light', light: 3 })).toEqual({
      layer: MapLayer.LIGHT,
      value: 3,
    });
  });

  it('paints the chosen door in the doors layer, and 0 takes it away', () => {
    expect(strokeOf({ ...DEFAULT_SETTINGS, tool: 'door', door: 3 })).toEqual({
      layer: MapLayer.DOORS,
      value: 3,
    });
    expect(strokeOf({ ...DEFAULT_SETTINGS, tool: 'door', door: 3, erase: true })).toEqual({
      layer: MapLayer.DOORS,
      value: 0,
    });
  });

  it("erases the chosen tool's layer with 0", () => {
    expect(strokeOf({ ...DEFAULT_SETTINGS, tool: 'cover', cover: 2, erase: true })).toEqual({
      layer: MapLayer.COVER,
      value: 0,
    });
    expect(strokeOf({ ...DEFAULT_SETTINGS, tool: 'light', erase: true })).toEqual({
      layer: MapLayer.LIGHT,
      value: 0,
    });
  });
});

describe('brushSquares', () => {
  it('is the square itself for 1 x 1', () => {
    expect(brushSquares({ col: 3, row: 2 }, 1, 10, 10)).toEqual([{ col: 3, row: 2 }]);
  });

  it('is nine squares for 3 x 3, kept inside the grid at an edge', () => {
    expect(brushSquares({ col: 5, row: 5 }, 3, 10, 10)).toHaveLength(9);
    expect(brushSquares({ col: 0, row: 0 }, 3, 10, 10)).toEqual([
      { col: 0, row: 0 },
      { col: 1, row: 0 },
      { col: 0, row: 1 },
      { col: 1, row: 1 },
    ]);
  });
});

describe('lineSquares', () => {
  it('leaves no square out between two pointer events, ends included', () => {
    expect(lineSquares({ col: 0, row: 0 }, { col: 4, row: 0 })).toEqual(
      [0, 1, 2, 3, 4].map((col) => ({ col, row: 0 })),
    );
    const diagonal = lineSquares({ col: 0, row: 0 }, { col: 3, row: 3 });
    expect(diagonal).toHaveLength(4);
    expect(diagonal[3]).toEqual({ col: 3, row: 3 });
  });

  it('walks backwards too, and is one square for the same square', () => {
    expect(lineSquares({ col: 3, row: 1 }, { col: 1, row: 1 })).toEqual(
      [3, 2, 1].map((col) => ({ col, row: 1 })),
    );
    expect(lineSquares({ col: 2, row: 2 }, { col: 2, row: 2 })).toEqual([{ col: 2, row: 2 }]);
  });
});

describe('paintHint', () => {
  it('says the tool, how to paint and how to erase', () => {
    expect(paintHint({ ...DEFAULT_SETTINGS, tool: 'terrain' })).toBe(
      'Terreno difícil · arraste para pintar · Shift apaga',
    );
    expect(paintHint({ ...DEFAULT_SETTINGS, tool: 'cover', cover: 2 })).toBe(
      'Cobertura · Três quartos · arraste para pintar · Shift apaga',
    );
    expect(paintHint({ ...DEFAULT_SETTINGS, tool: 'light', light: 3, erase: true })).toBe(
      'Luz · arraste para apagar',
    );
  });

  it('says the kind of door and that a tap puts it', () => {
    expect(paintHint({ ...DEFAULT_SETTINGS, tool: 'door', door: 5 })).toBe(
      'Porta · Secreta · toque para pôr · Shift tira',
    );
    expect(paintHint({ ...DEFAULT_SETTINGS, tool: 'door', erase: true })).toBe(
      'Porta · toque numa porta para tirá-la',
    );
  });
});

/** The words use no-break spaces to keep a number with its unit. */
const plain = (text: string) => text.replace(/\u00a0/g, ' ');

describe('layerLines', () => {
  const some: MapLayers = {
    columns: 24,
    rows: 16,
    walls: [{ col: 0, row: 0 }],
    terrain: [
      { col: 1, row: 1 },
      { col: 2, row: 1 },
    ],
    half: [
      { col: 3, row: 3 },
      { col: 4, row: 3 },
    ],
    threeQuarters: [{ col: 5, row: 5 }],
    light: { dark: [], dim: [{ col: 6, row: 6 }], bright: [] },
  };

  it('counts what each layer holds, in the words of the artboard', () => {
    const lines = layerLines(some);
    expect(lines.map((l) => l.name)).toEqual([
      'Terreno difícil',
      'Parede',
      'Cobertura',
      'Luz',
      'Portas',
    ]);
    expect(lines[4].detail).toBe('nada pintado');
    expect(plain(lines[0].detail)).toContain('2 quadrados');
    expect(plain(lines[0].detail)).toContain('custa +1,5 m por quadrado');
    expect(plain(lines[1].detail)).toContain('1 quadrado');
    expect(plain(lines[1].detail)).toContain('bloqueia movimento, visão e luz');
    expect(plain(lines[2].detail)).toBe('2 quadrados de meia cobertura e 1 de três quartos');
    expect(plain(lines[3].detail)).toBe('1 quadrado pintado');
  });

  it('says "nada pintado" for an empty layer and never names an object for cover', () => {
    const lines = layerLines(NO_LAYERS);
    expect(lines.every((l) => l.detail === 'nada pintado')).toBe(true);
    expect(plain(layerLines({ ...NO_LAYERS, half: [{ col: 0, row: 0 }] })[2].detail)).toBe(
      '1 quadrado de meia cobertura',
    );
  });
});
