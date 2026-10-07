/** One trap's area as a box in percent of the map, or `null` when the map has no grid. */
export interface PinArea {
  readonly left: number;
  readonly top: number;
  readonly width: number;
  readonly height: number;
}

/**
 * The block of squares a trap covers, drawn only (MAP-LANGUAGE.md): its own square, or the
 * `size` x `size` block around it, the point's square being the middle one (the first of the two
 * middle ones for an even side), clipped to the grid. The same rule the server fires by
 * (`pointSquares` in `maps/fog.go`); the app decides nothing with it, who stands in the area is the
 * server's to say.
 */
export function trapArea(
  xBp: number,
  yBp: number,
  size: number,
  columns: number,
  rows: number,
): PinArea | null {
  if (columns <= 0 || rows <= 0) {
    return null;
  }
  const col = Math.min(columns - 1, Math.floor((xBp / 10000) * columns));
  const row = Math.min(rows - 1, Math.floor((yBp / 10000) * rows));
  const side = Math.max(1, size);
  const first = (side - 1) >> 1;
  const c0 = Math.max(0, col - first);
  const r0 = Math.max(0, row - first);
  const c1 = Math.min(columns, col - first + side);
  const r1 = Math.min(rows, row - first + side);
  return {
    left: (c0 / columns) * 100,
    top: (r0 / rows) * 100,
    width: ((c1 - c0) / columns) * 100,
    height: ((r1 - r0) / rows) * 100,
  };
}
