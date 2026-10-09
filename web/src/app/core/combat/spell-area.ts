import { SpellAreaShape } from '../../../gen/meurpg/rules/v1/rules_pb';
import type { Square } from './combat-grid';

/**
 * The outline of a spell's area while the caster moves it over the map (PM-02a, PM-02b). The browser draws it alone,
 * with no call to the server while the finger or the pointer moves; once the point or the direction is confirmed the
 * server's `PreviewSpellArea` answers with the squares that really are inside (the walls cut them, a wall moves the
 * point to its near side) and those replace this outline. So this file is drawing, not rule: it follows the server's
 * geometry (`rules/grid/area.go`) so the outline and the answer agree on an open floor.
 *
 * - Squares are 5 ft; a square is inside when its center is.
 * - A sphere or a cylinder of radius n squares holds every square with dc² + dr² <= n² around the origin square, the
 *   origin included.
 * - A cone, a line and a cube come out of the caster in one of eight directions; the caster's own square is never
 *   inside.
 * - The direction toward a square is the server's `Toward`, worked out with integers.
 */

/** One of the eight directions: `dx` -1 (west) to 1 (east), `dy` -1 (north) to 1 (south), never both 0. */
export interface Direction {
  readonly dx: number;
  readonly dy: number;
}

/** The form and the size of an area, as `SpellTargets` says them. */
export interface AreaShape {
  readonly shape: SpellAreaShape;
  /** The radius of a sphere or a cylinder, the length of a cone or a line, the side of a cube, in feet. */
  readonly sizeFt: number;
  /** A line's width in feet; 0 is one square. */
  readonly widthFt: number;
}

const FEET_PER_SQUARE = 5;
/** tan(22.5°) ≈ 5/12: the cut between a straight direction and a diagonal one, in integers (the server's). */
const AXIS_RATIO_NUM = 5;
const AXIS_RATIO_DEN = 12;

/** The eight directions clockwise from the east: the order the arrow keys turn a cone, a line or a cube. */
export const DIRECTIONS: readonly Direction[] = [
  { dx: 1, dy: 0 },
  { dx: 1, dy: 1 },
  { dx: 0, dy: 1 },
  { dx: -1, dy: 1 },
  { dx: -1, dy: 0 },
  { dx: -1, dy: -1 },
  { dx: 0, dy: -1 },
  { dx: 1, dy: -1 },
];

const DIRECTION_WORDS = [
  'leste',
  'sudeste',
  'sul',
  'sudoeste',
  'oeste',
  'noroeste',
  'norte',
  'nordeste',
];

function squaresOf(feet: number): number {
  return Math.max(0, Math.floor(feet / FEET_PER_SQUARE));
}

/** Whether the area is centered on a point (a sphere or a cylinder) rather than coming out of the caster. */
export function centered(shape: SpellAreaShape): boolean {
  return shape === SpellAreaShape.SPHERE || shape === SpellAreaShape.CYLINDER;
}

/** The squares of a sphere or a cylinder of `radiusFt` around `origin`, the origin included. */
export function sphereSquares(origin: Square, radiusFt: number): Square[] {
  const n = squaresOf(radiusFt);
  const out: Square[] = [];
  for (let dr = -n; dr <= n; dr++) {
    for (let dc = -n; dc <= n; dc++) {
      if (dc * dc + dr * dr <= n * n) {
        out.push({ col: origin.col + dc, row: origin.row + dr });
      }
    }
  }
  return out;
}

/** The test of a cone or a line at A (along the direction) and P (to its side), both scaled by √2 on a diagonal. */
function inRay(area: AreaShape, d: Direction, along: number, side: number, n: number): boolean {
  const diagonal = d.dx !== 0 && d.dy !== 0;
  const longEnough = diagonal ? along * along <= 2 * n * n : along <= n;
  if (along <= 0 || !longEnough) {
    return false;
  }
  if (area.shape === SpellAreaShape.CONE) {
    return 2 * Math.abs(side) <= along;
  }
  const w = Math.max(squaresOf(area.widthFt), 1);
  if (diagonal) {
    return side >= 0 ? 2 * side * side <= w * w : 2 * side * side < w * w;
  }
  return -w < 2 * side && 2 * side <= w;
}

/** The squares of a cone or a line out of `caster` toward `d`, the caster's square left out. */
export function raySquares(caster: Square, d: Direction, area: AreaShape): Square[] {
  const n = squaresOf(area.sizeFt);
  const out: Square[] = [];
  for (let dr = -n; dr <= n; dr++) {
    for (let dc = -n; dc <= n; dc++) {
      const along = d.dx * dc + d.dy * dr;
      const side = d.dy * dc - d.dx * dr;
      if (inRay(area, d, along, side, n)) {
        out.push({ col: caster.col + dc, row: caster.row + dr });
      }
    }
  }
  return out;
}

/** The offsets a cube takes on one axis: 1..n ahead, -n..-1 behind, centered across (the extra square on the larger side). */
function cubeSpan(component: number, n: number): [number, number] {
  if (component > 0) {
    return [1, n];
  }
  if (component < 0) {
    return [-n, -1];
  }
  const lo = -Math.floor((n - 1) / 2);
  return [lo, lo + n - 1];
}

/** The n × n squares of a cube that touches the caster's square by a face (straight) or a corner (diagonal). */
export function cubeSquares(caster: Square, d: Direction, sideFt: number): Square[] {
  const n = squaresOf(sideFt);
  const [c0, c1] = cubeSpan(d.dx, n);
  const [r0, r1] = cubeSpan(d.dy, n);
  const out: Square[] = [];
  for (let dr = r0; dr <= r1; dr++) {
    for (let dc = c0; dc <= c1; dc++) {
      out.push({ col: caster.col + dc, row: caster.row + dr });
    }
  }
  return out;
}

