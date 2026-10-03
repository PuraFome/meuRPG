import { Injectable, inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';

import { PlayService } from '../../../gen/meurpg/play/v1/play_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';

/** The campaign's open session and the map it is on. */
export interface OpenSessionMap {
  readonly sessionNumber: number;
  /** Empty while the master has not chosen a map. */
  readonly mapId: string;
}

/**
 * Which map the open session is on (`PlayService.GetLiveSession`), for the
 * screens that only add a line about it: "Mapa atual da Sessão 4" in the
 * document's map dialog, and the warning before deleting that map.
 * `providedIn: 'root'`, imported only by lazy code.
 */
@Injectable({ providedIn: 'root' })
export class OpenSessionLookup {
  private readonly play = createClient(PlayService, inject(CONNECT_TRANSPORT));

  /** Null when the campaign has no open session (`failed_precondition`) or
   * the call fails: the line is a detail, never worth an error. */
  async currentMap(campaignId: string): Promise<OpenSessionMap | null> {
    try {
      const res = await this.play.getLiveSession({ campaignId });
      return res.gameSession
        ? { sessionNumber: res.gameSession.sessionNumber, mapId: res.currentMapId }
        : null;
    } catch {
      return null;
    }
  }
}
