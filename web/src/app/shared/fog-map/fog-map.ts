import { Component, computed, input, output, signal, viewChild } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { CharacterKind } from '../../../gen/meurpg/characters/v1/characters_pb';
import type { MapPoint, MapToken } from '../../../gen/meurpg/maps/v1/maps_pb';
import { combatantInitial } from '../../core/combat/combat-view';
import type { MapLayers } from '../../core/maps/layers';
import { hasLayers, NO_LAYERS } from '../../core/maps/layers';
import { tokenInitial, ViewToken } from '../map-view/map-geometry';
import { type FogStatus } from '../../core/maps/fog-view';
import { type Vision, seenCount, tileProgress, tileRects, visionLegend } from '../../core/maps/vision';
import { CombatantToken } from '../combatant-token/combatant-token';
import { MapLayersLegend } from '../map-layers/map-layers-legend';
import { MapView } from '../map-view/map-view';
import { PHONE_QUERY, mediaQuery } from '../map-view/media-query';
import { FogBase, type FogImage } from './fog-base';

/** How a viewer is addressed in the off-map notice: "Seu personagem", or, for the master, the character's name. */
export interface FogViewer {
  readonly name: string;
  readonly own: boolean;
}

/**
 * The map of the session (and of the map page) when the fog of war is on, as one
 * viewer sees it (MR-036, RN-10; E9-03): the tiles, the layers and the shading of
 * `app-fog-base` inside the map view's pan and zoom, the party's tokens always,
 * the NPCs and creatures as the server's reads give them, a legend under the map
 * that names every state it draws, and the notices: "Carregando o mapa" while the
 * tiles come, "Seu personagem não está neste mapa" when the viewer has no token.
 *
 * The same piece is the master's "Ver como" (`as`: the tiles of that player's
 * view) and the wolf's view of Wild Shape (9.17); it decides nothing: the server
 * sent each square's state and each token it may show.
 *
 * - **Phone:** the party's chips above the map (44 px targets) take the view to a
 *   character's token; two fingers zoom, and "−", "+" and "ajustar" do the same.
 * - **Loading:** a place whose tile has not arrived is grey stripes with a dashed
 *   border, and a token over it is left out until it arrives (never tokens on a
 *   blank grid). The notice is read once (`role="status"`) and says "parte N de M".
 *   Nothing moves, so reduced motion changes nothing.
 */
@Component({
  selector: 'app-fog-map',
  imports: [CombatantToken, FogBase, MapLayersLegend, MapView, MatIconModule],
  templateUrl: './fog-map.html',
  styleUrl: './fog-map.scss',
})
export class FogMap {
  readonly mapName = input('');
  /** The map's picture size, to reserve the frame (a player gets no image, only its size). */
  readonly imageWidth = input.required<number>();
  readonly imageHeight = input.required<number>();
  readonly status = input<FogStatus>('ready');
  readonly vision = input<Vision | null>(null);
  readonly layers = input<MapLayers | null>(null);
  readonly tokens = input<readonly MapToken[]>([]);
  readonly points = input<readonly MapPoint[]>([]);
  /** The master reading as this character ("Ver como"): the tiles' URLs say so. */
  readonly forCharacter = input<string | null>(null);
  /** The whole picture for a viewer that reads it whole; a player's map has none. */
  readonly image = input<FogImage | null>(null);
  /** Who sees: names the character in the off-map notice. */
  readonly viewer = input<FogViewer | null>(null);
  /** A click on a point opens it (the map page). */
  readonly selectablePoints = input(false);
  /** A label over the map's corner: "Vendo como Toren (Caio)" for the master's "Ver como". */
  readonly badge = input<string | null>(null);
  /** The caption "Você vê ..." under the legend. */
  readonly caption = input(true);
  /** The notice is above the map; a screen that has its own banner (the master's "Ver como") leaves it. */
  readonly notices = input(true);

  readonly pointSelect = output<string>();

  protected readonly phone = mediaQuery(PHONE_QUERY);
  protected readonly view = viewChild(MapView);
  private readonly settled = signal<ReadonlySet<string>>(new Set());

  protected readonly frame = computed<FogImage>(() => ({ url: '', width: this.imageWidth(), height: this.imageHeight() }));
  protected readonly noLayers = NO_LAYERS;
  protected readonly layerSet = computed(() => this.layers() ?? NO_LAYERS);

