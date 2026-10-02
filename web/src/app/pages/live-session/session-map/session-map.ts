import { Component, computed, inject, input, output, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { RouterLink } from '@angular/router';
import { Code, ConnectError } from '@connectrpc/connect';

import type { Map as MapMessage } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { describeConnectError } from '../../../core/connect/connect-errors';
import { mapErrorMessage } from '../../../core/maps/map-errors';
import { MapReveals } from '../../../core/maps/map-reveals';
import { MapState } from '../../../core/maps/map-state';
import { MapsClient } from '../../../core/maps/maps-client';
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
  private readonly source = inject(LiveSessionSource);

  readonly campaignId = input.required<string>();
  readonly state = input.required<MapState>();
  /** The session's current map, or `null` while there is none. */
  readonly mapId = input<string | null>(null);
  /** The master's copy speaks to the one who chooses the map. */
  readonly isMaster = input(false);
  /** The campaign's maps, for the master's select. */
  readonly maps = input<readonly MapMessage[]>([]);
  /** The master chose another map (or none): the page shows it. */
  readonly currentChanged = output<string | null>();

  protected readonly phone = mediaQuery(PHONE_QUERY);
  protected readonly error = signal<string | null>(null);
  protected readonly busy = signal(false);
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
    try {
      await this.api.placeToken(this.campaignId(), mapId, move.id, move.xBp, move.yBp);
    } catch (err) {
      state.upsertToken(before);
      this.error.set(mapErrorMessage(err, 'mover o token'));
    }
  }

  protected retry(): void {
    void this.state().refresh();
  }
}
