import { ChangeDetectionStrategy, Component, computed, effect, input, output, signal, untracked } from '@angular/core';

import type { MapLayers } from '../../core/maps/layers';
import {
  type TileProgress,
  type TileRect,
  type Vision,
  shadeRects,
  tileProgress,
  tileRects,
  tileUrl,
} from '../../core/maps/vision';
import { MapLayersOverlay } from '../map-layers/map-layers';

/** The whole picture of a map the viewer reads whole (a master): its URL and size. */
export interface FogImage {
  readonly url: string;
  readonly width: number;
  readonly height: number;
}

/**
 * The picture of a map with the fog of war on, as one viewer sees it (MR-036,
 * RN-10, MAP-LANGUAGE.md): the tiles the server made for them, the map's layers
 * (walls, terrain, cover) over them, and every square shaded by what the viewer
 * sees of it. It sits inside a surface that is as big as the map (the map view's
 * stage, the combat map's surface) and fills it.
 *
 * - **Tiles.** A player never gets the image whole, only tiles of 16 × 16
 *   squares. Each is an `<img>` at its place; the URL carries the tile's revision
 *   (`r`), so a tile that did not change keeps its URL and the browser keeps its
 *   copy, and a changed one is fetched again while the old pixels stay on screen
 *   (a tile only ever gains squares). A tile that has not arrived is a grey striped
 *   place with a dashed border, still (no animation). A tile the viewer does not
 *   have is not asked for: its squares are solid black. The master reading the
 *   whole image (no tiles) passes `image` instead.
 * - **Shading.** One state per square (`GetMapVision`): penumbra under 25 % black,
 *   "no escuro, em cinza" without colour (a backdrop filter, so the layers go grey
 *   too), "já visto" under 60 % black with fine dots, "não visto" solid black.
 *   Squares of a kind are joined into blocks. The pieces carry `data-shade`, and
 *   the host carries the counts, so a test reads the shading without pixels.
 * - **Loading.** `settledChange` tells which tiles are in; the screen says "parte N
 *   de M" and keeps the tokens off the places still waiting.
 *
 * Presentational: it never calls the API. Decorative for assistive tech: the
 * legend, the caption and the lists say what the map holds.
 */
@Component({
  selector: 'app-fog-base',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MapLayersOverlay],
  templateUrl: './fog-base.html',
  styleUrl: './fog-base.scss',
  host: {
    'aria-hidden': 'true',
    '[style.--cols]': 'vision().columns',
    '[style.--rows]': 'vision().rows',
    '[attr.data-tiles]': 'rects().length',
    '[attr.data-tiles-ready]': 'progress().done',
    '[attr.data-shaded]': 'shaded()',
  },
})
export class FogBase {
  readonly vision = input.required<Vision>();
  readonly layers = input<MapLayers | null>(null);
  /** The whole picture, when the viewer reads it whole (the master, or a map without the fog). */
  readonly image = input<FogImage | null>(null);
  /** The master reading as a player's character ("Ver como"): the tile URLs say so. */
  readonly forCharacter = input<string | null>(null);

  /** The places whose tile has arrived (or failed: a place that will never come is not waited for). */
  readonly settledChange = output<ReadonlySet<string>>();

  /** The tiles that have settled at least once, by place. */
  private readonly settled = signal<ReadonlySet<string>>(new Set());

  /** The route of the tiles: another map is another set of tiles (a changed revision is not). */
  private readonly route = computed(() => this.vision().tilesPath);
  protected readonly rects = computed(() => tileRects(this.vision()));
  protected readonly progress = computed(() => tileProgress(this.rects(), this.settled()));
  protected readonly blocks = computed(() => shadeRects(this.vision()));
  protected readonly grey = computed(() => this.blocks().filter((b) => b.shade === 'grey'));
  protected readonly dark = computed(() => this.blocks().filter((b) => b.shade !== 'grey'));
  protected readonly shaded = computed(() => {
    const counts: Record<string, number> = { dim: 0, grey: 0, remembered: 0, unseen: 0 };
    for (const b of this.blocks()) {
      counts[b.shade] += b.cols * b.rows;
    }
    return `dim:${counts['dim']} grey:${counts['grey']} remembered:${counts['remembered']} unseen:${counts['unseen']}`;
  });

  constructor() {
    // Another route, map or viewer is another set of tiles: nothing has arrived yet.
    effect(() => {
      this.route();
      this.forCharacter();
      untracked(() => this.settled.set(new Set()));
    });
    effect(() => this.settledChange.emit(this.settled()));
  }

  protected src(rect: TileRect): string {
    return tileUrl(this.vision(), rect, this.forCharacter());
  }

  protected pending(rect: TileRect): boolean {
    return !this.settled().has(rect.key);
  }

  protected settle(rect: TileRect): void {
    if (!this.settled().has(rect.key)) {
      this.settled.update((set) => new Set(set).add(rect.key));
    }
  }

  protected pct(value: number, of: number): number {
    return (value / of) * 100;
  }
}
