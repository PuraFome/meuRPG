import {
  OVERLAP_CM,
  type MapSizeCm,
  type Plan,
  gridLines,
  sheetLabel,
  sheetName,
  sheetOrigin,
} from '../print-math';

/** What one printed sheet draws, ready for the template. */
export interface Sheet {
  readonly key: string;
  readonly label: string;
  /** Where the map's image starts on the sheet, in cm (negative: cut off). */
  readonly imageLeft: number;
  readonly imageTop: number;
  /** The grid, one SVG path in cm on the sheet. */
  readonly grid: string;
  readonly bands: readonly { x: number; y: number; w: number; h: number }[];
  /** The dashed edge of each band and the crosses at the corners. */
  readonly marks: string;
  readonly crosses: string;
  /** The neighbours, named on the sheet's four bands. */
  readonly above: string | null;
  readonly below: string | null;
  readonly left: string | null;
  readonly right: string | null;
}

/** The length of the arms of a cross at an inner corner, in cm. */
const CROSS_ARM_CM = 0.35;
/** Digits kept in the grid's path: a thousandth of a cm is far below what a printer draws. */
const PATH_PRECISION = 1000;

function round(n: number): number {
  return Math.round(n * PATH_PRECISION) / PATH_PRECISION;
}

/** Which of the four sides of a sheet have a neighbour. */
interface Sides {
  readonly left: boolean;
  readonly right: boolean;
  readonly above: boolean;
  readonly below: boolean;
}

type Band = { x: number; y: number; w: number; h: number };

/** The 1 cm bands a sheet shares with its neighbours. */
function bandsOf(has: Sides, w: number, h: number): Band[] {
  const bands: Band[] = [];
  if (has.left) bands.push({ x: 0, y: 0, w: OVERLAP_CM, h });
  if (has.right) bands.push({ x: w - OVERLAP_CM, y: 0, w: OVERLAP_CM, h });
  if (has.above) bands.push({ x: 0, y: 0, w, h: OVERLAP_CM });
  if (has.below) bands.push({ x: 0, y: h - OVERLAP_CM, w, h: OVERLAP_CM });
  return bands;
}

/** The inner edges of the bands (where they meet is where the crosses go). */
function edgesOf(has: Sides, w: number, h: number) {
  return {
    x0: has.left ? OVERLAP_CM : 0,
    x1: has.right ? w - OVERLAP_CM : w,
    y0: has.above ? OVERLAP_CM : 0,
    y1: has.below ? h - OVERLAP_CM : h,
  };
}

/** The dashed line along the inner edge of each band. */
function marksOf(has: Sides, e: ReturnType<typeof edgesOf>, w: number, h: number): string {
  const marks: string[] = [];
  if (has.left) marks.push(`M${e.x0} 0V${h}`);
  if (has.right) marks.push(`M${e.x1} 0V${h}`);
  if (has.above) marks.push(`M0 ${e.y0}H${w}`);
  if (has.below) marks.push(`M0 ${e.y1}H${w}`);
  return marks.join('');
}

/** A cross at each inner corner; a sheet with no neighbour has none. */
function crossesOf(has: Sides, e: ReturnType<typeof edgesOf>): string {
  if (!has.left && !has.right && !has.above && !has.below) {
    return '';
  }
  const corners = [
    [e.x0, e.y0],
    [e.x1, e.y0],
    [e.x0, e.y1],
    [e.x1, e.y1],
  ];
  return corners
    .map(
      ([cx, cy]) =>
        `M${cx - CROSS_ARM_CM} ${cy}H${cx + CROSS_ARM_CM}M${cx} ${cy - CROSS_ARM_CM}V${cy + CROSS_ARM_CM}`,
    )
    .join('');
}

/** What sheet `row`, `column` of the plan draws: its tile of the image, the grid over it, the bands and the neighbours' names. */
export function buildSheet(
  plan: Plan,
  mapSize: MapSizeCm,
  squareCm: number,
  row: number,
  column: number,
): Sheet {
  const o = sheetOrigin(plan, row, column);
  const lines = gridLines(plan, mapSize, squareCm, row, column);
  const grid = [
    ...lines.xs.map((x) => `M${round(x)} 0V${round(lines.height)}`),
    ...lines.ys.map((y) => `M0 ${round(y)}H${round(lines.width)}`),
  ].join('');
  const has: Sides = {
    left: column > 0,
    right: column < plan.columns - 1,
    above: row > 0,
    below: row < plan.rows - 1,
  };
  const w = plan.usableW;
  const h = plan.usableH;
  const edges = edgesOf(has, w, h);
  return {
    key: sheetName(row, column),
    label: sheetLabel(row, column),
    imageLeft: -o.x,
    imageTop: -o.y,
    grid,
    bands: bandsOf(has, w, h),
    marks: marksOf(has, edges, w, h),
    crosses: crossesOf(has, edges),
    above: has.above ? sheetName(row - 1, column) : null,
    below: has.below ? sheetName(row + 1, column) : null,
    left: has.left ? sheetName(row, column - 1) : null,
    right: has.right ? sheetName(row, column + 1) : null,
  };
}
