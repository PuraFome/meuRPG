import type { WritableSignal } from '@angular/core';

import type { MapToken } from '../../../../gen/meurpg/maps/v1/maps_pb';
import type { MapLayers } from '../../../core/maps/layers';
import { mapErrorMessage } from '../../../core/maps/map-errors';
import { type MapState, tokenKey } from '../../../core/maps/map-state';
import type { MapsClient } from '../../../core/maps/maps-client';
import { MoveSaves } from '../../../core/maps/move-saves';
import type { RosterEntry } from '../../../core/maps/roster-client';
import { tokenSpot } from '../../../core/maps/token-spot';
import type { MapMove, MapSelection } from '../../../shared/map-view/map-view';

/** The middle of the map when no view is there to say what is on screen. */
const MIDDLE_BP = { xBp: 5000, yBp: 5000 };

/** What the moves of points and tokens, and the token calls, need from the editor. */
export interface EditorItemsHost {
  readonly api: Pick<MapsClient, 'updatePoint' | 'placeToken' | 'setTokenHidden' | 'removeToken'>;
  state(): MapState;
  campaignId(): string;
  mapId(): string;
  selectedToken(): MapToken | null;
  /** The middle of what is on screen, in basis points; `undefined` before the map is drawn. */
  centerBp(): { xBp: number; yBp: number } | undefined;
  layers(): MapLayers;
  readonly selection: WritableSignal<MapSelection | null>;
  readonly message: WritableSignal<string>;
}

/**
 * The items on the map. A move of a point or a token shows at once, then goes to the server one at a time per item (see
 * `MoveSaves`); a refusal puts the item back where the server last had it and says so. The token calls of the master: add a
 * character, hide or show the chosen token, take it off the map.
 */
export class EditorItems {
  private readonly saves = new MoveSaves();

  constructor(private readonly host: EditorItemsHost) {}

  /** Whether a move of this point is still on its way: what the screen shows for it is ahead of the server. */
  pendingPoint(pointId: string): boolean {
    return this.saves.isPending(`${this.host.mapId()}/point/${pointId}`);
  }

  async move(move: MapMove): Promise<void> {
    if (!this.host.mapId()) {
      return;
    }
    await (move.kind === 'point' ? this.movePoint(move) : this.moveToken(move));
  }

  private async movePoint(move: MapMove): Promise<void> {
    const { api, message } = this.host;
    const state = this.host.state();
    const mapId = this.host.mapId();
    const before = state.points().find((p) => p.id === move.id);
    if (!before) {
      return;
    }
    state.upsertPoint({ ...before, xBp: move.xBp, yBp: move.yBp });
    await this.saves.move(
      `${mapId}/point/${move.id}`,
      before,
      { xBp: move.xBp, yBp: move.yBp },
      {
        save: (to) =>
          api.updatePoint(this.host.campaignId(), mapId, move.id, { xBp: to.xBp, yBp: to.yBp }),
        failed: (saved, err) => {
          const now = state.points().find((p) => p.id === move.id);
          if (now) {
            state.upsertPoint({ ...now, xBp: saved.xBp, yBp: saved.yBp });
          }
          message.set(mapErrorMessage(err, 'mover o ponto'));
        },
      },
    );
  }

  private async moveToken(move: MapMove): Promise<void> {
    const { api, message } = this.host;
    const state = this.host.state();
    const mapId = this.host.mapId();
    const before = state.tokens().find((t) => tokenKey(t) === move.id);
    if (!before) {
      return;
    }
    state.upsertToken({ ...before, xBp: move.xBp, yBp: move.yBp });
    await this.saves.move(
      `${mapId}/token/${move.id}`,
      before,
      { xBp: move.xBp, yBp: move.yBp },
      {
        save: (to) =>
          before.creatureId
            ? api.placeToken(this.host.campaignId(), mapId, '', to.xBp, to.yBp, before.creatureId)
            : api.placeToken(this.host.campaignId(), mapId, move.id, to.xBp, to.yBp),
        failed: (saved, err) => {
          const now = state.tokens().find((t) => tokenKey(t) === move.id);
          if (now) {
            state.upsertToken({ ...now, xBp: saved.xBp, yBp: saved.yBp });
          }
          message.set(mapErrorMessage(err, 'mover o token'));
        },
      },
    );
  }

  async addToken(entry: RosterEntry): Promise<void> {
    const { host } = this;
    const mapId = host.mapId();
    if (!mapId) {
      return;
    }
    // The nearest free floor square to the middle of the screen: tokens added one after the other never share a square.
    const at = tokenSpot(host.centerBp() ?? MIDDLE_BP, host.layers(), host.state().tokens());
    try {
      const token = await host.api.placeToken(host.campaignId(), mapId, entry.id, at.xBp, at.yBp);
      host.state().upsertToken(token);
      host.selection.set({ kind: 'token', id: token.characterId });
      host.message.set(`${entry.name} está no mapa.`);
    } catch (err) {
      host.message.set(mapErrorMessage(err, 'pôr o token'));
    }
  }

  async toggleTokenHidden(): Promise<void> {
    const { host } = this;
    const token = host.selectedToken();
    const mapId = host.mapId();
    if (!token || !mapId) {
      return;
    }
    try {
      const saved = await host.api.setTokenHidden(
        host.campaignId(),
        mapId,
        token.characterId,
        !token.hidden,
      );
      host.state().upsertToken(saved);
    } catch (err) {
      host.message.set(mapErrorMessage(err, 'mudar o token'));
    }
  }

  async removeToken(): Promise<void> {
    const { host } = this;
    const token = host.selectedToken();
    const mapId = host.mapId();
    if (!token || !mapId) {
      return;
    }
    try {
      await host.api.removeToken(host.campaignId(), mapId, token.characterId, token.creatureId);
      if (token.creatureId) {
        host.state().removeCreatureToken(token.creatureId);
      } else {
        host.state().removeToken(token.characterId);
      }
      host.selection.set(null);
      host.message.set(`${token.name} saiu do mapa.`);
    } catch (err) {
      host.message.set(mapErrorMessage(err, 'tirar o token'));
    }
  }
}
