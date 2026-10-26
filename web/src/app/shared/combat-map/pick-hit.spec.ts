import { type HitBox, creatureAt, tokenSize } from './pick-hit';

const sword = (id: string, col: number, row: number): HitBox => ({ id, col, row, span: 1 });
const armor: HitBox = { id: 'armor', col: 21, row: 13, span: 2 };

describe('creatureAt: the creature whose square is under the pointer', () => {
  const unit = 9;
  // Squares are 9 px wide: sword 2 sits on square 21,12 (x 189-198, y 108-117), the armor over 21-22 x 13-14.
  const boxes = [sword('s1', 20, 12), sword('s2', 21, 12), sword('s3', 20, 13), armor];

  it('picks the sword whose square holds the point, not the token drawn over its neighbour', () => {
    expect(creatureAt(193, 112, unit, boxes)).toEqual({ id: 's2', exact: true });
    expect(creatureAt(184, 113, unit, boxes)).toEqual({ id: 's1', exact: true });
    expect(creatureAt(184, 122, unit, boxes)).toEqual({ id: 's3', exact: true });
  });

  it('picks a big creature from any of the squares it covers', () => {
    expect(creatureAt(21 * unit + 1, 13 * unit + 1, unit, boxes)).toEqual({
      id: 'armor',
      exact: true,
    });
    expect(creatureAt(22 * unit + 8, 14 * unit + 8, unit, boxes)).toEqual({
      id: 'armor',
      exact: true,
    });
  });

  it('falls back to the nearest creature within reach of an empty square, never farther', () => {
    // Square 19,12 is empty; sword 1 is the nearest, 5 px from its centre.
    expect(creatureAt(19 * unit + 4, 12 * unit + 4, unit, boxes, 22)).toEqual({
      id: 's1',
      exact: false,
    });
    expect(creatureAt(5 * unit, 5 * unit, unit, boxes, 22)).toBeNull();
  });

  it('knows nothing of a map that is not measured', () => {
    expect(creatureAt(10, 10, 0, boxes)).toBeNull();
  });
});

describe('tokenSize: a token is never bigger than its square', () => {
  it('is 86% of the square, at most 40 px, times the squares it spans', () => {
    expect(tokenSize(9, 1)).toBeCloseTo(7.74);
    expect(tokenSize(9, 2)).toBeCloseTo(15.48);
    expect(tokenSize(60, 1)).toBe(40);
  });

  it('never exceeds the room of its squares, for any square', () => {
    for (let unit = 1; unit <= 120; unit++) {
      for (const span of [1, 2, 3, 4]) {
        expect(tokenSize(unit, span)).toBeLessThanOrEqual(unit * span);
      }
    }
  });
});
