import {
  type MapFormation,
  type Map as MapMessage,
  MapMoveStop,
  type MapToken,
} from '../../../gen/meurpg/maps/v1/maps_pb';
import { mapMessage, mapToken } from './map-builders-testing';
import type { MoveOutcome } from './token-move';

/** A `TokenMoveClient` in memory: every call is recorded in `calls`; `failWith` makes the next ones throw. */
export class FakeTokenMoveClient {
  calls: string[] = [];
  failWith: unknown = null;

  private record(call: string, ...args: unknown[]): void {
    this.calls.push([call, ...args].join(' '));
    if (this.failWith) {
      throw this.failWith;
    }
  }

  /** What `moveTokens` answers; by default every token gets there (`asked` = `moved` = 1). */
  moveOutcome: MoveOutcome | null = null;

  async moveTokens(
    _c: string,
    mapId: string,
    characterIds: readonly string[],
    xBp: number,
    yBp: number,
  ): Promise<MoveOutcome> {
    this.record('moveTokens', mapId, characterIds.join(','), xBp, yBp);
    return (
      this.moveOutcome ?? {
        moved: true,
        blocked: '',
        results: characterIds.map((id) => ({
          characterId: id,
          name: id,
          xBp,
          yBp,
          asked: 1,
          moved: 1,
          stopped: MapMoveStop.UNSPECIFIED,
        })),
      }
    );
  }

  /** The tokens `putParty` answers; by default one per character at the asked point. */
  partyTokens: MapToken[] | null = null;

  async putParty(
    _c: string,
    mapId: string,
    characterIds: readonly string[],
    formation: MapFormation,
    xBp: number,
    yBp: number,
  ): Promise<{ tokens: readonly MapToken[]; movedForRoom: number }> {
    this.record('putParty', mapId, characterIds.join(','), formation, xBp, yBp);
    return {
      tokens: this.partyTokens ?? characterIds.map((id) => mapToken(id, id, { mapId, xBp, yBp })),
      movedForRoom: 0,
    };
  }

  async setPlayersMove(_c: string, mapId: string, enabled: boolean): Promise<MapMessage> {
    this.record('setPlayersMove', mapId, enabled);
    return mapMessage(mapId, 'Mapa', { gridColumns: 24, gridRows: 16, playersMoveTokens: enabled });
  }
}
