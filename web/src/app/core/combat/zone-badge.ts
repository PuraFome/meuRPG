import type { Square } from './combat-grid';

/** A point of the grid, in squares from the map's top left corner (a corner between squares, not a square). */
export interface BadgeSpot {
  readonly col: number;
  readonly row: number;
}

/**
 * Where each zone's letter badge sits: a corner of its footprint's box, the top left first, then the bottom right, top right and
 * bottom left when a badge already holds that corner (a zone laid over another). The point is kept inside the map; the layer
 * keeps the badge's own size inside it too.
 */
export function badgeSpots(
  footprints: readonly (readonly Square[])[],
  columns: number,
  rows: number,
): BadgeSpot[] {
  const taken: BadgeSpot[] = [];
  return footprints.map((squares) => {
    const cols = squares.map((s) => s.col);
    const rws = squares.map((s) => s.row);
    const [c0, c1] = [Math.min(...cols), Math.max(...cols) + 1];
    const [r0, r1] = [Math.min(...rws), Math.max(...rws) + 1];
    const corners: BadgeSpot[] = [
      { col: c0, row: r0 },
      { col: c1, row: r1 },
      { col: c1, row: r0 },
      { col: c0, row: r1 },
    ].map((p) => ({
      col: Math.min(Math.max(p.col, 0), columns),
      row: Math.min(Math.max(p.row, 0), rows),
    }));
    const free = corners.find((p) =>
      taken.every((t) => Math.abs(t.col - p.col) >= 1 || Math.abs(t.row - p.row) >= 1),
    );
    const spot = free ?? corners[0];
    taken.push(spot);
    return spot;
  });
}
