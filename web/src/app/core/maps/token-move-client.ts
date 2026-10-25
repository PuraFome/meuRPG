import { Injectable, inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';

import {
  type MapFormation,
  type Map as MapMessage,
  MapService,
  type MapToken,
} from '../../../gen/meurpg/maps/v1/maps_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';
import type { MoveOutcome } from './token-move';

/**
 * The calls that move tokens outside combat (B2, maps.proto): `MoveMapTokens`, `PutPartyOnMap` and `SetMapPlayersMove`,
 * apart from `MapsClient` so that file stays within its size. `providedIn: 'root'`; tests replace it with
 * `{ provide: TokenMoveClient, useValue }` (`FakeTokenMoveClient`).
 */
@Injectable({ providedIn: 'root' })
export class TokenMoveClient {
  private readonly client = createClient(MapService, inject(CONNECT_TRANSPORT));

  /** `MoveMapTokens` (B2): each token walks toward the square that holds (xBp, yBp) and stops before a wall, a locked door or a
   * square marked "Não dá para passar"; several move together around the first, the anchor. */
  async moveTokens(
    campaignId: string,
    mapId: string,
    characterIds: readonly string[],
    xBp: number,
    yBp: number,
  ): Promise<MoveOutcome> {
    const res = await this.client.moveMapTokens({
      campaignId,
      mapId,
      characterIds: [...characterIds],
      xBp,
      yBp,
    });
    return {
      moved: res.moved,
      blocked: res.blockedCharacterId,
      results: res.moves.map((m) => ({
        characterId: m.token?.characterId ?? '',
        name: m.token?.name ?? '',
        xBp: m.token?.xBp ?? 0,
        yBp: m.token?.yBp ?? 0,
        asked: m.squaresAsked,
        moved: m.squaresMoved,
        stopped: m.stoppedBy,
      })),
    };
  }

  /** `PutPartyOnMap` (B2): the characters in a formation from a square; the tokens as they are now. */
  async putParty(
    campaignId: string,
    mapId: string,
    characterIds: readonly string[],
    formation: MapFormation,
    xBp: number,
    yBp: number,
  ): Promise<{ tokens: readonly MapToken[]; movedForRoom: number }> {
    const res = await this.client.putPartyOnMap({
      campaignId,
      mapId,
      characterIds: [...characterIds],
      formation,
      xBp,
      yBp,
    });
    return { tokens: res.tokens, movedForRoom: res.movedForRoom };
  }

  /** `SetMapPlayersMove`: "Os jogadores movem os próprios personagens" on this map. The map as it is now. */
  async setPlayersMove(campaignId: string, mapId: string, enabled: boolean): Promise<MapMessage> {
    const res = await this.client.setMapPlayersMove({ campaignId, mapId, enabled });
    if (!res.map) {
      throw new Error('SetMapPlayersMove answered no map');
    }
    return res.map;
  }
}
