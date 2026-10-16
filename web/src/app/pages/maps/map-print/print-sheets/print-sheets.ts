import { Component, computed, input } from '@angular/core';

import type { MapImage } from '../../../../../gen/meurpg/maps/v1/maps_pb';
import { OVERLAP_CM, type MapSizeCm, type Plan } from '../print-math';
import { type Sheet, buildSheet } from './print-sheet';

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
        out.push(buildSheet(plan, this.mapSize(), this.squareCm(), row, column));
      }
    }
    return out;
  });
}
