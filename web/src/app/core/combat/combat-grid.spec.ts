import { arrowStep, gridRows, squareAt, squareCenter, stepSquare } from './combat-grid';

describe('combat grid maths', () => {
  it('works the rows out from the image ratio, like the server', () => {
    expect(gridRows(20, 2000, 1400)).toBe(14);
    expect(gridRows(20, 1000, 1000)).toBe(20);
    expect(gridRows(5, 3000, 1000)).toBe(2); // 1,67 rounds to 2
    expect(gridRows(5, 10000, 100)).toBe(1); // never fewer than one row
  });

  it('puts the center of a square and finds the square under a point', () => {
    expect(squareCenter({ col: 0, row: 0 }, 20, 10)).toEqual({ x: 2.5, y: 5 });
    expect(squareAt(0.26, 0.51, 20, 10)).toEqual({ col: 5, row: 5 });
    expect(squareAt(1, 1, 20, 10)).toEqual({ col: 19, row: 9 });
    expect(squareAt(-1, -1, 20, 10)).toEqual({ col: 0, row: 0 });
  });

  it('moves the choice one square with the arrows, inside the grid', () => {
    expect(stepSquare({ col: 0, row: 0 }, 'ArrowLeft', 20, 14)).toEqual({ col: 0, row: 0 });
    expect(stepSquare({ col: 3, row: 3 }, 'ArrowDown', 20, 14)).toEqual({ col: 3, row: 4 });
    expect(stepSquare({ col: 3, row: 3 }, 'x', 20, 14)).toEqual({ col: 3, row: 3 });
    expect(arrowStep('ArrowUp')).toEqual({ dc: 0, dr: -1 });
    expect(arrowStep('Enter')).toBeNull();
  });
});
