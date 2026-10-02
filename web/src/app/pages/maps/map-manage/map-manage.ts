import { Component, afterNextRender, computed, inject, input, signal } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { MapReveals } from '../../../core/maps/map-reveals';
import { MapState } from '../../../core/maps/map-state';
import { MapsClient } from '../../../core/maps/maps-client';
import { RosterClient } from '../../../core/maps/roster-client';
import { MapPointsList } from '../../../shared/map-lists/map-points-list';
import { MapTokensList } from '../../../shared/map-lists/map-tokens-list';
import { MapLegend } from '../../../shared/map-view/map-legend/map-legend';
import { MapView } from '../../../shared/map-view/map-view';

/**
 * The master's map on a phone (E5-24): the map only pans and zooms (the
 * caption says so), and revealing or hiding happens in the lists below it,
 * "Pontos do mapa" and "Tokens no mapa". Positions are edited on a
 * computer.
 */
@Component({
  selector: 'app-map-manage',
  imports: [MapLegend, MapPointsList, MapTokensList, MapView, MatIconModule],
  templateUrl: './map-manage.html',
  styleUrl: './map-manage.scss',
})
export class MapManage {
  readonly campaignId = input.required<string>();
  readonly state = input.required<MapState>();

  protected readonly reveals = new MapReveals(
    inject(MapsClient),
    () => this.state(),
    () => this.campaignId(),
  );

  /** Class lines and player names for the token rows, best effort. */
  protected readonly info = signal<ReadonlyMap<string, { classSummary: string; playerName: string | null }>>(new Map());

  constructor() {
    const roster = inject(RosterClient);
    afterNextRender(() => {
      roster.list(this.campaignId()).then(
        (list) => this.info.set(new Map(list.map((c) => [c.id, c]))),
        () => undefined,
      );
    });
  }

  protected readonly image = computed(() => {
    const image = this.state().map()?.image;
    return image ? { url: image.url, width: image.width, height: image.height } : null;
  });
}
