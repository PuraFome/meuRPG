import { pickWindow, turnWindow } from './pick-window';

describe('pickWindow', () => {
  it('has no window without a token', () => {
    expect(pickWindow([], 24, 16)).toBeNull();
  });

  it('holds the tokens with a margin, about as tall as it is wide, and inside the grid', () => {
    const w = pickWindow(
      [
        { col: 14, row: 4 },
        { col: 17, row: 6 },
        { col: 12, row: 8 },
      ],
      24,
      16,
    )!;
    expect(w.col).toBeLessThanOrEqual(11);
    expect(w.col + w.cols).toBeGreaterThanOrEqual(19);
    expect(w.row).toBeLessThanOrEqual(3);
    expect(w.row + w.rows).toBeGreaterThanOrEqual(10);
    expect(w.cols / w.rows).toBeLessThan(1.3);
    expect(w.col).toBeGreaterThanOrEqual(0);
    expect(w.col + w.cols).toBeLessThanOrEqual(24);
    expect(w.row + w.rows).toBeLessThanOrEqual(16);
  });

  it('shows the whole map, with its gutter, when a creature stands on the map edge', () => {
    expect(
      pickWindow(
        [
          { col: 0, row: 7 },
          { col: 12, row: 7 },
        ],
        40,
        30,
      ),
    ).toBeNull();
    expect(pickWindow([{ col: 10, row: 29 }], 40, 30)).toBeNull();
  });

  it('shows the whole map when the creatures are spread over it', () => {
    expect(
      pickWindow(
        [
          { col: 0, row: 0 },
          { col: 23, row: 15 },
        ],
        24,
        16,
      ),
    ).toBeNull();
  });
});

describe('turnWindow (C4-03)', () => {
  const own = { col: 12, row: 10 };

  it('is about twice as wide as it is tall, inside the grid, around the player and the creatures near', () => {
    const w = turnWindow(
      own,
      [
        { col: 15, row: 9 },
        { col: 14, row: 12 },
      ],
      40,
      30,
    );
    expect(w.cols / w.rows).toBeGreaterThan(2.4);
    expect(w.cols / w.rows).toBeLessThan(3.3);
    expect(w.col).toBeGreaterThanOrEqual(0);
    expect(w.col + w.cols).toBeLessThanOrEqual(40);
    expect(w.row + w.rows).toBeLessThanOrEqual(30);
    expect(w.col).toBeLessThanOrEqual(12);
    expect(w.col + w.cols).toBeGreaterThan(15);
    expect(w.row).toBeLessThanOrEqual(9);
    expect(w.row + w.rows).toBeGreaterThan(12);
  });

  it('is 125px tall at 354px wide: its rows are the share of its columns', () => {
    const w = turnWindow(own, [], 40, 30);
    expect((354 * w.rows) / w.cols).toBeGreaterThan(115);
    expect((354 * w.rows) / w.cols).toBeLessThan(135);
  });

  it('keeps the player in view when the creatures are far away, and shows the whole of a small map', () => {
    const w = turnWindow(own, [{ col: 39, row: 29 }], 40, 30);
    expect(own.col).toBeGreaterThanOrEqual(w.col);
    expect(own.col).toBeLessThan(w.col + w.cols);
    expect(own.row).toBeGreaterThanOrEqual(w.row);
    expect(own.row).toBeLessThan(w.row + w.rows);
    const small = turnWindow({ col: 3, row: 2 }, [], 8, 6);
    expect(small).toEqual({ col: 0, row: 1, cols: 8, rows: 3 });
  });
});
