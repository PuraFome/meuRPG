import { signal } from '@angular/core';

import type { GetMapVisionResponse } from '../../../gen/meurpg/maps/v1/maps_pb';
import { FogView } from './fog-view';
import { MapState } from './map-state';
import type { MapsClient } from './maps-client';
import { seenCount, decodeVision } from './vision';

/**
 * The master's "Ver como" (MR-036, E9-03): the map exactly as one character's
 * player gets it — the map's points and tokens (`GetMap` with `as_character_id`),
 * the layers and the vision with its tiles — kept apart from the master's own map
 * so choosing "Todos" is just closing it. Only the master, and only on the session
 * page or the editor: nothing changes for anyone.
 */
export class ViewAsMap {
  readonly map: MapState;
  readonly fog: FogView;
  private as = '';
  private mapId = '';

  constructor(
    private readonly api: MapsClient,
    private readonly campaignId: () => string,
  ) {
    this.map = new MapState((mapId) => this.api.get(this.campaignId(), mapId, this.as));
    this.fog = new FogView(
      (mapId, as) => this.api.vision(this.campaignId(), mapId, as ?? ''),
      (mapId, as) => this.api.layers(this.campaignId(), mapId, as ?? ''),
      () => true,
    );
  }

  /** Shows the map as `characterId`'s player sees it; `null` closes it. */
  async open(mapId: string | null, characterId: string | null): Promise<void> {
    if (mapId === null || characterId === null) {
      this.as = '';
      this.mapId = '';
      await Promise.all([this.map.open(null), this.fog.open(null)]);
      return;
    }
    if (this.as !== characterId || this.mapId !== mapId) {
      this.as = characterId;
      this.mapId = mapId;
      // Another character's view: nothing of the last one stays on screen while this one is read.
      void this.map.open(null);
    }
    await Promise.all([this.map.open(mapId), this.fog.open(mapId, characterId)]);
  }

  /** Reads everything again: the stream tells the master nothing about what a player sees. */
  async refresh(): Promise<void> {
    await Promise.all([this.map.refresh(), this.fog.refresh()]);
  }
}

/**
 * How many squares each character sees on the map ("76 quadrados vistos"), for the
 * "Ver como" list: one `GetMapVision` as each character, counted from the states
 * (counting, not rules). A character whose read failed has `null`.
 */
export class ViewAsCounts {
  readonly counts = signal<ReadonlyMap<string, number | null>>(new Map());
  readonly total = signal(0);
  private generation = 0;

  constructor(
    private readonly load: (mapId: string, characterId: string) => Promise<GetMapVisionResponse>,
  ) {}

  async read(mapId: string, characterIds: readonly string[]): Promise<void> {
    const generation = ++this.generation;
    const entries = await Promise.all(
      characterIds.map(async (id): Promise<[string, number | null, number]> => {
        try {
          const vision = decodeVision(await this.load(mapId, id));
          return [id, seenCount(vision), vision.columns * vision.rows];
        } catch {
          return [id, null, 0];
        }
      }),
    );
    if (generation !== this.generation) {
      return;
    }
    this.counts.set(new Map(entries.map(([id, n]) => [id, n])));
    this.total.set(entries.reduce((max, [, , t]) => Math.max(max, t), 0));
  }
}
