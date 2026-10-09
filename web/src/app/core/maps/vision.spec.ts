import { describe, expect, it } from 'vitest';

import {
  decodeVision,
  knownWindow,
  seenCount,
  shadeRects,
  shadeRuns,
  tileProgress,
  tileRects,
  tileUrl,
  unpackStates,
  visionLegend,
} from './vision';
import { packStates, statesOf, visionResponse } from './vision-testing';

describe('unpackStates', () => {
  it('reads four bits a square, the even square in the low bits, and ignores the unused half byte', () => {
    // squares 0..4 = 4, 5, 2, 0, 3 -> bytes 0x54, 0x02, 0x03
    expect(Array.from(unpackStates(Uint8Array.of(0x54, 0x02, 0x03), 5, 1))).toEqual([
      4, 5, 2, 0, 3,
    ]);
    expect(Array.from(unpackStates(packStates([1, 2, 3, 4, 5, 0]), 3, 2))).toEqual([
      1, 2, 3, 4, 5, 0,
    ]);
  });

  it('gives a grid of unseen squares for bytes that are missing', () => {
    expect(Array.from(unpackStates(new Uint8Array(), 2, 2))).toEqual([0, 0, 0, 0]);
  });
});

describe('the packing, against the Go oracle', () => {
  // The vector is two rows of Pensantus's view of the cave in the dark, as `picture` draws it in
  // backend/internal/maps/fog_test.go (TestMR036_FogViewsMatchTheCaveOracle; `codes` there unpacks the same way: four bits a
  // square, the low half of the byte first): B bright, d dim, g seen in grey, # a wall seen, a space unseen.
  const oracle = ['#gggggg#########   BBBB#', 'gggggggggggggggBBBBBBBd#'];
  const bytes = packStates(statesOf(oracle));

  it('packs 48 squares in 24 bytes, and unpacks them to the picture', () => {
    expect(bytes.length).toBe(24);
    const states = Array.from(unpackStates(bytes, 24, 2));
    expect(states).toEqual(statesOf(oracle));
    expect(states.filter((s) => s === 2).length).toBe(6 + 15);
    expect(states.filter((s) => s === 4).length).toBe(4 + 7);
    expect(states.filter((s) => s === 1).length).toBe(1 + 9 + 1 + 1);
    expect(states.filter((s) => s === 3).length).toBe(1);
    expect(states.filter((s) => s === 0).length).toBe(3);
  });

  it('shades the wall seen as "Visto" and leaves the squares not seen black', () => {
    const vision = decodeVision(visionResponse(oracle));
    expect(seenCount(vision)).toBe(45);
    expect(shadeRuns(vision).filter((r) => r.shade === 'unseen')).toEqual([
      { shade: 'unseen', col: 16, row: 0, len: 3 },
    ]);
  });
});

describe('the shading of each state', () => {
  const rows = ['.dg', 'rBw', '...'];
  const vision = decodeVision(visionResponse(rows));

  it('joins the squares drawn alike in a row and leaves "Visto" (bright light, a wall seen now) alone', () => {
    expect(shadeRuns(vision)).toEqual([
      { shade: 'unseen', col: 0, row: 0, len: 1 },
      { shade: 'dim', col: 1, row: 0, len: 1 },
      { shade: 'grey', col: 2, row: 0, len: 1 },
      { shade: 'remembered', col: 0, row: 1, len: 1 },
      { shade: 'unseen', col: 0, row: 2, len: 3 },
    ]);
  });

  it('counts the squares seen now: bright, dim, grey and the walls next to them, not the remembered or the unseen', () => {
    expect(seenCount(vision)).toBe(4);
  });

  it('names only the states on the map, for the legend', () => {
    expect(visionLegend(vision)).toEqual({
      seen: true,
      dim: true,
      grey: true,
      remembered: true,
      unseen: true,
    });
    expect(visionLegend(decodeVision(visionResponse(['BB', 'dd'])))).toEqual({
      seen: true,
      dim: true,
      grey: false,
      remembered: false,
      unseen: false,
    });
  });

  it('joins runs that sit one on top of the other, equal in column and length, into one block', () => {
    const cave = decodeVision(visionResponse(['....', '.gg.', '.gg.', '.g..', '....']));
    expect(shadeRects(cave).filter((r) => r.shade === 'grey')).toEqual([
      { shade: 'grey', col: 1, row: 1, cols: 2, rows: 2 },
      { shade: 'grey', col: 1, row: 3, cols: 1, rows: 1 },
    ]);
    const total = (shade: string) =>
      shadeRects(cave)
        .filter((r) => r.shade === shade)
        .reduce((n, r) => n + r.cols * r.rows, 0);
    expect(total('grey')).toBe(5);
    expect(total('unseen')).toBe(20 - 5);
  });

  it('shades every square exactly once, whatever the blocks', () => {
    const squares = rows.join('').length;
    const shaded = shadeRects(vision).reduce((n, r) => n + r.cols * r.rows, 0);
    expect(shaded).toBe(
      statesOf(rows).filter((s) => s === 0 || s === 2 || s === 3 || s === 5).length,
    );
    expect(shaded).toBeLessThanOrEqual(squares);
  });
});

