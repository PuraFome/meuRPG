import { describe, expect, it } from 'vitest';

import {
  areaLine,
  clampSquare,
  edgeMarkOf,
  listMove,
  movedPoint,
  turnedDirection,
} from './area-moves';

describe('area moves', () => {
  it('keeps a square on the map', () => {
    expect(clampSquare({ col: -2, row: 99 }, 12, 8)).toEqual({ col: 0, row: 7 });
  });

  it('moves a point from the hover, else from the caster, else from the corner, by n squares', () => {
    const from = { kind: 'point', square: { col: 4, row: 4 } } as const;
    expect(movedPoint(from, null, { dc: 1, dr: 0 }, 5, 12, 8)).toEqual({
      kind: 'point',
      square: { col: 9, row: 4 },
    });
    expect(movedPoint(null, { col: 2, row: 2 }, { dc: 0, dr: -1 }, 1, 12, 8).kind).toBe('point');
    expect(movedPoint(null, { col: 2, row: 2 }, { dc: 0, dr: -1 }, 1, 12, 8)).toMatchObject({
      square: { col: 2, row: 1 },
    });
    expect(movedPoint(null, null, { dc: -1, dr: 0 }, 1, 12, 8)).toMatchObject({
      square: { col: 0, row: 0 },
    });
  });

  it('turns a direction, and starts east', () => {
    expect(turnedDirection(null, 1)).toEqual({ kind: 'direction', direction: { dx: 1, dy: 0 } });
    const turned = turnedDirection({ kind: 'direction', direction: { dx: 1, dy: 0 } }, 1);
    expect(turned.kind).toBe('direction');
    expect(turned).not.toEqual({ kind: 'direction', direction: { dx: 1, dy: 0 } });
  });

  it('draws nothing without a caster or an origin', () => {
    expect(areaLine(null, { col: 1, row: 1 }, 12, 8)).toBeNull();
    expect(areaLine({ col: 1, row: 1 }, null, 12, 8)).toBeNull();
    expect(areaLine({ col: 0, row: 0 }, { col: 5, row: 0 }, 12, 8)?.label).toBeTruthy();
    expect(edgeMarkOf(null, { col: 1, row: 1 }, 12, 8)).toBeNull();
    expect(
      edgeMarkOf({ kind: 'direction', direction: { dx: 1, dy: 0 } }, { col: 0, row: 0 }, 10, 10),
    ).toEqual({ x: 10, y: 5 });
  });

  it('moves the list cursor round, and names the keys that choose and close', () => {
    expect(listMove('ArrowDown', 2, 3)).toBe(0);
    expect(listMove('ArrowUp', 0, 3)).toBe(2);
    expect(listMove('Home', 2, 3)).toBe(0);
    expect(listMove('End', 0, 3)).toBe(2);
    expect(listMove(' ', 0, 3)).toBe('choose');
    expect(listMove('Escape', 0, 3)).toBe('close');
    expect(listMove('x', 0, 3)).toBeNull();
  });
});
