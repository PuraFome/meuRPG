import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

import type { MapLayers } from '../../core/maps/layers';

/**
 * The map's painted layers drawn over its image (Etapa 9, MAP-LANGUAGE.md),
 * the same on every map of the app: **Parede** the dark diagonal hatch,
 * **Terreno difícil** the cross hatch, **Cobertura** a pictogram in the middle
 * of the square (a dotted square filled from the bottom to one half, "Meia
 * cobertura", or to three quarters, "Três quartos"). Each mark has a name in
 * `MapLayersLegend`: never colour alone, and never an object's name, because
 * the app knows only the degree.
 *
 * Presentational and pure drawing: it takes the decoded squares
 * (`core/maps/layers.ts`) and decides nothing about what they do. It sits in
 * a map surface that sets `--cols` and `--rows` (`app-combat-map` does), so a
 * square is `100% / --cols` wide. The map editor (9.12) and the session fog
 * (9.13) reuse it as it is; the fog's shading (the darkening of what was only
 * remembered) is theirs, drawn over or under it.
 */
@Component({
  selector: 'app-map-layers',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @for (s of layers().terrain; track s.row * 1000 + s.col) {
      <span class="sq sq--terrain" [style.left.%]="x(s.col)" [style.top.%]="y(s.row)"></span>
    }
    @for (s of layers().walls; track s.row * 1000 + s.col) {
      <span class="sq sq--wall" [style.left.%]="x(s.col)" [style.top.%]="y(s.row)"></span>
    }
    @for (s of layers().half; track s.row * 1000 + s.col) {
      <span class="sq sq--cover sq--half" [style.left.%]="x(s.col)" [style.top.%]="y(s.row)"></span>
    }
    @for (s of layers().threeQuarters; track s.row * 1000 + s.col) {
      <span class="sq sq--cover sq--three" [style.left.%]="x(s.col)" [style.top.%]="y(s.row)"></span>
    }
  `,
  styleUrl: './map-layers.scss',
  host: { 'aria-hidden': 'true' },
})
export class MapLayersOverlay {
  readonly layers = input.required<MapLayers>();
  /** The grid, when the surface is not the one that set it (the editor). */
  private readonly columns = computed(() => this.layers().columns);
  private readonly rows = computed(() => this.layers().rows);

  protected x(col: number): number {
    return (col / this.columns()) * 100;
  }

  protected y(row: number): number {
    return (row / this.rows()) * 100;
  }
}
