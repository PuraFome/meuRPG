import { signal } from '@angular/core';

import type { MapsClient } from './maps-client';
import { PaintedLayers } from './paint-layers';
import { PaintQueue } from './paint-queue';

/** `loading`: the first read of the painted layers has not answered; `ready`: they are on screen and painting is on; `failed`: the read failed. */
export type PaintReadiness = 'loading' | 'ready' | 'failed';

/**
 * The master's painting of one map, apart from the editor's screen (MR-034): the painted layers as he sees them (`PaintedLayers`),
 * the queue that saves his strokes (`PaintQueue`, which remembers the map each stroke was made on) and whether he can paint at
 * all. Painting is off until the layers have been read once: a stroke made before them would be drawn over an empty map and
 * "Tudo salvo" would be a lie. The layers are read again when the grid or the map changes (a new grid clears them, so what waited
 * to be saved is dropped) and when a refusal dropped strokes; never while strokes wait, because the copy on the screen is then the
 * newer one, and once more when the queue drains if a read was skipped.
 */
export class PaintSession {
  readonly layers = new PaintedLayers(true);
  readonly queue: PaintQueue;
  readonly readiness = signal<PaintReadiness>('loading');

  private key = '';
  private generation = 0;
  private skipped: { campaignId: string; mapId: string; columns: number } | null = null;
  private lastRefused = 0;

  constructor(private readonly api: Pick<MapsClient, 'layers' | 'paint'>) {
    this.queue = new PaintQueue((t, layer, value, squares) => this.api.paint(t.campaignId, t.mapId, layer, value, squares).then(() => undefined));
  }

  /** Shows the painted layers of a map: a new map or a new grid starts again; the same key is a plain read only when forced. */
  async open(campaignId: string, mapId: string, columns: number, revision: number, force = false): Promise<void> {
    const key = `${campaignId}|${mapId}|${columns}|${revision}`;
    if (!force && key === this.key) {
      return;
    }
    const gridChanged = this.key !== '' && key.split('|').slice(0, 3).join('|') !== this.key.split('|').slice(0, 3).join('|');
    this.key = key;
    if (gridChanged || columns <= 0) {
      // The strokes that wait belong to a grid that is gone.
      this.queue.clear();
    }
    if (columns <= 0) {
      this.generation++;
      this.layers.clear();
      this.readiness.set('ready');
      return;
    }
    if (this.queue.busy) {
      this.skipped = { campaignId, mapId, columns };
      this.queue.whenIdle(() => {
        const again = this.skipped;
        this.skipped = null;
        if (again) {
          void this.open(again.campaignId, again.mapId, again.columns, revision, true);
        }
      });
      return;
    }
    const generation = ++this.generation;
    if (this.readiness() !== 'ready') {
      this.readiness.set('loading');
    }
    try {
      const packed = await this.api.layers(campaignId, mapId);
      if (generation !== this.generation) {
        return;
      }
      if (this.queue.busy) {
        // A stroke was made while the read was on its way: the screen has the newer copy; read again when it is saved.
        this.skipped = { campaignId, mapId, columns };
        this.queue.whenIdle(() => void this.open(campaignId, mapId, columns, revision, true));
        this.readiness.set('ready');
        return;
      }
      this.layers.load(packed);
      this.readiness.set('ready');
    } catch {
      if (generation === this.generation) {
        this.key = '';
        this.readiness.set('failed');
      }
    }
  }

  /** A refusal dropped strokes (or the page asks): read what the server has, once per refusal. */
  syncAfterRefusal(campaignId: string, mapId: string, columns: number, revision: number): void {
    const n = this.queue.refused();
    if (n !== this.lastRefused) {
      this.lastRefused = n;
      void this.open(campaignId, mapId, columns, revision, true);
    }
  }
}
