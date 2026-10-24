import {
  Component,
  ElementRef,
  Injector,
  OnInit,
  afterNextRender,
  computed,
  inject,
  input,
  signal,
} from '@angular/core';
import { MatIconModule } from '@angular/material/icon';
import { RouterLink } from '@angular/router';

import type { Map as MapMessage } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { describeConnectError } from '../../../core/connect/connect-errors';
import { MapsClient } from '../../../core/maps/maps-client';
import { PanelAction } from '../panel-action/panel-action';
import { PanelHead } from '../panel-head/panel-head';

type PanelState =
  | { status: 'loading' }
  | { status: 'error'; message: string }
  | { status: 'ready'; maps: readonly MapMessage[] };

/** What a row says under the name. The master: "Submapa de Mirathel · 24 × 16 quadrados · 3 pontos" (a main map
 * says no "submapa", and one with no grid says no squares). A player: "Mapa principal" or "Submapa de …". */
export function mapRowSub(map: MapMessage, isMaster: boolean): string {
  const sub = map.parentMaps.length > 0 ? `Submapa de ${map.parentMaps[0].name}` : '';
  if (!isMaster) {
    return sub || 'Mapa principal';
  }
  const parts = [
    sub,
    map.gridColumns > 0 && map.gridRows > 0 ? `${map.gridColumns} × ${map.gridRows} quadrados` : '',
    map.pointCount === 1 ? '1 ponto' : `${map.pointCount} pontos`,
  ];
  return parts.filter((p) => p !== '').join(' · ');
}

/** How many maps show before "Ver todos os N mapas". */
const SHOWN = 5;

/**
 * The campaign page's "Mapas" panel (E5-09, E5-31, MR-008; board C6-02): the count, the
 * master's text actions ("Novo mapa", "Gerar masmorra", "Gerar tesouro") in the title row, and one
 * row per map (the name with "Mapa atual", its size and points, "Revelado" with an eye or
 * "Escondido" with an eye-off, a chevron). Five rows show, then "Ver todos os N mapas". A
 * player sees the maps they may open, and nothing at all while there are none.
 */
@Component({
  selector: 'app-maps-panel',
  imports: [MatIconModule, PanelAction, PanelHead, RouterLink],
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
  protected readonly shown = SHOWN;
  protected readonly expanded = signal(false);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);

  protected readonly maps = computed(() => {
    const s = this.state();
    return s.status === 'ready' ? s.maps : [];
  });
  protected readonly rows = computed(() =>
    this.expanded() ? this.maps() : this.maps().slice(0, SHOWN),
  );
  /** "4 mapas", empty while the list is not here. */
  protected readonly count = computed(() => {
    const n = this.maps().length;
    return this.state().status === 'ready' ? `${n} ${n === 1 ? 'mapa' : 'mapas'}` : '';
  });

  protected showAll(): void {
    this.expanded.set(true);
    // The link goes away: the focus lands on the first map that was not there.
    afterNextRender(
      () => this.host.nativeElement.querySelector<HTMLElement>('.js-first-extra')?.focus(),
      { injector: this.injector },
    );
  }

  ngOnInit(): void {
    this.api.list(this.campaignId()).then(
      (maps) => this.state.set({ status: 'ready', maps }),
      (err: unknown) => this.state.set({ status: 'error', message: describeConnectError(err, {}) }),
    );
  }
}
