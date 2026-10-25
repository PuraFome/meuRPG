import { pickWindow } from './pick-window';

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
