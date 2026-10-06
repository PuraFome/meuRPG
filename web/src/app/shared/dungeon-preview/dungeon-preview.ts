import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

import type { DungeonDoor, DungeonStair } from '../../../gen/meurpg/maps/v1/dungeons_pb';
import type { MapLayers } from '../../core/maps/layers';
import { doorCountText, doorSquaresOf, dungeonSizeText, roomCountText, stairCountText, wallPath } from '../../core/maps/dungeon-layout';
import { MapLayersOverlay } from '../map-layers/map-layers';
import { MapLayersLegend } from '../map-layers/map-layers-legend';
import { StairMark } from '../map-layers/stair-mark';

/** What the server's `PreviewDungeon` returns of a layout (the browser draws it; nothing here generates). */
export interface DungeonLayout {
  readonly width: number;
  readonly height: number;
  /** One bit a square, set where a creature can stand (the walls layer's byte layout). */
  readonly open: Uint8Array;
  readonly doors: readonly DungeonDoor[];
  readonly stairs: readonly DungeonStair[];
  readonly roomCount: number;
}

let nextId = 0;

/**
 * The dungeon preview of "Gerar masmorra" (E10-05 1): the floor as paper with a faint grid, the walls and the rock behind them in the image's own colours (the flat dark of the generated image) with the wall mark the map draws over it (the veil and the
 * hatch of MAP-LANGUAGE-E10, the same direction as the editor's), as one shape, not a square each, the doors by kind and the stairs drawn over it exactly as the map will draw them
 * (`app-door-mark`, `app-stair-mark`), and under it the counts and the legend of what the drawing shows. The secret door appears with its
 * mark and the crossed eye in the legend: this page is the master's. The picture is `role="img"` with the counts as its name; its parts
 * are decorative.
 */
@Component({
  selector: 'app-dungeon-preview',
  imports: [MapLayersLegend, MapLayersOverlay, StairMark],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="dp" [style.--cols]="layout().width" [style.--rows]="layout().height" [style.aspect-ratio]="layout().width + ' / ' + layout().height" role="img" [attr.aria-label]="description()">
      <svg class="dp__svg" aria-hidden="true" preserveAspectRatio="none" [attr.viewBox]="'0 0 ' + layout().width + ' ' + layout().height">
        <defs>
          <pattern [attr.id]="patternId" patternUnits="userSpaceOnUse" width="0.42" height="0.42" patternTransform="rotate(-45)">
            <path d="M0 0V0.42" class="dp__line" />
          </pattern>
        </defs>
        <path class="dp__grid" [attr.d]="gridPath()" />
        <path class="dp__rock" [attr.d]="walls()" />
        <path class="dp__veil" [attr.d]="walls()" />
        <path class="dp__hatch" [attr.d]="walls()" [attr.fill]="'url(#' + patternId + ')'" />
      </svg>
      <app-map-layers [layers]="doorLayers()" />
      @for (s of layout().stairs; track $index) {
        <span class="dp__stair" [style.left.%]="((s.x + 0.5) / layout().width) * 100" [style.top.%]="((s.y + 0.5) / layout().height) * 100">
          <app-stair-mark [direction]="s.up ? 'up' : 'down'" />
        </span>
      }
    </div>
    <p class="dp__counts">
      <span><b>{{ roomCountText(layout().roomCount) }}</b></span>
      <span><b>{{ doorsText() }}</b></span>
      <span><b>{{ stairsText() }}</b></span>
      <span><b>{{ sizeText() }}</b></span>
    </p>
    <app-map-layers-legend [layers]="doorLayers()" [showWalls]="true" [stairs]="stairKinds()" />
  `,
  styleUrl: './dungeon-preview.scss',
})
export class DungeonPreview {
  readonly layout = input.required<DungeonLayout>();

  protected readonly patternId = `dp-hatch-${nextId++}`;
  protected readonly roomCountText = roomCountText;

  protected readonly walls = computed(() => wallPath(this.layout().open, this.layout().width, this.layout().height));
  protected readonly doorLayers = computed<MapLayers>(() => ({
    columns: this.layout().width,
    rows: this.layout().height,
    walls: [],
    terrain: [],
    half: [],
    threeQuarters: [],
    doors: doorSquaresOf(this.layout().doors),
  }));
  protected readonly stairKinds = computed(() => ({ up: this.layout().stairs.some((s) => s.up), down: this.layout().stairs.some((s) => !s.up) }));
  protected readonly gridPath = computed(() => {
    const { width, height } = this.layout();
    const parts: string[] = [];
    for (let c = 0; c <= width; c++) {
      parts.push(`M${c} 0V${height}`);
    }
    for (let r = 0; r <= height; r++) {
      parts.push(`M0 ${r}H${width}`);
    }
    return parts.join('');
  });
  protected readonly doorsText = computed(() => doorCountText(this.layout().doors));
  protected readonly stairsText = computed(() => stairCountText(this.layout().stairs.length));
  protected readonly sizeText = computed(() => `${this.layout().width} × ${this.layout().height} quadrados`);
  protected readonly description = computed(
    () => `Prévia da masmorra: ${roomCountText(this.layout().roomCount)}, ${this.doorsText()}, ${this.stairsText()}, ${this.sizeText()} (${dungeonSizeText(this.layout().width, this.layout().height)}).`,
  );
}
