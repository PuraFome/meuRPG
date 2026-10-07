import { Injectable, inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';
import { timestampDate } from '@bufbuild/protobuf/wkt';

import { GameSession, PlayService } from '../../../../gen/meurpg/play/v1/play_pb';
import { CONNECT_TRANSPORT } from '../../../core/connect/transport';
import { GameSessionSource, GameSessionVm, StartGameSessionResultVm } from './game-session-card.types';

function toVm(gameSession: GameSession): GameSessionVm {
  return {
    id: gameSession.id,
    sessionNumber: gameSession.sessionNumber,
    startedAt: gameSession.startedAt ? timestampDate(gameSession.startedAt) : new Date(0),
    endedAt: gameSession.endedAt ? timestampDate(gameSession.endedAt) : null,
  };
}

/**
 * `GameSessionSource` over the generated `PlayService` client
 * (`meurpg.play.v1`, phase 2). Provided at the route level for
 * `/campaigns/:id` — see `../campaign-detail.routes.ts` — so this client
 * stays out of the eager bundle.
 */
@Injectable()
export class GameSessionSourceLive implements GameSessionSource {
  private readonly client = createClient(PlayService, inject(CONNECT_TRANSPORT));

  async getCurrentSession(campaignId: string): Promise<GameSessionVm | null> {
    const res = await this.client.listGameSessions({ campaignId });
    const open = res.gameSessions.find((gs) => !gs.endedAt);
    return open ? toVm(open) : null;
  }

  async startGameSession(campaignId: string, idempotencyKey: string): Promise<StartGameSessionResultVm> {
    const res = await this.client.startGameSession({ campaignId, idempotencyKey });
    return { session: toVm(res.gameSession!), lockedSheetCount: res.lockedSheetCount };
  }

  async endGameSession(campaignId: string, gameSessionId: string): Promise<GameSessionVm> {
    const res = await this.client.endGameSession({ campaignId, gameSessionId });
    return toVm(res.gameSession!);
  }
}
