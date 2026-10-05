import { signal } from '@angular/core';
import { Code, ConnectError } from '@connectrpc/connect';

import type { GetMapVisionResponse } from '../../../gen/meurpg/maps/v1/maps_pb';
import { type MapLayers, NO_LAYERS, type PackedLayers, decodeLayers } from './layers';
import { type Vision, decodeVision } from './vision';

/**
 * - `idle`: no fog map is open;
 * - `loading`: the first read of the open map;
 * - `ready`: the vision is on screen (a later read keeps it there);
 * - `error`: the first read failed.
 */
export type FogStatus = 'idle' | 'loading' | 'ready' | 'error';

/** Why the last read failed, by the code and the typed detail, never by the message:
 * `gone` (`not_found`: the character a master reads as died or left, or the map is hidden), `no-grid`
 * (`failed_precondition`: the map has no grid), `offline` (`unavailable`: the app's "Reconectando…"), `failed`. */
export type FogError = 'gone' | 'no-grid' | 'offline' | 'failed';

export function fogError(err: unknown): FogError {
  switch (ConnectError.from(err, Code.Unavailable).code) {
    case Code.NotFound:
      return 'gone';
    case Code.FailedPrecondition:
      return 'no-grid';
    case Code.Unavailable:
    case Code.Aborted:
      return 'offline';
    default:
      return 'failed';
  }
}

/** Whether two visions draw the same: the same states and the same tiles at the same revisions. A new image with the same states changes the tiles' revisions. */
function sameVision(a: Vision | null, b: Vision): boolean {
  return (
    a !== null &&
    a.revision === b.revision &&
    a.tilesPath === b.tilesPath &&
    a.tiles.length === b.tiles.length &&
    a.tiles.every((t, i) => t.tx === b.tiles[i]?.tx && t.ty === b.tiles[i]?.ty && t.revision === b.tiles[i]?.revision)
  );
}

/**
 * What one viewer sees of a map with the fog of war on (MR-036, RN-10): the
 * vision (`GetMapVision`, the states and the tiles) and the layers (walls,
 * terrain, cover) filtered to it, read together and read again on
 * `vision_changed`. The viewer is the signed-in player, or, with `as`, the
 * master reading as that character's player ("Ver como"). Pure TypeScript, so
 * the rules (a stale answer never overwrites a newer one; an unchanged
 * revision keeps the same object, so nothing is drawn again; a failed read
 * keeps what is on screen) are tested without a DOM.
 */
export class FogView {
  readonly vision = signal<Vision | null>(null);
  readonly layers = signal<MapLayers>(NO_LAYERS);
  readonly status = signal<FogStatus>('idle');
  /** Why the last read failed; `null` after a read that worked. */
  readonly error = signal<FogError | null>(null);

  private mapId: string | null = null;
  private as: string | null = null;
  private generation = 0;

  constructor(
    private readonly loadVision: (mapId: string, as: string | null) => Promise<GetMapVisionResponse>,
    private readonly loadLayers: (mapId: string, as: string | null) => Promise<PackedLayers>,
  ) {}

  /** Shows what `as` (or the signed-in player) sees of `mapId`; `null` closes it.
   * The same map and viewer again is a plain read. */
  async open(mapId: string | null, as: string | null = null): Promise<void> {
    if (mapId === this.mapId && as === this.as && mapId !== null) {
      return this.refresh();
    }
    this.mapId = mapId;
    this.as = as;
    this.generation++;
    this.vision.set(null);
    this.layers.set(NO_LAYERS);
    this.error.set(null);
    if (mapId === null) {
      this.status.set('idle');
      return;
    }
    this.status.set('loading');
    await this.read();
  }

  /** Reads the open map again (`vision_changed`, `map_changed`). */
  async refresh(): Promise<void> {
    if (this.mapId !== null) {
      await this.read();
    }
  }

  private async read(): Promise<void> {
    const mapId = this.mapId;
    if (mapId === null) {
      return;
    }
    const generation = ++this.generation;
    try {
      const [vision, layers] = await Promise.all([
        this.loadVision(mapId, this.as),
        this.loadLayers(mapId, this.as),
      ]);
      if (generation !== this.generation) {
        return;
      }
      const next = decodeVision(vision);
      if (!sameVision(this.vision(), next)) {
        this.vision.set(next);
      }
      this.error.set(null);
      this.layers.set(decodeLayers(layers));
      this.status.set('ready');
    } catch (err) {
      if (generation === this.generation) {
        this.error.set(fogError(err));
        if (this.status() !== 'ready') {
          this.status.set('error');
        }
      }
    }
  }
}
