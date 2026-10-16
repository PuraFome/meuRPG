import type { WritableSignal } from '@angular/core';

import { mapErrorMessage } from '../../../core/maps/map-errors';
import { type MapState, tokenKey } from '../../../core/maps/map-state';
import type { MapsClient } from '../../../core/maps/maps-client';
import { MoveSaves } from '../../../core/maps/move-saves';
import type { MapMove } from '../../../shared/map-view/map-view';

/** What the saves of a drag or a key move need from the editor. */
export interface EditorMovesHost {
  readonly api: Pick<MapsClient, 'updatePoint' | 'placeToken'>;
  state(): MapState;
  campaignId(): string;
  mapId(): string;
  readonly message: WritableSignal<string>;
}

/**
 * The moves of points and tokens: each shows at once, then goes to the server one at a time per item (see `MoveSaves`); a refusal
 * puts the item back where the server last had it and says so.
 */
export class EditorMoves {
  private readonly saves = new MoveSaves();

  constructor(private readonly host: EditorMovesHost) {}

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
}
