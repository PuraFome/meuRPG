import { Component, computed, inject, input, output, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { RouterLink } from '@angular/router';
import { Code, ConnectError } from '@connectrpc/connect';

import type { Map as MapMessage, MapPoint } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { describeConnectError } from '../../../core/connect/connect-errors';
import { mapErrorMessage } from '../../../core/maps/map-errors';
import { MapReveals } from '../../../core/maps/map-reveals';
import { MapState } from '../../../core/maps/map-state';
import { MapsClient } from '../../../core/maps/maps-client';
import { MoveSaves } from '../../../core/maps/move-saves';
import { SceneClient } from '../../../core/play/scene-client';
import { sceneErrorMessage } from '../../../core/play/scene-errors';
import type { SceneState } from '../../../core/play/scene-state';
import { MapPointsList } from '../../../shared/map-lists/map-points-list';
import { MapLegend } from '../../../shared/map-view/map-legend/map-legend';
import { MapMove, MapView } from '../../../shared/map-view/map-view';
import { PHONE_QUERY, mediaQuery } from '../../../shared/map-view/media-query';
import { LiveSessionSource } from '../live-session.types';

/**
 * The map half of the session page (MR-012, E5-02 to E5-06): the session's
 * current map, the same for everyone at the table.
 *
 * - **Player:** a still preview (zoomed in on the party on a phone, the
 *   whole map on a computer) with only the revealed points and the visible
 *   tokens; the whole preview and "Ver mapa" open the full map. Without a
 *   map, "O mestre ainda não escolheu um mapa." The page reads the map
 *   again on `map_changed`; if the server answers `not_found`, the player
 *   lost sight of it and gets the empty state again.
 * - **Master:** the "Mapa atual" select (`SetCurrentMap`, which also
 *   reveals a hidden map), the map with hidden things dashed, tokens you
 *   drag (`PlaceMapToken`) on a computer, "Abrir mapa" for the editor, and
 *   "Pontos do mapa" with "Revelar aos jogadores" and "Esconder".
 */
@Component({
  selector: 'app-session-map',
  imports: [
    MapLegend,
    MapPointsList,
    MapView,
    MatButtonModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    RouterLink,
  ],
  templateUrl: './session-map.html',
  styleUrl: './session-map.scss',
})
export class SessionMap {
  private readonly api = inject(MapsClient);
  /** Saves each token's moves one at a time (see `MoveSaves`). */
  private readonly moves = new MoveSaves();
  private readonly source = inject(LiveSessionSource);
  private readonly sceneApi = inject(SceneClient);

  readonly campaignId = input.required<string>();
  readonly state = input.required<MapState>();
  /** The session's current map, or `null` while there is none. */
  readonly mapId = input<string | null>(null);
  /** The master's copy speaks to the one who chooses the map. */
  readonly isMaster = input(false);
  /** The campaign's maps, for the master's select. */
  readonly maps = input<readonly MapMessage[]>([]);
  /** The session's RP scene, for "Abrir cena" on a scene point (master). */
  readonly scene = input<SceneState | null>(null);
  /** The master chose another map (or none): the page shows it. */
  readonly currentChanged = output<string | null>();

  protected readonly phone = mediaQuery(PHONE_QUERY);
  protected readonly error = signal<string | null>(null);
  protected readonly busy = signal(false);
  protected readonly sceneError = signal<string | null>(null);
  protected readonly reveals = new MapReveals(
    inject(MapsClient),
    () => this.state(),
    () => this.campaignId(),
  );

  protected readonly map = computed(() => this.state().map());
  protected readonly status = computed(() => this.state().status());
  protected readonly image = computed(() => {
    const image = this.map()?.image;
    return image ? { url: image.url, width: image.width, height: image.height } : null;
  });
  protected readonly openHiddenNote = computed(() => {
    const chosen = this.maps().find((m) => m.id === this.mapId());
    return chosen !== undefined && !chosen.revealed;
  });

  protected readonly pendingScene = signal<string | null>(null);

  /** "Abrir cena" on a scene point of the map: the same call as the picker's, so the
   * open scene appears (and takes focus) at once. A refusal is said under the list. */
  protected async openScene(point: MapPoint): Promise<void> {
    const state = this.scene();
    if (!state || this.pendingScene() !== null) {
      return;
    }
    this.pendingScene.set(point.id);
    this.sceneError.set(null);
    try {
      state.openedHere(await this.sceneApi.open(this.campaignId(), point.id));
    } catch (err) {
      this.sceneError.set(sceneErrorMessage(err, 'abrir a cena'));
    } finally {
      this.pendingScene.set(null);
    }
  }

  protected async choose(select: HTMLSelectElement): Promise<void> {
    const mapId = select.value === '' ? null : select.value;
    this.busy.set(true);
    this.error.set(null);
    try {
      const current = await this.source.setCurrentMap(this.campaignId(), mapId);
      this.currentChanged.emit(current);
    } catch (err) {
      select.value = this.mapId() ?? '';
      this.error.set(
        ConnectError.from(err).code === Code.FailedPrecondition
          ? 'A sessão acabou: o mapa só muda durante a sessão.'
          : describeConnectError(err, {
              [Code.NotFound]: 'Esse mapa não existe mais. Recarregue a página.',
              [Code.PermissionDenied]: 'Só o mestre da campanha escolhe o mapa.',
            }),
      );
    } finally {
      this.busy.set(false);
    }
  }

  protected async onMoved(move: MapMove): Promise<void> {
    const state = this.state();
    const mapId = state.map()?.id;
    const before = state.tokens().find((t) => t.characterId === move.id);
    if (move.kind !== 'token' || !mapId || !before) {
      return;
    }
    state.upsertToken({ ...before, xBp: move.xBp, yBp: move.yBp });
    await this.moves.move(
      `${mapId}/${move.id}`,
      before,
      { xBp: move.xBp, yBp: move.yBp },
      {
        save: (to) => this.api.placeToken(this.campaignId(), mapId, move.id, to.xBp, to.yBp),
        failed: (saved, err) => {
          const now = state.tokens().find((t) => t.characterId === move.id);
          if (now) {
            state.upsertToken({ ...now, xBp: saved.xBp, yBp: saved.yBp });
          }
          this.error.set(mapErrorMessage(err, 'mover o token'));
        },
      },
    );
  }

  protected retry(): void {
    void this.state().refresh();
  }
}
