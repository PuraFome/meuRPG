import { Injectable, inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';

import {
  type GetMapResponse,
  type Map as MapMessage,
  MapPointKind,
  MapService,
  type MapPoint,
  type MapToken,
} from '../../../gen/meurpg/maps/v1/maps_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';

/** What a point's editor saves in one call (`UpdateMapPoint`). */
export interface PointChanges {
  readonly kind?: MapPointKind;
  readonly name?: string;
  readonly description?: string;
  readonly xBp?: number;
  readonly yBp?: number;
  /** A SUBMAP point's target; `''` removes it. */
  readonly targetMapId?: string;
  readonly revealed?: boolean;
}

/**
 * Thin wrapper around the generated `MapService` client (MR-008, MR-009,
 * MR-012), in the same shape as `GalleryClient`. `providedIn: 'root'`
 * because the map pages, the campaign page's Mapas panel and the session
 * page all use it, and only lazy code imports this file. Callers map
 * errors (`not_found`, `permission_denied`, `aborted`…) to Portuguese
 * themselves. Tests replace it with `{ provide: MapsClient, useValue }`.
 */
@Injectable({ providedIn: 'root' })
export class MapsClient {
  private readonly client = createClient(MapService, inject(CONNECT_TRANSPORT));

  async list(campaignId: string): Promise<MapMessage[]> {
    return (await this.client.listMaps({ campaignId })).maps;
  }

  get(campaignId: string, mapId: string): Promise<GetMapResponse> {
    return this.client.getMap({ campaignId, mapId });
  }

  async create(campaignId: string, name: string, imageId: string): Promise<MapMessage> {
    const res = await this.client.createMap({ campaignId, name, imageId });
    return need(res.map, 'CreateMap');
  }

  async update(
    campaignId: string,
    mapId: string,
    revision: number,
    changes: { name?: string; imageId?: string },
  ): Promise<MapMessage> {
    const res = await this.client.updateMap({ campaignId, mapId, revision, ...changes });
    return need(res.map, 'UpdateMap');
  }

  async delete(campaignId: string, mapId: string): Promise<void> {
    await this.client.deleteMap({ campaignId, mapId });
  }

  async setRevealed(campaignId: string, mapId: string, revealed: boolean): Promise<MapMessage> {
    const res = await this.client.setMapRevealed({ campaignId, mapId, revealed });
    return need(res.map, 'SetMapRevealed');
  }

  /** `SetMapGrid` (RN-21): squares of 1,5 m across the image's width, 4 to
   * 200; 0 clears the grid. The server answers with the rows it worked out. */
  async setGrid(campaignId: string, mapId: string, columns: number): Promise<MapMessage> {
    const res = await this.client.setMapGrid({ campaignId, mapId, columns });
    return need(res.map, 'SetMapGrid');
  }

  async createPoint(
    campaignId: string,
    mapId: string,
    point: {
      kind: MapPointKind;
      name: string;
      description: string;
      xBp: number;
      yBp: number;
      targetMapId?: string;
    },
  ): Promise<MapPoint> {
    const res = await this.client.createMapPoint({ campaignId, mapId, ...point });
    return need(res.point, 'CreateMapPoint');
  }

  async updatePoint(
    campaignId: string,
    mapId: string,
    pointId: string,
    changes: PointChanges,
  ): Promise<MapPoint> {
    const res = await this.client.updateMapPoint({ campaignId, mapId, pointId, ...changes });
    return need(res.point, 'UpdateMapPoint');
  }

  async deletePoint(campaignId: string, mapId: string, pointId: string): Promise<void> {
    await this.client.deleteMapPoint({ campaignId, mapId, pointId });
  }

  async setPointRevealed(
    campaignId: string,
    mapId: string,
    pointId: string,
    revealed: boolean,
  ): Promise<MapPoint> {
    const res = await this.client.setMapPointRevealed({ campaignId, mapId, pointId, revealed });
    return need(res.point, 'SetMapPointRevealed');
  }

  async placeToken(
    campaignId: string,
    mapId: string,
    characterId: string,
    xBp: number,
    yBp: number,
  ): Promise<MapToken> {
    const res = await this.client.placeMapToken({ campaignId, mapId, characterId, xBp, yBp });
    return need(res.token, 'PlaceMapToken');
  }

  async setTokenHidden(
    campaignId: string,
    mapId: string,
    characterId: string,
    hidden: boolean,
  ): Promise<MapToken> {
    const res = await this.client.setMapTokenHidden({ campaignId, mapId, characterId, hidden });
    return need(res.token, 'SetMapTokenHidden');
  }

  async removeToken(campaignId: string, mapId: string, characterId: string): Promise<void> {
    await this.client.removeMapToken({ campaignId, mapId, characterId });
  }
}

function need<T>(value: T | undefined, call: string): T {
  if (value === undefined) {
    throw new Error(`${call} answered without its result`);
  }
  return value;
}
