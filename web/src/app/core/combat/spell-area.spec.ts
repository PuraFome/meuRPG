import { SpellAreaShape } from '../../../gen/meurpg/rules/v1/rules_pb';
import type { Square } from './combat-grid';
import {
  type AreaShape,
  areaSquares,
  beyondRange,
  directionWord,
  fromCaster,
  outlinePath,
  rangeFt,
  rotate,
  sphereSquares,
  toward,
} from './spell-area';

const sorted = (squares: readonly Square[]) =>
  squares
    .map((s) => `${s.col},${s.row}`)
    .sort()
    .join(' ');
const list = (pairs: [number, number][]) => sorted(pairs.map(([col, row]) => ({ col, row })));

const cone15: AreaShape = { shape: SpellAreaShape.CONE, sizeFt: 15, widthFt: 0 };
const line100: AreaShape = { shape: SpellAreaShape.LINE, sizeFt: 100, widthFt: 5 };
const cube15: AreaShape = { shape: SpellAreaShape.CUBE, sizeFt: 15, widthFt: 0 };
const caster = { col: 4, row: 4 };

describe('spell area outline', () => {
  it('draws a 15 ft cone east of the caster as the seven squares the server takes', () => {
    const squares = areaSquares(cone15, { caster, direction: { dx: 1, dy: 0 } }, 9, 9);
    expect(sorted(squares)).toBe(
      list([
        [5, 4],
        [6, 3],
        [6, 4],
        [6, 5],
        [7, 3],
        [7, 4],
        [7, 5],
      ]),
    );
  });

  it('draws a 15 ft cone to the south-east as six squares', () => {
    const squares = areaSquares(cone15, { caster, direction: { dx: 1, dy: 1 } }, 9, 9);
    expect(sorted(squares)).toBe(
      list([
        [5, 5],
        [6, 5],
        [7, 5],
        [5, 6],
        [6, 6],
        [5, 7],
      ]),
    );
  });

  it("draws a 100 ft line east as twenty squares in the caster's row", () => {
    const squares = fromCaster(caster, { dx: 1, dy: 0 }, line100);
    expect(squares).toHaveLength(20);
    expect(squares.every((s) => s.row === caster.row && s.col > caster.col)).toBe(true);
    expect(Math.max(...squares.map((s) => s.col))).toBe(caster.col + 20);
  });

  it('cuts the line at the edge of the grid', () => {
    expect(areaSquares(line100, { caster, direction: { dx: 1, dy: 0 } }, 9, 9)).toHaveLength(4);
  });

  it('touches the caster by a face with a 15 ft cube east: columns +1 to +3, rows -1 to +1', () => {
    const squares = fromCaster(caster, { dx: 1, dy: 0 }, cube15);
    const cols = [...new Set(squares.map((s) => s.col - caster.col))].sort();
    const rows = [...new Set(squares.map((s) => s.row - caster.row))].sort();
    expect(squares).toHaveLength(9);
    expect(cols).toEqual([1, 2, 3]);
    expect(rows).toEqual([-1, 0, 1]);
  });

  it('touches the caster by a corner with a diagonal cube', () => {
    const squares = fromCaster(caster, { dx: -1, dy: -1 }, cube15);
    expect(sorted(squares.map((s) => ({ col: s.col - 4, row: s.row - 4 })))).toBe(
      list([
        [-3, -3],
        [-2, -3],
        [-1, -3],
        [-3, -2],
        [-2, -2],
        [-1, -2],
        [-3, -1],
        [-2, -1],
        [-1, -1],
      ]),
    );
  });

  it('holds 49 squares in a 20 ft sphere and 5 in a 5 ft one, the origin included', () => {
    expect(sphereSquares({ col: 10, row: 10 }, 20)).toHaveLength(49);
    const small = sphereSquares({ col: 10, row: 10 }, 5);
    expect(small).toHaveLength(5);
    expect(small).toContainEqual({ col: 10, row: 10 });
  });

  it("never includes the caster's square in a cone, a line or a cube, in any of the eight directions", () => {
    for (const shape of [cone15, line100, cube15]) {
      for (let i = 0; i < 8; i++) {
        const d = rotate({ dx: 1, dy: 0 }, i);
        const squares = fromCaster(caster, d, shape);
        expect(squares.length).toBeGreaterThan(0);
        expect(squares).not.toContainEqual(caster);
      }
    }
  });

  it('gives no squares for a direction that is not one of the eight', () => {
    expect(fromCaster(caster, { dx: 0, dy: 0 }, cone15)).toEqual([]);
    expect(fromCaster(caster, { dx: 2, dy: 0 }, cone15)).toEqual([]);
  });
});

describe('the direction toward a square (the server rule)', () => {
  it('keeps the axis up to 5/12 of a turn off it, and the diagonal beyond', () => {
    expect(toward(caster, { col: 16, row: 9 })).toEqual({ dx: 1, dy: 0 }); // 12 across, 5 down
    expect(toward(caster, { col: 16, row: 10 })).toEqual({ dx: 1, dy: 1 }); // 12 across, 6 down
    expect(toward(caster, { col: 9, row: 16 })).toEqual({ dx: 0, dy: 1 });
    expect(toward(caster, { col: 0, row: 0 })).toEqual({ dx: -1, dy: -1 });
    expect(toward(caster, caster)).toBeNull();
  });

  it('turns 45 degrees a step, both ways, and names the direction', () => {
    expect(rotate({ dx: 1, dy: 0 }, 1)).toEqual({ dx: 1, dy: 1 });
    expect(rotate({ dx: 1, dy: 0 }, -1)).toEqual({ dx: 1, dy: -1 });
    expect(rotate({ dx: 0, dy: -1 }, 2)).toEqual({ dx: 1, dy: 0 });
    expect(directionWord({ dx: 1, dy: -1 })).toBe('nordeste');
  });
});

describe('range and outline', () => {
  it('measures a range as the squares between centers, rounded down (RN-21)', () => {
    expect(rangeFt({ col: 0, row: 0 }, { col: 5, row: 0 })).toBe(25);
    expect(rangeFt({ col: 0, row: 0 }, { col: 4, row: 4 })).toBe(25);
    expect(rangeFt({ col: 0, row: 0 }, { col: 1, row: 1 })).toBe(5);
  });

  it('lists the squares beyond the range', () => {
    const out = beyondRange({ col: 0, row: 0 }, 5, 3, 3);
    expect(sorted(out)).toBe(
      list([
        [2, 0],
        [2, 1],
        [0, 2],
        [1, 2],
        [2, 2],
      ]),
    );
  });

  it('outlines a single square with its four sides and two squares without the side they share', () => {
    expect(outlinePath([{ col: 1, row: 1 }])).toBe('M1 1h1M1 2h1M1 1v1M2 1v1');
    expect(
      outlinePath([
        { col: 0, row: 0 },
        { col: 1, row: 0 },
      ]).split('M'),
    ).toHaveLength(7);
  });
});
