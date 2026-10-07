import { Injectable, inject } from '@angular/core';
import type { MessageInitShape } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';

import {
  type DungeonOptionsSchema,
  DungeonService,
  type GetDungeonRoomsResponse,
  type PreviewDungeonResponse,
} from '../../../gen/meurpg/maps/v1/dungeons_pb';
import type { Map as MapMessage, MapPoint } from '../../../gen/meurpg/maps/v1/maps_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';

/** The page's choices as the request carries them (`DungeonOptions`: a field left out takes the generator's default). */
export type DungeonOptionsInit = MessageInitShape<typeof DungeonOptionsSchema>;

/**
 * Thin wrapper around the generated `DungeonService` client (MR-010, RN-26, slice 10.6d): the master's "Gerar masmorra" page, the
 * rooms list of a generated map and "Redesenhar". Every call is the master's; a player gets `not_found` from all of them, so the app
 * never calls this as a player. Callers map the errors to Portuguese (`dungeon-errors.ts`). Tests replace it with
 * `{ provide: DungeonsClient, useValue }`.
 */
@Injectable({ providedIn: 'root' })
export class DungeonsClient {
  private readonly client = createClient(DungeonService, inject(CONNECT_TRANSPORT));

  /** `PreviewDungeon`: the layout for the live preview; stores nothing. No seed asks the server to draw one. */
  preview(
    campaignId: string,
    options: DungeonOptionsInit,
    seed?: bigint,
    signal?: AbortSignal,
  ): Promise<PreviewDungeonResponse> {
    return this.client.previewDungeon(
      { campaignId, options, ...(seed === undefined ? {} : { seed }) },
      { signal },
    );
  }

  /** `CreateDungeonMap`: the hidden map with its fog on. The call goes on in the server if `signal` aborts it. */
  async create(
    campaignId: string,
    name: string,
    options: DungeonOptionsInit,
    seed: bigint,
    idempotencyKey: string,
    signal?: AbortSignal,
  ): Promise<{ map: MapMessage; seed: bigint; roomCount: number }> {
    const res = await this.client.createDungeonMap(
      { campaignId, name, options, seed, idempotencyKey },
      { signal },
    );
    if (!res.map) {
      throw new Error('CreateDungeonMap answered without a map');
    }
    return { map: res.map, seed: res.seed, roomCount: res.roomCount };
  }

  /** `GetDungeonRooms`: `not_found` for a map the generator did not make. */
  rooms(campaignId: string, mapId: string): Promise<GetDungeonRoomsResponse> {
    return this.client.getDungeonRooms({ campaignId, mapId });
  }

  /** `PlaceDungeonScene`: a hidden scene point, "Sala N", on the middle square of the room. */
  async placeScene(
    campaignId: string,
    mapId: string,
    roomId: number,
    idempotencyKey: string,
  ): Promise<MapPoint> {
    const res = await this.client.placeDungeonScene({ campaignId, mapId, roomId, idempotencyKey });
    if (!res.point) {
      throw new Error('PlaceDungeonScene answered without a point');
    }
    return res.point;
  }

  /** `RedrawDungeonMap`: a new image from the walls and doors the map has now. */
  async redraw(campaignId: string, mapId: string): Promise<MapMessage> {
    const res = await this.client.redrawDungeonMap({ campaignId, mapId });
    if (!res.map) {
      throw new Error('RedrawDungeonMap answered without a map');
    }
    return res.map;
  }
}