/** The squares of a shape that comes out of the caster, before the grid's edges cut it. */
export function fromCaster(caster: Square, d: Direction, area: AreaShape): Square[] {
  if (!validDirection(d)) {
    return [];
  }
  if (area.shape === SpellAreaShape.CUBE) {
    return cubeSquares(caster, d, area.sizeFt);
  }
  if (area.shape === SpellAreaShape.CONE || area.shape === SpellAreaShape.LINE) {
    return raySquares(caster, d, area);
  }
  return [];
}

/** The squares of the area placed at `origin` (a sphere, a cylinder) or out of `caster` toward `direction`, inside the grid. */
export function areaSquares(
  area: AreaShape,
  at: { readonly origin: Square } | { readonly caster: Square; readonly direction: Direction },
  columns: number,
  rows: number,
): Square[] {
  const raw =
    'origin' in at
      ? centered(area.shape)
        ? sphereSquares(at.origin, area.sizeFt)
        : []
      : fromCaster(at.caster, at.direction, area);
  return raw.filter((s) => s.col >= 0 && s.row >= 0 && s.col < columns && s.row < rows);
}

export function validDirection(d: Direction): boolean {
  return (
    Number.isInteger(d.dx) &&
    Number.isInteger(d.dy) &&
    Math.abs(d.dx) <= 1 &&
    Math.abs(d.dy) <= 1 &&
    (d.dx !== 0 || d.dy !== 0)
  );
}

/** The direction, among the eight, of `to` as seen from `from` (the server's `Toward`); `null` on the same square. */
export function toward(from: Square, to: Square): Direction | null {
  const dc = to.col - from.col;
  const dr = to.row - from.row;
  if (dc === 0 && dr === 0) {
    return null;
  }
  const ac = Math.abs(dc);
  const ar = Math.abs(dr);
  if (AXIS_RATIO_DEN * ar <= AXIS_RATIO_NUM * ac) {
    return { dx: Math.sign(dc), dy: 0 };
  }
  if (AXIS_RATIO_DEN * ac <= AXIS_RATIO_NUM * ar) {
    return { dx: 0, dy: Math.sign(dr) };
  }
  return { dx: Math.sign(dc), dy: Math.sign(dr) };
}

/** The direction `steps` turns of 45° clockwise from `d` (negative: counter-clockwise). */
export function rotate(d: Direction, steps: number): Direction {
  const i = DIRECTIONS.findIndex((x) => x.dx === d.dx && x.dy === d.dy);
  const n = DIRECTIONS.length;
  return DIRECTIONS[((((i < 0 ? 0 : i) + steps) % n) + n) % n];
}

/** "leste", "nordeste": the direction in a word, for the announcements. */
export function directionWord(d: Direction): string {
  const i = DIRECTIONS.findIndex((x) => x.dx === d.dx && x.dy === d.dy);
  return i < 0 ? '' : DIRECTION_WORDS[i];
}

/** The distance for a spell's range, in feet: the squares between the centers rounded down, 5 ft each (RN-21, the server's `RangeFt`). */
export function rangeFt(from: Square, to: Square): number {
  const dc = to.col - from.col;
  const dr = to.row - from.row;
  const n = dc * dc + dr * dr;
  let r = Math.floor(Math.sqrt(n));
  while (r * r > n) {
    r--;
  }
  while ((r + 1) * (r + 1) <= n) {
    r++;
  }
  return r * FEET_PER_SQUARE;
}

/**
 * The outline of a set of squares as an SVG path in squares (one unit a square): every side of a square whose neighbour
 * on that side is not in the set. One path draws a circle, a cone or the cut a wall made, and a gap inside it too.
 */
export function outlinePath(squares: readonly Square[]): string {
  const set = new Set(squares.map((s) => `${s.col},${s.row}`));
  const has = (c: number, r: number) => set.has(`${c},${r}`);
  const parts: string[] = [];
  for (const { col: c, row: r } of squares) {
    if (!has(c, r - 1)) {
      parts.push(`M${c} ${r}h1`);
    }
    if (!has(c, r + 1)) {
      parts.push(`M${c} ${r + 1}h1`);
    }
    if (!has(c - 1, r)) {
      parts.push(`M${c} ${r}v1`);
    }
    if (!has(c + 1, r)) {
      parts.push(`M${c + 1} ${r}v1`);
    }
  }
  return parts.join('');
}

/** The squares as one SVG path of unit rectangles (the area's fill, the dimmed squares beyond the range). */
export function cellsPath(squares: readonly Square[]): string {
  return squares.map((s) => `M${s.col} ${s.row}h1v1h-1z`).join('');
}

/** The squares of the grid farther than `rangeFt` from `from` (RN-21 distance): the ones the picker dims. */
export function beyondRange(from: Square, range: number, columns: number, rows: number): Square[] {
  const out: Square[] = [];
  for (let row = 0; row < rows; row++) {
    for (let col = 0; col < columns; col++) {
      if (rangeFt(from, { col, row }) > range) {
        out.push({ col, row });
      }
    }
  }
  return out;
}

/** Whether two squares are the same. */
export function sameSquare(a: Square | null | undefined, b: Square | null | undefined): boolean {
  return !!a && !!b && a.col === b.col && a.row === b.row;
}
