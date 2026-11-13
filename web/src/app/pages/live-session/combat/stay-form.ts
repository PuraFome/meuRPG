import type { Combatant } from '../../../../gen/meurpg/play/v1/combat_pb';
import type { VitalsVm } from '../../../core/play/live-session.types';
import { joinDots, tight } from '../../../core/format/text';
import { metersFixed } from '../../../core/units';

/** The time each druid's form has left, by character (the server sends the time only to the master and the owner). */
export function formTimesOf(vitals: readonly VitalsVm[]): Map<string, string> {
  return new Map(
    vitals
      .filter((v) => v.wildShape?.timeLeftPt)
      .map((v) => [v.characterId, v.wildShape!.timeLeftPt!] as const),
  );
}

/**
 * "Permanecer na forma" (the Druida tab, after "Voltar"): what it costs and why it cannot be pressed ('' when it can). `null`
 * outside a form. `ended` is the server's "A forma já acabou": nothing was spent, and the row stays off.
 */
export function stayView(
  own: Combatant | null | undefined,
  vitals: VitalsVm | null | undefined,
  ended: boolean,
): { readonly detail: string; readonly reason: string } | null {
  if (!own?.wildShapeBeastKey) {
    return null;
  }
  const resource = vitals?.resources?.find((r) => r.key === 'wild_shape');
  const left = resource ? resource.total - resource.used : null;
  const cost =
    resource && left !== null
      ? `gasta 1 uso (restam ${left} de ${resource.total}) e o tempo recomeça`
      : 'gasta 1 uso e o tempo recomeça';
  let reason = '';
  if (ended) {
    reason = 'A forma já acabou. Para virar fera de novo, use Forma Selvagem.';
  } else if (left !== null && left <= 0) {
    reason = 'Sem usos de Forma Selvagem · voltam no descanso curto ou longo';
  }
  return {
    detail: joinDots(['Sem magias', tight(metersFixed(own.speedDft / 10)), cost]),
    reason,
  };
}
