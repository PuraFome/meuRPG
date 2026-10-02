import { Component, OnInit, computed, inject, input, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { RouterLink } from '@angular/router';

import type { Map as MapMessage } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { describeConnectError } from '../../../core/connect/connect-errors';
import { MapsClient } from '../../../core/maps/maps-client';

type PanelState =
  | { status: 'loading' }
  | { status: 'error'; message: string }
  | { status: 'ready'; maps: readonly MapMessage[] };

/** "Mapa principal, 5 pontos" / "Submapa de Mirathel e arredores, 2 pontos". */
export function mapRowSub(map: MapMessage, isMaster: boolean): string {
  const where =
    map.parentMaps.length > 0 ? `Submapa de ${map.parentMaps[0].name}` : 'Mapa principal';
  if (!isMaster) {
    return where;
  }
  const count = map.pointCount === 1 ? '1 ponto' : `${map.pointCount} pontos`;
  return `${where}, ${count}`;
}

/**
 * The campaign page's "Mapas" panel (E5-09, E5-31, MR-008): one row per
 * map with its tags ("Mapa atual" with a pin, "Revelado" with an eye,
 * "Escondido" dashed with an eye-off) and, for the master, "Novo mapa". A
 * player sees the maps they may open, and nothing at all while there are
 * none.
 */
@Component({
  selector: 'app-maps-panel',
  imports: [MatButtonModule, MatIconModule, RouterLink],
  templateUrl: './maps-panel.html',
  styleUrl: './maps-panel.scss',
})
export class MapsPanel implements OnInit {
  private readonly api = inject(MapsClient);

  readonly campaignId = input.required<string>();
  readonly isMaster = input(false);

  protected readonly state = signal<PanelState>({ status: 'loading' });
  protected readonly visible = computed(() => {
    const s = this.state();
    return this.isMaster() || (s.status === 'ready' && s.maps.length > 0);
  });
  protected readonly sub = mapRowSub;

  ngOnInit(): void {
    this.api.list(this.campaignId()).then(
      (maps) => this.state.set({ status: 'ready', maps }),
      (err: unknown) => this.state.set({ status: 'error', message: describeConnectError(err, {}) }),
    );
  }
}
