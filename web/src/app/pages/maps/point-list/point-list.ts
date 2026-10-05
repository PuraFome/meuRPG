import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { type MapPoint, MapPointKind, TrapState } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { TrapTrigger } from '../../../../gen/meurpg/rules/v1/rules_pb';
import { joinDots, tight } from '../../../core/format/text';
import { lightRadii } from '../../../core/maps/light-presets';
import { poText } from '../../../core/traps/treasure-text';
import { trapStateWord } from '../../../core/traps/trap-text';
import { ChestIcon } from '../../../shared/chest-icon/chest-icon';
import { pointKindIcon, pointKindLabel } from '../../../shared/map-view/map-labels';

interface Row {
  readonly point: MapPoint;
  readonly sub: string;
  readonly tags: readonly { readonly icon: string; readonly text: string }[];
}

/** What a row says under the name: the kind and the numbers that matter ("Armadilha · área 2×2 · notar CD 15 · achar CD 15"). */
export function pointSubLine(p: MapPoint, lightName = ''): string {
  switch (p.kind) {
    case MapPointKind.LIGHT:
      return p.light ? joinDots(['Luz', ...(lightName ? [lightName] : []), lightRadii(p.light.brightFt, p.light.dimFt)]) : 'Luz';
    case MapPointKind.TRAP: {
      const t = p.trap;
      return tight(
        joinDots([
          'Armadilha',
          ...(t && t.areaSize > 1 ? [`área ${t.areaSize}×${t.areaSize}`] : []),
          ...(t?.trigger === TrapTrigger.MANUAL ? ['manual'] : []),
          ...(t && t.noticeDc > 0 ? [`notar CD ${t.noticeDc}`] : []),
          ...(t ? [`achar CD ${t.findDc}`] : []),
        ]),
      );
    }
    case MapPointKind.TREASURE:
      return joinDots(['Tesouro', poText(p.treasureValuePo)]);
    default:
      return p.targetMap ? `${pointKindLabel(p.kind)}: ${p.targetMap.name}` : pointKindLabel(p.kind);
  }
}

function tagsOf(p: MapPoint): Row['tags'] {
  const tags: { icon: string; text: string }[] = [];
  if (p.kind === MapPointKind.TRAP) {
    const state = p.trap?.state ?? TrapState.ARMED;
    tags.push({ icon: state === TrapState.DISARMED ? 'check_circle' : 'warning', text: trapStateWord(state) });
  } else if (p.kind === MapPointKind.TREASURE) {
    tags.push(
      p.treasureConverted
        ? { icon: 'lock', text: 'Convertido em XP' }
        : p.treasureFoundAt
          ? { icon: 'check', text: 'Encontrado' }
          : { icon: 'close', text: 'Não encontrado' },
    );
  }
  if (!p.revealed && !p.treasureFoundAt) {
    tags.push({ icon: 'visibility_off', text: 'Escondido' });
  }
  return tags;
}

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
      tags: tagsOf(point),
    })),
  );
}
