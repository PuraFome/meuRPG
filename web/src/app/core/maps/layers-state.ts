import { signal } from '@angular/core';

import { type MapLayers, NO_LAYERS, type PackedLayers, decodeLayers } from './layers';

/**
 * The painted layers of the map the combat is on, as signals: read on the
 * first open and again whenever the map's `layers_revision` changes (the
 * server bumps it on each painting, and for a player on a fog map only when
 * what they know changes). A failed read keeps what is on screen: the layers
 * are decoration of the map, never a reason to hide the combat. Pure
 * TypeScript, so the rule (a stale answer never overwrites a newer one) is
 * tested without a DOM.
 */
export class LayersState {
  readonly layers = signal<MapLayers>(NO_LAYERS);
  private key = '';
  private generation = 0;

  constructor(
    private readonly load: (mapId: string) => Promise<PackedLayers>,
    /** The viewer is a player: the doors are decoded with the guard (RN-10). */
    private readonly player: () => boolean = () => false,
  ) {}

  /** Shows the layers of `mapId` at `revision`; `null` clears them. A read for
   * the same map and revision is not repeated. */
  async open(mapId: string | null, revision: number): Promise<void> {
    const key = mapId === null ? '' : `${mapId}@${revision}`;
    if (key === this.key) {
      return;
    }
    this.key = key;
    const generation = ++this.generation;
    if (mapId === null) {
      this.layers.set(NO_LAYERS);
      return;
    }
    try {
      const packed = await this.load(mapId);
      if (generation === this.generation) {
        this.layers.set(decodeLayers(packed, this.player()));
      }
    } catch {
      if (generation === this.generation) {
        // Let the next change try again.
        this.key = '';
      }
    }
  }
}
