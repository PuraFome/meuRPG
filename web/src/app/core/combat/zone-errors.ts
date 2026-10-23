import { Code, ConnectError } from '@connectrpc/connect';

import { EncounterBlockedReason } from '../../../gen/meurpg/play/v1/combat_pb';
import { metersText } from '../units';
import { combatErrorMessage, encounterBlocked } from './combat-errors';
import { ZONE_MOVE_RANGE_FT } from './zone-move';

const ZONE_GONE = 'Essa zona não existe mais, ou você não a vê. A tela foi atualizada.';

function code(err: unknown): Code {
  return ConnectError.from(err, Code.Unavailable).code;
}

/** The words of a refused `MoveSpellZone`: the range in meters and how far past it, else the other combat refusals. */
export function zoneMoveError(err: unknown): string {
  const blocked = encounterBlocked(err);
  if (blocked?.reason === EncounterBlockedReason.TARGET_OUT_OF_REACH) {
    return `Longe demais: a área anda até ${metersText(ZONE_MOVE_RANGE_FT)} do ponto atual. Faltam ${metersText(blocked.missingFt)}.`;
  }
  if (!blocked && code(err) === Code.NotFound) {
    return ZONE_GONE;
  }
  if (!blocked && code(err) === Code.PermissionDenied) {
    return 'Só quem lançou a magia, na vez dela, pode mover a zona.';
  }
  return combatErrorMessage(err, 'mover a zona');
}

/** The words of a refused `EndSpellZone`. */
export function zoneEndError(err: unknown): string {
  if (code(err) === Code.NotFound) {
    return ZONE_GONE;
  }
  if (code(err) === Code.PermissionDenied) {
    return 'Só o mestre encerra uma zona.';
  }
  return combatErrorMessage(err, 'encerrar a zona');
}
