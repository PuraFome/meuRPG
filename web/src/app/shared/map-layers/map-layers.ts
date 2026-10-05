import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

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
 *
 * Two more marks belong to the editor (9.12): the **painted light** (`lightGlyphs`: a small glyph in the
 * corner of the square, a sun for Claro, a half moon for Penumbra, a moon for Escuro, never a texture; the
 * master alone has that layer) and the **brush cursor** (`cursor`: the outline of the squares the next
 * stroke would paint).
 */
/** The squares the next stroke would paint: the top-left one and the size, already kept inside the grid. */
export interface BrushCursor {
  readonly col: number;
  readonly row: number;
  readonly w: number;
  readonly h: number;
  readonly erase?: boolean;
}

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
    @if (lightGlyphs() && layers().light; as light) {
      @for (s of light.bright; track s.row * 1000 + s.col) {
        <span class="sq sq--light" [style.left.%]="x(s.col)" [style.top.%]="y(s.row)"><mat-icon class="lg">light_mode</mat-icon></span>
      }
      @for (s of light.dim; track s.row * 1000 + s.col) {
        <span class="sq sq--light" [style.left.%]="x(s.col)" [style.top.%]="y(s.row)"><mat-icon class="lg">contrast</mat-icon></span>
      }
      @for (s of light.dark; track s.row * 1000 + s.col) {
        <span class="sq sq--light" [style.left.%]="x(s.col)" [style.top.%]="y(s.row)"><mat-icon class="lg">dark_mode</mat-icon></span>
      }
    }
    @if (cursor(); as c) {
      <span
        class="cursor"
        [class.cursor--erase]="c.erase"
        [style.left.%]="x(c.col)"
        [style.top.%]="y(c.row)"
        [style.width.%]="(c.w / columns()) * 100"
        [style.height.%]="(c.h / rows()) * 100"
      ></span>
    }
  `,
  imports: [MatIconModule],
  styleUrl: './map-layers.scss',
  host: { 'aria-hidden': 'true' },
})
export class MapLayersOverlay {
  readonly layers = input.required<MapLayers>();
  /** The painted light as corner glyphs: the master's editor only. */
  readonly lightGlyphs = input(false);
  /** The brush cursor: the squares a stroke would paint (its top-left square and its size in squares), and whether it erases. */
  readonly cursor = input<BrushCursor | null>(null);
  /** The grid, when the surface is not the one that set it (the editor). */
  protected readonly columns = computed(() => this.layers().columns);
  protected readonly rows = computed(() => this.layers().rows);

  protected x(col: number): number {
    return (col / this.columns()) * 100;
  }

  protected y(row: number): number {
    return (row / this.rows()) * 100;
  }
}
