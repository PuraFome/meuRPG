import { squareAt } from '../combat/combat-grid';
import type { Square } from '../combat/combat-grid';
import type { MapLayers } from './layers';

const BP_MAX = 10000;

/** The door states (`DoorState`) that are not open: a creature does not stand in their square. */
const DOOR_CLOSED = 2;
const DOOR_LOCKED = 3;
const DOOR_BARRED = 4;
const DOOR_SECRET = 5;
const SHUT_DOORS = new Set([DOOR_CLOSED, DOOR_LOCKED, DOOR_BARRED, DOOR_SECRET]);

/** The center of a square is half a square in. */
const HALF = 0.5;

/** Where a new token goes: the center (in basis points) of the free floor square nearest the middle of what is on
 * screen. Free is no token on it; floor is no wall, no three-quarters-cover column and no shut door. The middle itself
 * comes back when the map has no grid yet, or nothing is left. */
export function tokenSpot(
  middle: { xBp: number; yBp: number },
  layers: MapLayers,
  tokens: readonly { xBp: number; yBp: number }[],
): { xBp: number; yBp: number } {
  const { columns, rows } = layers;
  if (columns <= 0 || rows <= 0) {
    return middle;
  }
  const at = (xBp: number, yBp: number): Square =>
    squareAt(xBp / BP_MAX, yBp / BP_MAX, columns, rows);
  const key = (s: Square) => s.row * columns + s.col;
  const blocked = new Set<number>(tokens.map((t) => key(at(t.xBp, t.yBp))));
  for (const s of [...layers.walls, ...layers.threeQuarters]) {
    blocked.add(key(s));
  }
  for (const d of layers.doors ?? []) {
    if (SHUT_DOORS.has(d.state)) {
      blocked.add(key(d));
    }
  }
  const from = at(middle.xBp, middle.yBp);
  let best: Square | null = null;
  let bestDistance = Infinity;
  for (let row = 0; row < rows; row++) {
    for (let col = 0; col < columns; col++) {
      const distance = (col - from.col) ** 2 + (row - from.row) ** 2;
      if (distance < bestDistance && !blocked.has(row * columns + col)) {
        best = { col, row };
        bestDistance = distance;
      }
    }
  }
  if (!best) {
    return middle;
  }
  return {
    xBp: Math.round(((best.col + HALF) / columns) * BP_MAX),
    yBp: Math.round(((best.row + HALF) / rows) * BP_MAX),
  };
}