  protected readonly rects = computed(() => {
    const v = this.vision();
    return v ? tileRects(v) : [];
  });
  protected readonly progress = computed(() => tileProgress(this.rects(), this.settled()));
  protected readonly loading = computed(
    () => this.status() === 'loading' || (this.vision() !== null && this.progress().done < this.progress().total),
  );
  protected readonly part = computed(() => Math.min(this.progress().total, this.progress().done + 1));

  /** The tokens the map draws: a token never sits over a place whose tile is still on its way. */
  protected readonly shownTokens = computed(() => {
    const v = this.vision();
    if (!v) {
      return [];
    }
    if (v.tilesPath === '') {
      return this.tokens();
    }
    const size = v.tileSquares;
    return this.tokens().filter((t) => {
      const col = Math.min(v.columns - 1, Math.floor((t.xBp / 10000) * v.columns));
      const row = Math.min(v.rows - 1, Math.floor((t.yBp / 10000) * v.rows));
      const key = `${Math.floor(col / size)}:${Math.floor(row / size)}`;
      // A square with no tile is black and has no wait: its token shows.
      return this.settled().has(key) || !this.rects().some((r) => r.key === key);
    });
  });

  protected readonly party = computed(() => this.tokens().filter((t) => isParty(t) && !t.creatureId));
  protected readonly enemies = computed(() => this.shownTokens().filter((t) => isNpc(t)));
  protected readonly creatures = computed(() => this.shownTokens().filter((t) => !!t.creatureId));
  protected readonly mine = computed(() => this.tokens().find((t) => t.mine && !t.creatureId) ?? null);
  protected readonly companions = computed(() => this.party().filter((t) => !t.mine));
  protected readonly offMap = computed(() => {
    const v = this.vision();
    return v !== null && v.fogEnabled && !v.characterOnMap;
  });
  protected readonly legend = computed(() => {
    const v = this.vision();
    return v ? visionLegend(v) : null;
  });
  protected readonly hasLayerMarks = computed(() => hasLayers(this.layerSet()));

  protected readonly summary = computed<readonly string[]>(() => {
    const v = this.vision();
    if (!v || this.loading() || this.offMap()) {
      return [];
    }
    const seen = seenCount(v);
    const names = this.enemies().map((t) => t.name);
    return [
      `${seen} de ${v.columns * v.rows} quadrados à vista.`,
      names.length > 0 ? `Inimigos à vista: ${names.join(', ')}.` : 'Nenhum inimigo à vista.',
    ];
  });

  protected readonly offMapTitle = computed(() => {
    const viewer = this.viewer();
    return viewer ? `${viewer.name} fora do mapa` : 'Personagem fora do mapa';
  });
  protected readonly offMapText = computed(() =>
    this.viewer()?.own === false
      ? 'O personagem não está neste mapa: o jogador vê só o que já tinha visto.'
      : 'Seu personagem não está neste mapa. Você vê só o que já tinha visto.',
  );

  protected readonly initialOf = (token: ViewToken, all: readonly ViewToken[]): string =>
    isNpc(token) ? combatantInitial(token.name) : tokenInitial(token, all);

  protected initial(t: MapToken): string {
    return this.initialOf(t, this.tokens());
  }

  protected onSettled(set: ReadonlySet<string>): void {
    this.settled.set(set);
  }

  protected goTo(t: MapToken): void {
    this.view()?.focusOn({ xBp: t.xBp, yBp: t.yBp });
  }

  zoomIn(): void {
    this.view()?.zoomIn();
  }

  zoomOut(): void {
    this.view()?.zoomOut();
  }

  fit(): void {
    this.view()?.fit();
  }
}

/** A player's character (a creature of it too): the party, which the fog never hides. */
function isParty(t: { kind: number; creatureId: string }): boolean {
  return t.kind === CharacterKind.PLAYER || t.creatureId !== '';
}

function isNpc(t: { kind?: number; creatureId?: string }): boolean {
  return !t.creatureId && t.kind !== undefined && t.kind !== CharacterKind.PLAYER && t.kind !== CharacterKind.UNSPECIFIED;
}
