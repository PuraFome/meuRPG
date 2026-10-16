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

/** Builders for the map specs (never imported by the app itself, so never bundled): a `Map`, a `MapPoint` and a `MapToken` as the server sends them. */
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
