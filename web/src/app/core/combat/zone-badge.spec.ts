import { badgeSpots } from './zone-badge';

const sq = (col: number, row: number) => ({ col, row });
const COLS = 24;
const ROWS = 16;

function block(col: number, row: number, w: number, h: number) {
  const out = [];
  for (let c = col; c < col + w; c++) {
    for (let r = row; r < row + h; r++) {
      out.push(sq(c, r));
    }
  }
  return out;
}

describe('the corner badge of a zone', () => {
  it('sits at the top left corner of the footprint', () => {
    expect(badgeSpots([block(4, 3, 2, 2)], COLS, ROWS)).toEqual([{ col: 4, row: 3 }]);
  });

  it('stays inside the map for a zone at every corner of it', () => {
    const zones = [
      block(0, 0, 3, 3),
      block(COLS - 3, 0, 3, 3),
      block(0, ROWS - 3, 3, 3),
      block(COLS - 3, ROWS - 3, 3, 3),
      [sq(0, 0)],
      [sq(COLS - 1, ROWS - 1)],
    ];
    for (const z of zones) {
      const [spot] = badgeSpots([z], COLS, ROWS);
      expect(spot.col).toBeGreaterThanOrEqual(0);
      expect(spot.col).toBeLessThanOrEqual(COLS);
      expect(spot.row).toBeGreaterThanOrEqual(0);
      expect(spot.row).toBeLessThanOrEqual(ROWS);
    }
  });

  it('moves a later zone to the opposite corner when two share the first one', () => {
    const spots = badgeSpots([block(4, 3, 3, 3), block(4, 3, 3, 3)], COLS, ROWS);
    expect(spots).toEqual([
      { col: 4, row: 3 },
      { col: 7, row: 6 },
    ]);
  });

  it('keeps a single-square zone readable: the second badge takes another corner', () => {
    const spots = badgeSpots([[sq(0, 0)], [sq(0, 0)]], COLS, ROWS);
    expect(spots[0]).toEqual({ col: 0, row: 0 });
    expect(spots[1]).toEqual({ col: 1, row: 1 });
  });
});
