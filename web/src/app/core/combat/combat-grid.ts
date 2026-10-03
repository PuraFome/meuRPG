/**
 * The combat grid's arithmetic, kept apart from the components so it is
 * tested without a DOM (RN-21): squares of 1,5 m (5 ft) across and down,
 * counted from 0 at the top left, and a king's move (a diagonal costs one
 * square too).
 */

/** One square is 5 ft, which is 1,5 m. */
export const SQUARE_FT = 5;
export const SQUARE_M = 1.5;
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

/** Meters as the table says them: "1,5 m", "30 m". */
export function formatMeters(meters: number): string {
  const rounded = Math.round(meters * 10) / 10;
  return `${String(rounded).replace('.', ',')} m`;
}

export function feetToMeters(feet: number): number {
  return (feet / SQUARE_FT) * SQUARE_M;
}

export function squaresToMeters(squares: number): number {
  return squares * SQUARE_M;
}

/** How many squares apart: Chebyshev, so a diagonal counts as one. */
export function distance(a: Square, b: Square): number {
  return Math.max(Math.abs(a.col - b.col), Math.abs(a.row - b.row));
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

/** How many squares the movement left reaches (feet to whole squares). */
export function reachSquares(movementLeftFt: number): number {
  return Math.max(0, Math.floor(movementLeftFt / SQUARE_FT));
}

/** Whether `to` is within `reach` squares of `from`, a different square,
 * inside the grid and not one of the `occupied` ones. */
export function canReach(
  from: Square,
  to: Square,
  reach: number,
  columns: number,
  rows: number,
  occupied: readonly Square[],
): boolean {
  return (
    inGrid(to, columns, rows) &&
    distance(from, to) > 0 &&
    distance(from, to) <= reach &&
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
  const after = Math.max(0, leftFt - distance(from, to) * SQUARE_FT);
  return `${parts.join(' e ')}. Depois restam ${formatMeters(feetToMeters(after))}.`;
}

/** What the live line under the map says once a square is chosen (E6-10):
 * "Mover 3 m. 2 quadrados para a direita e 1 para baixo. Depois restam 4,5 m." */
export function moveSentence(from: Square, to: Square, leftFt: number): string {
  return `Mover ${formatMeters(squaresToMeters(distance(from, to)))}. ${moveDetail(from, to, leftFt)}`;
}

/** The ordinal circle: "2º círculo", with a no-break space so a line never
 * ends on "1º" with "círculo" alone on the next. */
export function circleLabel(level: number): string {
  return level === 0 ? 'Truque' : `${level}º\u00a0círculo`;
}
