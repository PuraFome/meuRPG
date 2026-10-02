import { Component, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { MapPoint } from '../../../gen/meurpg/maps/v1/maps_pb';
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
  readonly toggle = output<PointToggle>();

  protected readonly icon = pointKindIcon;
  protected readonly sub = pointSub;
}
