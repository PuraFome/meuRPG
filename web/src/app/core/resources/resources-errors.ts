import { Code, ConnectError } from '@connectrpc/connect';

import {
  type ResourceBlocked,
  ResourceBlockedReason,
  ResourceBlockedSchema,
} from '../../../gen/meurpg/play/v1/resources_pb';
import { combatErrorMessage, sessionClosed } from '../combat/combat-errors';
import { describeConnectError } from '../connect/connect-errors';

/** The typed detail of a `failed_precondition` from `ResourceService`, or `null` (another code, or another detail).
 * Never read from the message. */
export function resourceBlocked(err: unknown): ResourceBlocked | null {
  const connectErr = ConnectError.from(err, Code.Unavailable);
  if (connectErr.code !== Code.FailedPrecondition) {
    return null;
  }
  return connectErr.findDetails(ResourceBlockedSchema)[0] ?? null;
}

/** What a refusal that comes from a rule says. Only the reasons of the rests and the hit dice are worded here: the
 * resource flows that come with their own dialogs word theirs. */
export function resourceBlockedMessage(reason: ResourceBlockedReason): string {
  switch (reason) {
    case ResourceBlockedReason.COMBAT_OPEN:
      return 'Há um combate em andamento: termine o combate antes de descansar.';
    case ResourceBlockedReason.NO_HIT_DICE_LEFT:
      return 'Não há dado de vida desse tipo.';
    default:
      return 'Isso não pode ser feito agora. A tela foi atualizada.';
  }
}

/** The master's rest: the combat in the way, no open session, and the codes every call shares. */
export function restErrorMessage(err: unknown): string {
  const blocked = resourceBlocked(err);
  if (blocked) {
    return resourceBlockedMessage(blocked.reason);
  }
  if (sessionClosed(err)) {
    return 'A sessão acabou: o descanso só vale durante a sessão.';
  }
  return describeConnectError(err, {
    [Code.InvalidArgument]:
      'Não deu para descansar: os dados de vida escolhidos não batem com a ficha. Feche e tente de novo.',
    [Code.PermissionDenied]: 'Só o mestre da campanha faz o grupo descansar.',
    [Code.NotFound]: 'Essa campanha não existe mais, ou você não participa dela.',
    [Code.Unavailable]: 'Não deu para descansar: o servidor não respondeu. Tente de novo.',
  });
}

/** A hit die spent: the combat in the way ("Há um combate em andamento."), no die of that size, the wrong way of
 * rolling (the combat's wording), no open session. */
export function hitDiceErrorMessage(err: unknown): string {
  const blocked = resourceBlocked(err);
  if (blocked) {
    return blocked.reason === ResourceBlockedReason.COMBAT_OPEN
      ? 'Há um combate em andamento.'
      : resourceBlockedMessage(blocked.reason);
  }
  if (sessionClosed(err)) {
    return 'A sessão acabou: os dados de vida só se gastam durante a sessão.';
  }
  return combatErrorMessage(err, 'gastar o dado de vida');
}