describe('the tiles', () => {
  const response = visionResponse(
    Array.from({ length: 20 }, () => 'B'.repeat(24)),
    {
      tilesPath: '/images/maps/m1/tiles/',
      tileSquares: 16,
      tiles: [
        { $typeName: 'meurpg.maps.v1.MapTile', tx: 0, ty: 0, revision: 12 },
        { $typeName: 'meurpg.maps.v1.MapTile', tx: 1, ty: 0, revision: 3 },
        { $typeName: 'meurpg.maps.v1.MapTile', tx: 0, ty: 1, revision: 5 },
      ],
    },
  );
  const vision = decodeVision(response);

  it('places each tile on the grid; the last column and row of tiles are smaller', () => {
    expect(tileRects(vision).map((r) => [r.key, r.col, r.row, r.cols, r.rows])).toEqual([
      ['0:0', 0, 0, 16, 16],
      ['1:0', 16, 0, 8, 16],
      ['0:1', 0, 16, 16, 4],
    ]);
  });

  it('asks for a tile by its revision, so an unchanged tile keeps its URL and a changed one gets a new one', () => {
    expect(tileUrl(vision, { tx: 1, ty: 0, revision: 3 }, null)).toBe(
      '/images/maps/m1/tiles/1/0?r=3',
    );
    expect(tileUrl(vision, { tx: 1, ty: 0, revision: 4 }, null)).toBe(
      '/images/maps/m1/tiles/1/0?r=4',
    );
  });

  it('adds the character the master reads as, and nothing for a player', () => {
    expect(tileUrl(vision, { tx: 0, ty: 0, revision: 12 }, 'char-1')).toBe(
      '/images/maps/m1/tiles/0/0?r=12&as=char-1',
    );
  });

  it('has no tiles for a viewer that reads the whole image (the master, a map without the fog)', () => {
    expect(tileRects(decodeVision(visionResponse(['BB'])))).toEqual([]);
  });

  it('counts the tiles in: "parte N de M"', () => {
    const rects = tileRects(vision);
    expect(tileProgress(rects, new Set())).toEqual({ total: 3, done: 0 });
    expect(tileProgress(rects, new Set(['1:0', '0:1']))).toEqual({ total: 3, done: 2 });
  });
});

describe('knownWindow', () => {
  const rows = ['..........', '..........', '...#BB#...', '...#Br#...', '..........', '..........'];
  const vision = decodeVision(visionResponse(rows));

  it('is the box of what the viewer knows (seen now, remembered, a wall) with the margin, on the grid', () => {
    expect(knownWindow(vision, [], 1)).toEqual({ col: 2, row: 1, cols: 6, rows: 4 });
    // The margin never leaves the grid.
    expect(knownWindow(vision, [], 5)).toEqual({ col: 0, row: 0, cols: 10, rows: 6 });
  });

  it('grows to hold a token outside what the viewer sees', () => {
    expect(knownWindow(vision, [{ col: 9, row: 5 }], 0)).toEqual({
      col: 3,
      row: 2,
      cols: 7,
      rows: 4,
    });
  });

  it('is nothing when nothing is known and there is no token', () => {
    expect(knownWindow(decodeVision(visionResponse(['..', '..'])), [], 2)).toBeNull();
  });
});
