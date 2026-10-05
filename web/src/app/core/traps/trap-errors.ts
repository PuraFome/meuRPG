import { Code, ConnectError } from '@connectrpc/connect';

import { EncounterBlockedReason } from '../../../gen/meurpg/play/v1/combat_pb';
import { MapBlockedReason, MapBlockedSchema } from '../../../gen/meurpg/maps/v1/maps_pb';
import { combatErrorMessage, encounterBlocked, sessionClosed } from '../combat/combat-errors';
import { describeConnectError } from '../connect/connect-errors';

/** The reason of a trap call that failed with `EncounterBlocked`, or `null`. */
export function trapBlockedReason(err: unknown): EncounterBlockedReason | null {
  return encounterBlocked(err)?.reason ?? null;
}

/** Whether the search needs a second die (Perception with disadvantage, a typed roll). */
export function needsTwoDice(err: unknown): boolean {
  return trapBlockedReason(err) === EncounterBlockedReason.SEARCH_NEEDS_TWO_DICE;
}

/** The Portuguese message for a failed trap call of `PlayService`, by code and typed detail (never by
 * message). `what` finishes "Não deu para …". */
export function trapErrorMessage(err: unknown, what = 'fazer isso'): string {
  switch (trapBlockedReason(err)) {
    case EncounterBlockedReason.TRAP_NOT_ON_MAP:
      return 'O seu personagem não está no mapa. Peça ao mestre para colocar o seu token.';
    case EncounterBlockedReason.TRAP_NOT_ARMED:
      return 'Essa armadilha já disparou ou foi desarmada. A tela foi atualizada.';
    case EncounterBlockedReason.TRAP_SEARCH_NOT_NOW:
      return 'Agora não dá para procurar: o combate não está na vez de ninguém.';
    case EncounterBlockedReason.SEARCH_NEEDS_TWO_DICE:
      return 'Há penumbra por perto: digite também o segundo dado.';
    case EncounterBlockedReason.NOT_YOUR_TURN:
      return 'Procurar numa luta só na sua vez.';
    case EncounterBlockedReason.ACTION_USED:
      return 'Você já usou a sua ação neste turno.';
    default:
  }
  if (sessionClosed(err)) {
    return 'A sessão acabou: as armadilhas só mudam durante a sessão.';
  }
  const blocked = encounterBlocked(err);
  if (blocked) {
    return combatErrorMessage(err, what);
  }
  return describeConnectError(err, {
    [Code.InvalidArgument]: `Não deu para ${what}: confira os campos e tente de novo.`,
    [Code.NotFound]: 'Essa armadilha, ou um desses personagens, não existe mais. Feche e tente de novo.',
    [Code.PermissionDenied]: 'Só o mestre da campanha faz isso.',
    [Code.FailedPrecondition]: 'A sessão mudou enquanto você agia. A tela foi atualizada; tente de novo.',
    [Code.Unavailable]: `Não deu para ${what}: o servidor não respondeu. Tente de novo.`,
  });
}

/** The Portuguese message for a failed trap or treasure call of `MapService`. */
export function trapMapErrorMessage(err: unknown, what = 'fazer isso'): string {
  const connectErr = ConnectError.from(err, Code.Unavailable);
  if (connectErr.code === Code.FailedPrecondition) {
    const blocked = connectErr.findDetails(MapBlockedSchema)[0];
    if (blocked?.reason === MapBlockedReason.TREASURE_CONVERTED) {
      return 'Esse tesouro já virou XP. Para desmarcar, desfaça esse XP na página da campanha.';
    }
  }
  return describeConnectError(err, {
    [Code.InvalidArgument]: `Não deu para ${what}: marque pelo menos um personagem e tente de novo.`,
    [Code.NotFound]: 'Esse ponto, ou um desses personagens, não existe mais. Feche e tente de novo.',
    [Code.PermissionDenied]: 'Só o mestre da campanha faz isso.',
    [Code.Unavailable]: `Não deu para ${what}: o servidor não respondeu. Tente de novo.`,
  });
}
