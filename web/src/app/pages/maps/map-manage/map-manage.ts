import { Component, afterNextRender, computed, effect, inject, input, signal, untracked, viewChild } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { PaintedLayers } from '../../../core/maps/paint-layers';
import type { Map as MapMessage } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { MapReveals } from '../../../core/maps/map-reveals';
import { MapState } from '../../../core/maps/map-state';
import { MapsClient } from '../../../core/maps/maps-client';
import { mapTokenInitial } from '../../../core/maps/token-initial';
import { RosterClient } from '../../../core/maps/roster-client';
import { LightPresets } from '../../../core/maps/light-presets';
import { CharacterKind } from '../../../../gen/meurpg/characters/v1/characters_pb';
import { trapMapErrorMessage } from '../../../core/traps/trap-errors';
import type { PickRow } from '../../../shared/person-pick/person-pick';
import { type TreasureMark, MapPointsList } from '../../../shared/map-lists/map-points-list';
import { MapTokensList } from '../../../shared/map-lists/map-tokens-list';
import { MapLayersLegend } from '../../../shared/map-layers/map-layers-legend';
import { MapPinsLegend } from '../../../shared/map-pins/map-pins-legend';
import { MapLegend } from '../../../shared/map-view/map-legend/map-legend';
import { MapView } from '../../../shared/map-view/map-view';
import { EditorOverlay } from '../editor-overlay/editor-overlay';
import { FogPanel } from '../fog-panel/fog-panel';

/**
 * The master's map on a phone (E5-24, E9-01 7): the map only pans and zooms (the
 * caption says so), and revealing or hiding happens in the lists below it,
 * "Pontos do mapa" and "Tokens no mapa". Positions are edited on a
 * computer, and so is the painting: a fixed notice says so, the layers are drawn
 * with their legend, and the fog settings ("Névoa de guerra") are editable here.
 */
@Component({
  selector: 'app-map-manage',
  imports: [EditorOverlay, FogPanel, MapLayersLegend, MapLegend, MapPinsLegend, MapPointsList, MapTokensList, MapView, MatIconModule],
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

  /** The painted layers, read-only here. */
  protected readonly painted = new PaintedLayers();
  private readonly mapsApi = inject(MapsClient);
  protected readonly map = computed(() => this.state().map());
  protected readonly initialOf = mapTokenInitial;
  private readonly lights = inject(LightPresets);
  private readonly list = viewChild(MapPointsList);
  protected readonly lightNames = signal<ReadonlyMap<string, string>>(new Map());
  protected readonly markBusy = signal(false);
  protected readonly markError = signal('');
  /** Who found a treasure: the living player characters, with their class and player. */
  protected readonly finders = signal<readonly PickRow[]>([]);
  /** Which painted light levels the map has: the legend names only those. */
  protected readonly lightLevels = computed(() => {
    const l = this.painted.layers().light;
    return { bright: (l?.bright.length ?? 0) > 0, dim: (l?.dim.length ?? 0) > 0, dark: (l?.dark.length ?? 0) > 0 };
  });

  constructor() {
    const roster = inject(RosterClient);
    effect(() => {
      const map = this.map();
      if (!map) {
        return;
      }
      // Read again when the grid or the layers change (a new grid clears them).
      void map.layersRevision;
      untracked(() => {
        if (map.gridColumns <= 0) {
          this.painted.clear();
          return;
        }
        void this.mapsApi.layers(this.campaignId(), map.id).then(
          (packed) => this.painted.load(packed),
          () => undefined,
        );
      });
    });
    afterNextRender(() => {
      roster.list(this.campaignId()).then(
        (list) => {
          this.info.set(new Map(list.map((c) => [c.id, c])));
          this.finders.set(
            list
              .filter((c) => c.kind === CharacterKind.PLAYER && c.playerUserId !== '')
              .map((c) => ({ id: c.id, name: c.name, sub: [c.classSummary, c.playerName].filter(Boolean).join(' · ') })),
          );
        },
        () => undefined,
      );
      this.lights.list(this.campaignId()).then(
        (list) => this.lightNames.set(new Map(list.map((o) => [o.key, o.name]))),
        () => undefined,
      );
    });
  }

  /** "Marcar como encontrado": saved at once, as the editor's panel and the session's card do. */
  protected async markFound(mark: TreasureMark): Promise<void> {
    if (this.markBusy()) {
      return;
    }
    this.markBusy.set(true);
    this.markError.set('');
    try {
      const point = await this.mapsApi.markTreasureFound(this.campaignId(), mark.point.mapId, mark.point.id, [...mark.characterIds]);
      this.state().upsertPoint(point);
      this.list()?.closeMarking();
    } catch (err) {
      this.markError.set(trapMapErrorMessage(err, 'marcar o tesouro'));
    } finally {
      this.markBusy.set(false);
    }
  }

  protected onMapChanged(map: MapMessage): void {
    this.state().setMap(map);
  }

  protected readonly image = computed(() => {
    const image = this.state().map()?.image;
    return image ? { url: image.url, width: image.width, height: image.height } : null;
  });
}
