import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { type MapPoint, MapPointKind } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { pointSubLine, pointTags } from '../../../core/maps/point-text';
import { ChestIcon } from '../../../shared/chest-icon/chest-icon';
import { pointKindIcon } from '../../../shared/map-view/map-labels';

interface Row {
  readonly point: MapPoint;
  readonly sub: string;
  readonly tags: ReturnType<typeof pointTags>;
}

export { pointSubLine };

/**
 * "Pontos do mapa" in the editor's side column (E9-02 1): every point as a row with its glyph, its name, the numbers that
 * matter and its state in words, the way to pick one when two share a square (a trapped chest). A row is a button; the one
 * picked is the one the panel edits. Presentational.
 */
@Component({
  selector: 'app-point-list',
  imports: [ChestIcon, MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './point-list.html',
  styleUrl: './point-list.scss',
})
export class PointList {
  readonly points = input.required<readonly MapPoint[]>();
  readonly selectedId = input<string | null>(null);
  /** The preset names by key, for "Luz · Tocha · 6 m claro + 6 m de penumbra". */
  readonly lightNames = input<ReadonlyMap<string, string>>(new Map());

  readonly pick = output<string>();

  protected readonly Treasure = MapPointKind.TREASURE;
  protected readonly icon = pointKindIcon;
  protected readonly rows = computed<readonly Row[]>(() =>
    this.points().map((point) => ({
      point,
      sub: pointSubLine(point, this.lightNames().get(point.light?.presetKey ?? '') ?? ''),
      tags: pointTags(point),
    })),
  );
}
