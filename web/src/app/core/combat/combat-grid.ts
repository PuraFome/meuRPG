/**
 * The combat grid's geometry, kept apart from the components so it is tested
 * without a DOM (RN-21): squares of 1,5 m (5 ft) across and down, counted from
 * 0 at the top left, and where a square sits on the map. Rules (what a move
 * costs, where a combatant can go, what blocks it) are the server's, read
 * through `GetMoveOptions`; this file only places and steps squares. The
 * distances in words are `core/units.ts`'s.
 */

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

/** The ordinal circle: "2º círculo", with a no-break space so a line never
 * ends on "1º" with "círculo" alone on the next. */
export function circleLabel(level: number): string {
  return level === 0 ? 'Truque' : `${level}º\u00a0círculo`;
}
