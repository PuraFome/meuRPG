import {
  arrowStep,
  canReach,
  distance,
  gridRows,
  inGrid,
  lengthDft,
  moveSentence,
  squareAt,
  type Square,
  squareCenter,
  stepSquare,
} from './combat-grid';

describe('combat grid maths', () => {
  it('works the rows out from the image ratio, like the server', () => {
    expect(gridRows(20, 2000, 1400)).toBe(14);
    expect(gridRows(20, 1000, 1000)).toBe(20);
    expect(gridRows(5, 3000, 1000)).toBe(2); // 1,67 rounds to 2
    expect(gridRows(5, 10000, 100)).toBe(1); // never fewer than one row
  });

  it('measures the straight line between centres: a diagonal step is 7,1 ft, not 5 (RN-21)', () => {
    expect(distance({ col: 0, row: 0 }, { col: 3, row: 0 })).toBe(3);
    expect(distance({ col: 4, row: 8 }, { col: 6, row: 9 })).toBeCloseTo(2.236, 3);
    expect(lengthDft({ col: 0, row: 0 }, { col: 1, row: 0 })).toBe(50); // a square straight
    expect(lengthDft({ col: 0, row: 0 }, { col: 1, row: 1 })).toBe(71); // one diagonal step
    expect(lengthDft({ col: 0, row: 0 }, { col: 4, row: 4 })).toBe(283);
  });

  it('reaches a free square inside the circle and the grid, never an occupied one', () => {
    const from = { col: 5, row: 7 };
    const taken = [{ col: 6, row: 7 }];
    // 25 ft left = 250 dft.
    expect(canReach(from, { col: 7, row: 8 }, 250, 20, 14, taken)).toBe(true);
    expect(canReach(from, { col: 6, row: 7 }, 250, 20, 14, taken)).toBe(false);
    expect(canReach(from, { col: 11, row: 7 }, 250, 20, 14, taken)).toBe(false);
    expect(canReach(from, from, 250, 20, 14, taken)).toBe(false);
    expect(canReach({ col: 0, row: 0 }, { col: -1, row: 0 }, 250, 20, 14, [])).toBe(false);
  });

  it('draws a circle: with 30 ft (4, 4) is in reach and (5, 5) is not, as on the server', () => {
    const from = { col: 5, row: 5 };
    expect(canReach(from, { col: 9, row: 9 }, 300, 20, 14, [])).toBe(true); // 28,3 ft
    expect(canReach(from, { col: 10, row: 10 }, 300, 20, 14, [])).toBe(false); // 35,4 ft
    expect(canReach(from, { col: 11, row: 5 }, 300, 20, 14, [])).toBe(true); // 6 squares straight
    expect(canReach(from, { col: 5, row: 12 }, 300, 20, 14, [])).toBe(false); // 7 squares
  });

  it('puts the center of a square and finds the square under a point', () => {
    expect(squareCenter({ col: 0, row: 0 }, 20, 10)).toEqual({ x: 2.5, y: 5 });
    expect(squareAt(0.26, 0.51, 20, 10)).toEqual({ col: 5, row: 5 });
    expect(squareAt(1, 1, 20, 10)).toEqual({ col: 19, row: 9 });
    expect(squareAt(-1, -1, 20, 10)).toEqual({ col: 0, row: 0 });
    expect(inGrid({ col: 19, row: 9 }, 20, 10)).toBe(true);
    expect(inGrid({ col: 20, row: 9 }, 20, 10)).toBe(false);
  });

  it('moves the choice one square with the arrows, inside the grid', () => {
    expect(stepSquare({ col: 0, row: 0 }, 'ArrowLeft', 20, 14)).toEqual({ col: 0, row: 0 });
    expect(stepSquare({ col: 3, row: 3 }, 'ArrowDown', 20, 14)).toEqual({ col: 3, row: 4 });
    expect(stepSquare({ col: 3, row: 3 }, 'x', 20, 14)).toEqual({ col: 3, row: 3 });
    expect(arrowStep('ArrowUp')).toEqual({ dc: 0, dr: -1 });
    expect(arrowStep('Enter')).toBeNull();
  });

  it('says how far a move is, in meters and squares (E6-10, E8-01)', () => {
    // The text ties numbers to their units with no-break spaces: read it with plain ones.
    const say = (from: Square, to: Square, left: number) => moveSentence(from, to, left).replace(/\u00a0/g, ' ');
    // 2 squares across and 1 down is 11,2 ft: 3,4 m, no longer the 10 ft of a king's move.
    expect(say({ col: 4, row: 8 }, { col: 6, row: 9 }, 25)).toBe(
      'Mover 3,4 m. 2 quadrados para a direita e 1 quadrado para baixo. Depois restam 4,1 m.',
    );
    expect(say({ col: 4, row: 8 }, { col: 4, row: 7 }, 25)).toBe(
      'Mover 1,5 m. 1 quadrado para cima. Depois restam 6 m.',
    );
    expect(say({ col: 4, row: 8 }, { col: 1, row: 8 }, 10)).toBe(
      'Mover 4,5 m. 3 quadrados para a esquerda. Depois restam 0 m.',
    );
  });
});
