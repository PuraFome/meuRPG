import type { MapLayers } from './layers';
import { tokenSpot } from './token-spot';

// A grid of 5 x 5 squares: square (c, r) has its center at ((c + 0.5) * 2000, (r + 0.5) * 2000) basis points.
const centerOf = (col: number, row: number) => ({
  xBp: (col + 0.5) * 2000,
  yBp: (row + 0.5) * 2000,
});

function layers(extra: Partial<MapLayers> = {}): MapLayers {
  return { columns: 5, rows: 5, walls: [], terrain: [], half: [], threeQuarters: [], ...extra };
}

describe('tokenSpot', () => {
  it('takes the middle square of an open map', () => {
    expect(tokenSpot(centerOf(2, 2), layers(), [])).toEqual(centerOf(2, 2));
  });

  it('never takes a wall, a column or a shut door, but an open door is floor', () => {
    const spot = tokenSpot(
      centerOf(2, 2),
      layers({
        walls: [{ col: 2, row: 2 }],
        threeQuarters: [{ col: 2, row: 1 }],
        doors: [
          { col: 1, row: 2, state: 2, axis: 'v' },
          { col: 3, row: 2, state: 4, axis: 'v' },
          { col: 2, row: 3, state: 1, axis: 'h' },
        ],
      }),
      [],
    );
    expect(spot).toEqual(centerOf(2, 3));
  });

  it('puts each new token on its own square, nearest the middle first', () => {
    const tokens: { xBp: number; yBp: number }[] = [];
    const placed: { xBp: number; yBp: number }[] = [];
    for (let n = 0; n < 4; n++) {
      const spot = tokenSpot(centerOf(2, 2), layers(), tokens);
      placed.push(spot);
      tokens.push(spot);
    }
    expect(placed[0]).toEqual(centerOf(2, 2));
    expect(new Set(placed.map((p) => `${p.xBp},${p.yBp}`)).size).toBe(4);
    // The four neighbours are all one square from the middle.
    for (const p of placed.slice(1)) {
      const dc = Math.round(p.xBp / 2000 - 0.5) - 2;
      const dr = Math.round(p.yBp / 2000 - 0.5) - 2;
      expect(Math.abs(dc) + Math.abs(dr)).toBe(1);
    }
  });

  it('keeps the middle of the screen when the map has no grid yet or no square is left', () => {
    const middle = { xBp: 1234, yBp: 4321 };
    expect(tokenSpot(middle, layers({ columns: 0, rows: 0 }), [])).toEqual(middle);
    const all = Array.from({ length: 25 }, (_v, n) => ({ col: n % 5, row: Math.floor(n / 5) }));
    expect(tokenSpot(middle, layers({ walls: all }), [])).toEqual(middle);
  });
});
