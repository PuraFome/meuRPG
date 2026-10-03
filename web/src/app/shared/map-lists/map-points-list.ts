import { Component, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import { MapPointKind, type MapPoint } from '../../../gen/meurpg/maps/v1/maps_pb';
import { pointKindIcon, pointKindLabel } from '../map-view/map-labels';

export interface PointToggle {
  readonly point: MapPoint;
  /** The state the master asked for. */
  readonly revealed: boolean;
}

/** "Submapa: Torre de Mirathel", or just "Batalha". */
export function pointSub(point: MapPoint): string {
  const kind = pointKindLabel(point.kind);
  return point.targetMap ? `${kind}: ${point.targetMap.name}` : kind;
}

/**
 * "Pontos do mapa" (E5-04, E5-06, E5-24): every point of the map with its
 * state in words ("Revelado" with an eye, "Escondido" with an eye-off) and
 * an outlined "Revelar aos jogadores" or "Esconder", the non-pointer way to
 * what the map shows (MR-009). The master only: a player never gets the
 * hidden rows. The button's accessible name carries the point's name.
 *
 * In the live session (`openScenePointId` set, `null` when no scene is open) a
 * SCENE point also says what it does with the open scene (MR-015, question 53):
 * "Abrir cena", "Trocar para esta cena" when another one is open, or "Cena
 * aberta agora" when it is this one. A scene with no actions says so and cannot
 * open.
 */
@Component({
  selector: 'app-map-points-list',
  imports: [MatButtonModule, MatIconModule],
  templateUrl: './map-points-list.html',
  styleUrl: './map-lists.scss',
})
export class MapPointsList {
  readonly points = input.required<readonly MapPoint[]>();
  /** The point whose call is in flight: its button waits. */
  readonly pendingId = input<string | null>(null);
  /** The heading's level: 3 inside the map's panel, 2 as a panel of its own. */
  readonly headingLevel = input<2 | 3>(3);
  /** The session's open scene point: `null` for none; left out (`undefined`)
   * where there is no session, and then no row offers "Abrir cena". */
  readonly openScenePointId = input<string | null | undefined>(undefined);
  /** The scene point whose "Abrir cena" call is in flight. */
  readonly pendingSceneId = input<string | null>(null);
  readonly toggle = output<PointToggle>();
  /** "Abrir cena" / "Trocar para esta cena" on a scene point. */
  readonly openScene = output<MapPoint>();

  protected readonly Scene = MapPointKind.SCENE;

  protected readonly icon = pointKindIcon;
  protected readonly sub = pointSub;
}
