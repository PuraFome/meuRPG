import { create } from '@bufbuild/protobuf';

import { CharacterKind } from '../../../gen/meurpg/characters/v1/characters_pb';
import {
  type Map as MapMessage,
  MapPointKind,
  MapSchema,
  type MapPoint,
  MapPointSchema,
  type MapToken,
  MapTokenSchema,
  type GetMapResponse,
} from '../../../gen/meurpg/maps/v1/maps_pb';
import type { PointChanges } from './maps-client';

/**
 * Builders and a stand-in for the map specs (never imported by the app
 * itself, so never bundled): a `Map`, a `MapPoint` and a `MapToken` as the
 * server sends them, and a `MapsClient` that remembers its calls.
 */
export function mapMessage(
  id: string,
  name: string,
  partial: Partial<Omit<MapMessage, '$typeName'>> = {},
): MapMessage {
  return create(MapSchema, {
    id,
    campaignId: 'camp-1',
    name,
    image: {
      id: `img-${id}`,
      url: `/images/img-${id}`,
      thumbnailUrl: `/images/img-${id}/thumb`,
      width: 2400,
      height: 1600,
      name: `Imagem de ${name}`,
    },
    revealed: false,
    revision: 1,
    ...partial,
  });
}

export function mapPoint(
  id: string,
  name: string,
  partial: Partial<Omit<MapPoint, '$typeName'>> = {},
): MapPoint {
  return create(MapPointSchema, {
    id,
    mapId: 'map-1',
    kind: MapPointKind.SCENE,
    name,
    description: '',
    xBp: 5000,
    yBp: 5000,
    revealed: false,
    ...partial,
  });
}

export function mapToken(
  characterId: string,
  name: string,
  partial: Partial<Omit<MapToken, '$typeName'>> = {},
): MapToken {
  return create(MapTokenSchema, {
    mapId: 'map-1',
    characterId,
    name,
    kind: CharacterKind.PLAYER,
    xBp: 4000,
    yBp: 4000,
    hidden: false,
    ...partial,
  });
}

export function mapResponse(
  map: MapMessage,
  points: MapPoint[] = [],
  tokens: MapToken[] = [],
): GetMapResponse {
  return { $typeName: 'meurpg.maps.v1.GetMapResponse', map, points, tokens };
}

/** A `MapsClient` over in-memory maps: every call is recorded in `calls`. */
export class FakeMapsClient {
  maps: MapMessage[] = [];
  responses = new Map<string, GetMapResponse>();
  calls: string[] = [];
  failWith: unknown = null;

  private record(call: string, ...args: unknown[]): void {
    this.calls.push([call, ...args].join(' '));
    if (this.failWith) {
      throw this.failWith;
    }
  }

  async list(_campaignId: string): Promise<MapMessage[]> {
    this.record('list');
    return this.maps;
  }

  async get(_campaignId: string, mapId: string): Promise<GetMapResponse> {
    this.record('get', mapId);
    const res = this.responses.get(mapId);
    if (!res) {
      const { ConnectError, Code } = await import('@connectrpc/connect');
      throw new ConnectError('not found', Code.NotFound);
    }
    return res;
  }

  async create(_c: string, name: string, imageId: string): Promise<MapMessage> {
    this.record('create', name, imageId);
    return mapMessage('new-map', name);
  }

  async setPointRevealed(
    _c: string,
    mapId: string,
    pointId: string,
    revealed: boolean,
  ): Promise<MapPoint> {
    this.record('setPointRevealed', mapId, pointId, revealed);
    return mapPoint(pointId, 'Ponto', { revealed });
  }

  async setTokenHidden(
    _c: string,
    mapId: string,
    characterId: string,
    hidden: boolean,
  ): Promise<MapToken> {
    this.record('setTokenHidden', mapId, characterId, hidden);
    return mapToken(characterId, 'Token', { hidden });
  }

  async placeToken(
    _c: string,
    mapId: string,
    characterId: string,
    xBp: number,
    yBp: number,
  ): Promise<MapToken> {
    this.record('placeToken', mapId, characterId, xBp, yBp);
    return mapToken(characterId, 'Token', { xBp, yBp });
  }

  async updatePoint(
    _c: string,
    mapId: string,
    pointId: string,
    changes: PointChanges,
  ): Promise<MapPoint> {
    this.record('updatePoint', mapId, pointId, JSON.stringify(changes));
    return mapPoint(pointId, changes.name ?? 'Ponto', { ...changes } as never);
  }
}
