import { Component, computed, input } from '@angular/core';

import type { MapImage } from '../../../../../gen/meurpg/maps/v1/maps_pb';
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
interface Sheet {
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

/**
 * What the printer gets (state 6 of E8-12): one sheet per page, each a tile
 * of the map's image with the grid drawn over it at the chosen size. Nothing
 * of this shows on screen. The image is the master's (`/images/<id>`), the
 * same URL on every sheet, so it is fetched once; the grid is SVG in cm over
 * it, never an edit of the picture. Points and tokens stay off.
 *
 * The 1 cm bands a sheet shares with its neighbours are tinted, with a dashed
 * line along the inner edge, a cross at each inner corner and the neighbour's
 * name on the band. The page label sits in the top band (4 mm, about 11 pt),
 * and a 5 cm ruler says "confira a escala". The sheet's own colours do not
 * follow the theme: paper is white and ink is black.
 */
@Component({
  selector: 'app-print-sheets',
  templateUrl: './print-sheets.html',
  styleUrl: './print-sheets.scss',
})
export class PrintSheets {
  readonly plan = input.required<Plan>();
  readonly image = input.required<MapImage>();
  readonly mapSize = input.required<MapSizeCm>();
  readonly squareCm = input.required<number>();

  protected readonly overlap = OVERLAP_CM;
  protected readonly viewBox = computed(() => `0 0 ${this.plan().usableW} ${this.plan().usableH}`);
  protected readonly squareLabel = computed(() => String(this.squareCm()).replace('.', ','));

  protected readonly sheets = computed<Sheet[]>(() => {
    const plan = this.plan();
    const out: Sheet[] = [];
    for (let row = 0; row < plan.rows; row++) {
      for (let column = 0; column < plan.columns; column++) {
        out.push(this.sheet(plan, row, column));
      }
    }
    return out;
  });

  private sheet(plan: Plan, row: number, column: number): Sheet {
    const o = sheetOrigin(plan, row, column);
    const lines = gridLines(plan, this.mapSize(), this.squareCm(), row, column);
    const grid = [
      ...lines.xs.map((x) => `M${r(x)} 0V${r(lines.height)}`),
      ...lines.ys.map((y) => `M0 ${r(y)}H${r(lines.width)}`),
    ].join('');

    const hasLeft = column > 0;
    const hasRight = column < plan.columns - 1;
    const hasAbove = row > 0;
    const hasBelow = row < plan.rows - 1;
    const w = plan.usableW;
    const h = plan.usableH;
    const bands: { x: number; y: number; w: number; h: number }[] = [];
    if (hasLeft) bands.push({ x: 0, y: 0, w: OVERLAP_CM, h });
    if (hasRight) bands.push({ x: w - OVERLAP_CM, y: 0, w: OVERLAP_CM, h });
    if (hasAbove) bands.push({ x: 0, y: 0, w, h: OVERLAP_CM });
    if (hasBelow) bands.push({ x: 0, y: h - OVERLAP_CM, w, h: OVERLAP_CM });

    // The inner edges of the bands, and a cross where they meet.
    const x0 = hasLeft ? OVERLAP_CM : 0;
    const x1 = hasRight ? w - OVERLAP_CM : w;
    const y0 = hasAbove ? OVERLAP_CM : 0;
    const y1 = hasBelow ? h - OVERLAP_CM : h;
    const marks: string[] = [];
    if (hasLeft) marks.push(`M${x0} 0V${h}`);
    if (hasRight) marks.push(`M${x1} 0V${h}`);
    if (hasAbove) marks.push(`M0 ${y0}H${w}`);
    if (hasBelow) marks.push(`M0 ${y1}H${w}`);
    const arm = 0.35;
    const crosses: string[] = [];
    if (hasLeft || hasRight || hasAbove || hasBelow) {
      for (const [cx, cy] of [
        [x0, y0],
        [x1, y0],
        [x0, y1],
        [x1, y1],
      ]) {
        crosses.push(`M${cx - arm} ${cy}H${cx + arm}M${cx} ${cy - arm}V${cy + arm}`);
      }
    }

    return {
      key: sheetName(row, column),
      label: sheetLabel(row, column),
      imageLeft: -o.x,
      imageTop: -o.y,
      grid,
      bands,
      marks: marks.join(''),
      crosses: crosses.join(''),
      above: hasAbove ? sheetName(row - 1, column) : null,
      below: hasBelow ? sheetName(row + 1, column) : null,
      left: hasLeft ? sheetName(row, column - 1) : null,
      right: hasRight ? sheetName(row, column + 1) : null,
    };
  }
}

function r(n: number): number {
  return Math.round(n * 1000) / 1000;
}
