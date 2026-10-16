import type { WritableSignal } from '@angular/core';

import type { MapToken } from '../../../../gen/meurpg/maps/v1/maps_pb';
import type { MapLayers } from '../../../core/maps/layers';
import { mapErrorMessage } from '../../../core/maps/map-errors';
import type { MapState } from '../../../core/maps/map-state';
import type { MapsClient } from '../../../core/maps/maps-client';
import type { RosterEntry } from '../../../core/maps/roster-client';
import { tokenSpot } from '../../../core/maps/token-spot';
import type { MapSelection } from '../../../shared/map-view/map-view';

/** The middle of the map when no view is there to say what is on screen. */
const MIDDLE_BP = { xBp: 5000, yBp: 5000 };

/** What the token calls need from the editor. */
export interface EditorTokensHost {
  readonly api: Pick<MapsClient, 'placeToken' | 'setTokenHidden' | 'removeToken'>;
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

/** The token calls of the master: add a character, hide or show the chosen token, take it off the map. */
export class EditorTokens {
  constructor(private readonly host: EditorTokensHost) {}

  async add(entry: RosterEntry): Promise<void> {
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

  async toggleHidden(): Promise<void> {
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

  async remove(): Promise<void> {
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
