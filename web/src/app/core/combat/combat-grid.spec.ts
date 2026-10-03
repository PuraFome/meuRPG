import {
  arrowStep,
  canReach,
  distance,
  feetToMeters,
  formatMeters,
  gridRows,
  inGrid,
  moveSentence,
  reachSquares,
  squareAt,
  squareCenter,
  stepSquare,
  tight,
} from './combat-grid';

describe('combat grid maths', () => {
  it('works the rows out from the image ratio, like the server', () => {
    expect(gridRows(20, 2000, 1400)).toBe(14);
    expect(gridRows(20, 1000, 1000)).toBe(20);
    expect(gridRows(5, 3000, 1000)).toBe(2); // 1,67 rounds to 2
    expect(gridRows(5, 10000, 100)).toBe(1); // never fewer than one row
  });

  it('says meters with a decimal comma and no useless zero', () => {
    expect(formatMeters(1.5)).toBe('1,5 m');
    expect(formatMeters(30)).toBe('30 m');
    expect(formatMeters(feetToMeters(25))).toBe('7,5 m');
    expect(formatMeters(feetToMeters(5))).toBe('1,5 m');
  });

  it('counts a diagonal as one square (a king\'s move)', () => {
    expect(distance({ col: 4, row: 8 }, { col: 6, row: 9 })).toBe(2);
    expect(distance({ col: 0, row: 0 }, { col: 3, row: 3 })).toBe(3);
  });

  it('turns the movement left into whole squares', () => {
    expect(reachSquares(25)).toBe(5);
    expect(reachSquares(22)).toBe(4);
    expect(reachSquares(0)).toBe(0);
  });

  it('reaches a free square inside the range and the grid, never an occupied one', () => {
    const from = { col: 5, row: 7 };
    const taken = [{ col: 6, row: 7 }];
    expect(canReach(from, { col: 7, row: 8 }, 5, 20, 14, taken)).toBe(true);
    expect(canReach(from, { col: 6, row: 7 }, 5, 20, 14, taken)).toBe(false);
    expect(canReach(from, { col: 11, row: 7 }, 5, 20, 14, taken)).toBe(false);
    expect(canReach(from, from, 5, 20, 14, taken)).toBe(false);
    expect(canReach({ col: 0, row: 0 }, { col: -1, row: 0 }, 5, 20, 14, [])).toBe(false);
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

  it('says how far a move is, as the artboard does (E6-10)', () => {
    expect(moveSentence({ col: 4, row: 8 }, { col: 6, row: 9 }, 25)).toBe(
      'Mover 3 m. 2 quadrados para a direita e 1 quadrado para baixo. Depois restam 4,5 m.',
    );
    expect(moveSentence({ col: 4, row: 8 }, { col: 4, row: 7 }, 25)).toBe(
      'Mover 1,5 m. 1 quadrado para cima. Depois restam 6 m.',
    );
    expect(moveSentence({ col: 4, row: 8 }, { col: 1, row: 8 }, 10)).toBe(
      'Mover 4,5 m. 3 quadrados para a esquerda. Depois restam 0 m.',
    );
  });
});

describe('tight', () => {
  it('keeps a number, its unit and the word before it together', () => {
    expect(tight('+5 para acertar · alcance 6 m')).toBe('+5 para acertar · alcance\u00a06\u00a0m');
    expect(tight('Dá para andar mais 4,5 m (3 quadrados)')).toBe('Dá para andar mais\u00a04,5\u00a0m (3 quadrados)');
    expect(tight('Digite um número de 1 a 20')).toBe('Digite um número de\u00a01\u00a0a\u00a020');
    expect(tight('Restam 7,5 m de 15 m')).toBe('Restam\u00a07,5\u00a0m de\u00a015\u00a0m');
  });
});
