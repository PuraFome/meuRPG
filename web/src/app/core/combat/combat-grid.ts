/**
 * The combat grid's arithmetic, kept apart from the components so it is
 * tested without a DOM (RN-21): squares of 1,5 m (5 ft) across and down,
 * counted from 0 at the top left. A move is a straight line between the
 * centres of two squares and costs its exact length (a diagonal step is 7,1 ft),
 * so what a combatant reaches is a circle, the same one the server draws.
 * TEMPORARY: it knows nothing of walls, rubble or other creatures' costs, which
 * only the server does (slice 9.15 replaces the reach with `GetMoveOptions`).
 * The distances in words are `core/units.ts`'s.
 */

import { distanceText } from '../units';

/** The screen takes 5 to 60 columns; the server takes 4 to 200. */
export const MIN_COLUMNS = 5;
export const MAX_COLUMNS = 60;

export interface Square {
  readonly col: number;
  readonly row: number;
}

/** The rows a grid of `columns` has on an image: the server's own rounding
 * (maps.proto: round(columns * height / width)), at least 1. */
export function gridRows(columns: number, imageWidth: number, imageHeight: number): number {
  return Math.max(1, Math.round((columns * imageHeight) / Math.max(1, imageWidth)));
}

/** The cost of one square straight, in tenths of a foot: what the server charges. */
export const DFT_PER_SQUARE = 50;

/** How many squares apart: the straight line between the centres, so a diagonal
 * step is 1,41 squares (7,1 ft), not one. */
export function distance(a: Square, b: Square): number {
  return Math.hypot(a.col - b.col, a.row - b.row);
}

/** The same distance in tenths of a foot, rounded to the nearest like the
 * server's (a square straight is 50, a diagonal one 71). */
export function lengthDft(a: Square, b: Square): number {
  return Math.round(distance(a, b) * DFT_PER_SQUARE);
}

/** Whether `s` is a square of a grid of `columns` x `rows`. */
export function inGrid(s: Square, columns: number, rows: number): boolean {
  return s.col >= 0 && s.row >= 0 && s.col < columns && s.row < rows;
}

/** Where the center of a square is, as a percentage of the grid's width and
 * height: the position of a token or a frame. */
export function squareCenter(s: Square, columns: number, rows: number): { x: number; y: number } {
  return { x: ((s.col + 0.5) / columns) * 100, y: ((s.row + 0.5) / rows) * 100 };
}

/** The square under a point of the map: `x` and `y` are fractions of the
 * map's width and height (0 to 1). Clamped to the grid. */
export function squareAt(x: number, y: number, columns: number, rows: number): Square {
  return {
    col: Math.min(columns - 1, Math.max(0, Math.floor(x * columns))),
    row: Math.min(rows - 1, Math.max(0, Math.floor(y * rows))),
  };
}

/** One arrow key's step, or `null` for any other key. */
export function arrowStep(key: string): { dc: number; dr: number } | null {
  switch (key) {
    case 'ArrowLeft':
      return { dc: -1, dr: 0 };
    case 'ArrowRight':
      return { dc: 1, dr: 0 };
    case 'ArrowUp':
      return { dc: 0, dr: -1 };
    case 'ArrowDown':
      return { dc: 0, dr: 1 };
    default:
      return null;
  }
}

/** The square one arrow key away, kept inside the grid. */
export function stepSquare(from: Square, key: string, columns: number, rows: number): Square {
  const step = arrowStep(key);
  if (!step) {
    return from;
  }
  return {
    col: Math.min(columns - 1, Math.max(0, from.col + step.dc)),
    row: Math.min(rows - 1, Math.max(0, from.row + step.dr)),
  };
}

/** Whether `to` is within the circle `leftDft` (the movement left, in tenths of
 * a foot) draws around `from`, a different square, inside the grid and not one
 * of the `occupied` ones. */
export function canReach(
  from: Square,
  to: Square,
  leftDft: number,
  columns: number,
  rows: number,
  occupied: readonly Square[],
): boolean {
  return (
    inGrid(to, columns, rows) &&
    lengthDft(from, to) > 0 &&
    lengthDft(from, to) <= leftDft &&
    !occupied.some((o) => o.col === to.col && o.row === to.row)
  );
}

function squares(n: number): string {
  return `${n} ${n === 1 ? 'quadrado' : 'quadrados'}`;
}

/** How a move goes, without its cost: "2 quadrados para a direita e 1 para
 * baixo. Depois restam 4,5 m." `leftFt` is the movement left before it. */
export function moveDetail(from: Square, to: Square, leftFt: number): string {
  const dc = to.col - from.col;
  const dr = to.row - from.row;
  const parts: string[] = [];
  if (dc !== 0) {
    parts.push(`${squares(Math.abs(dc))} para ${dc > 0 ? 'a direita' : 'a esquerda'}`);
  }
  if (dr !== 0) {
    parts.push(`${squares(Math.abs(dr))} para ${dr > 0 ? 'baixo' : 'cima'}`);
  }
  const after = Math.max(0, leftFt - lengthDft(from, to) / 10);
  return `${parts.join(' e ')}. Depois restam ${distanceText(after)}.`;
}

/** What the live line under the map says once a square is chosen (E6-10):
 * "Mover 3 m. 2 quadrados para a direita e 1 para baixo. Depois restam 4,5 m." */
export function moveSentence(from: Square, to: Square, leftFt: number): string {
  return `Mover ${distanceText(lengthDft(from, to) / 10)}. ${moveDetail(from, to, leftFt)}`;
}

/** The ordinal circle: "2º círculo", with a no-break space so a line never
 * ends on "1º" with "círculo" alone on the next. */
export function circleLabel(level: number): string {
  return level === 0 ? 'Truque' : `${level}º\u00a0círculo`;
}
