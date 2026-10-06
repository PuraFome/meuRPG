import { signal } from '@angular/core';
import { Code, ConnectError } from '@connectrpc/connect';

import type { GetDungeonRoomsResponse } from '../../../gen/meurpg/maps/v1/dungeons_pb';
import type { DungeonsClient } from './dungeons-client';

/**
 * What the master's map page knows of a generated dungeon: the rooms list, the options and the seed, and whether the image is still the
 * generator's (`GetDungeonRooms`). It is read only for a map the server says is a generated dungeon (`Map.generated_dungeon`, the master's
 * alone), so a player's page never asks. A `not_found` means "no dungeon here"; any other failure keeps `rooms` empty and sets `failed`, so the
 * page says it and offers "Tentar de novo" instead of silently dropping the master's list. A stale answer never replaces a newer read. Plain
 * TypeScript, so it is tested without a DOM.
 */
export class DungeonInfo {
  readonly rooms = signal<GetDungeonRoomsResponse | null>(null);
  /** The last read failed for a reason other than `not_found`. */
  readonly failed = signal(false);
  private generation = 0;

  constructor(private readonly api: Pick<DungeonsClient, 'rooms'>) {}

  async load(campaignId: string, mapId: string): Promise<void> {
    const generation = ++this.generation;
    try {
      const res = await this.api.rooms(campaignId, mapId);
      if (generation === this.generation) {
        this.rooms.set(res);
        this.failed.set(false);
      }
    } catch (err) {
      if (generation === this.generation) {
        this.rooms.set(null);
        this.failed.set(ConnectError.from(err).code !== Code.NotFound);
      }
    }
  }

  clear(): void {
    this.generation++;
    this.rooms.set(null);
    this.failed.set(false);
  }
}
