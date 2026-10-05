import { pillSide } from './combat-map';

const cells = (...xs: [number, number][]) => new Set(xs.map(([c, r]) => `${c},${r}`));

describe('where the "Vez" word goes (it must read as its own token\'s)', () => {
  it('goes above when nothing is near', () => {
    expect(pillSide({ col: 5, row: 8 }, 1, cells([5, 8]), 24, 16)).toBe('above');
  });

  it('is not above a token whose neighbour stands diagonally up (the pill is wider than a square): below, then', () => {
    // Sálvia at 5,8 and Toren at 6,7: the word above her would sit beside Toren.
    expect(pillSide({ col: 5, row: 8 }, 1, cells([5, 8], [6, 7]), 24, 16)).toBe('below');
  });

  it('goes to the side when above and below are both taken', () => {
    expect(pillSide({ col: 5, row: 8 }, 1, cells([5, 8], [5, 7], [5, 9]), 24, 16)).toBe('right');
    expect(pillSide({ col: 5, row: 8 }, 1, cells([5, 8], [5, 7], [5, 9], [6, 8]), 24, 16)).toBe('left');
  });

  it('never goes off the map: on the top row it goes below', () => {
    expect(pillSide({ col: 5, row: 0 }, 1, cells([5, 0]), 24, 16)).toBe('below');
  });

  it('with every side taken keeps the side with the fewest neighbours (above on a tie)', () => {
    const around = cells([5, 8], [4, 7], [5, 7], [6, 7], [4, 9], [5, 9], [6, 9], [4, 8], [6, 8]);
    expect(pillSide({ col: 5, row: 8 }, 1, around, 24, 16)).toBe('above');
  });

  it('a big token counts its own squares as its own', () => {
    expect(pillSide({ col: 5, row: 8 }, 2, cells([5, 8], [6, 8], [5, 9], [6, 9]), 24, 16)).toBe('above');
  });
});
