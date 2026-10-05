import { type MapPoint, MapPointKind, TrapState } from '../../../gen/meurpg/maps/v1/maps_pb';
import { TrapTrigger } from '../../../gen/meurpg/rules/v1/rules_pb';
import { pointHidden, pointKindLabel } from '../../shared/map-view/map-labels';
import { joinDots, tight } from '../format/text';
import { poText } from '../traps/treasure-text';
import { trapStateWord } from '../traps/trap-text';
import { lightRadii } from './light-presets';

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

export function pointTags(p: MapPoint): readonly { readonly icon: string; readonly text: string }[] {
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
  if (p.kind === MapPointKind.LIGHT) {
    // A Luz is the master's alone: no hidden state, no reveal.
    tags.push({ icon: 'person', text: 'Só você vê' });
  } else if (pointHidden(p)) {
    tags.push({ icon: 'visibility_off', text: 'Escondido' });
  } else if (p.revealed) {
    // Shown to the players by the master: said in words too, as for the plain kinds.
    tags.push({ icon: 'visibility', text: 'Revelado' });
  }
  return tags;
}
