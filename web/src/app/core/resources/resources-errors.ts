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
export function resourceBlockedMessage(
  reason: ResourceBlockedReason,
  needed = 0,
  available = 0,
): string {
  switch (reason) {
    case ResourceBlockedReason.COMBAT_OPEN:
      return 'Há um combate em andamento: termine o combate antes de descansar.';
    case ResourceBlockedReason.NO_HIT_DICE_LEFT:
      return 'Não há dado de vida desse tipo.';
    case ResourceBlockedReason.NOT_ENOUGH_POINTS:
      return `Não há pontos suficientes: precisa de ${needed}, restam ${available}.`;
    case ResourceBlockedReason.SLOT_LEVEL_TOO_HIGH:
      return 'A Conjuração Flexível cria espaços de 1º a 5º nível.';
    case ResourceBlockedReason.NO_FREE_SLOT:
      return 'Você não tem espaço livre desse nível.';
    case ResourceBlockedReason.SORCERY_POINTS_FULL:
      return `Você já tem o máximo de pontos de feitiçaria. O máximo é o seu nível (${needed}): converter um espaço agora perderia os pontos.`;
    case ResourceBlockedReason.SORCERY_POINTS_OVER:
      return `Converter esse espaço passaria do máximo de pontos de feitiçaria (${needed}): você tem ${available}.`;
    case ResourceBlockedReason.NO_USES_LEFT:
      return 'Sem usos da Inspiração de Bardo: eles voltam num descanso longo.';
    case ResourceBlockedReason.TARGET_REFUSED:
      return 'Essa criatura não pode receber o dado.';
    case ResourceBlockedReason.NOT_AVAILABLE:
      return 'Você não tem essa habilidade.';
    case ResourceBlockedReason.INSPIRATION_PENDING:
      return 'Responda primeiro à pergunta da Inspiração de Bardo.';
    default:
      return 'Isso não pode ser feito agora. A tela foi atualizada.';
  }
}

/** A refusal of the class resource flows (Cura pelas Mãos, Conjuração Flexível, Inspiração de Bardo): the rule's
 * (`ResourceBlocked`, with its numbers), the combat's (not the turn, the action or the bonus action used), no open
 * session, and the codes every call shares. `what` finishes "Não deu para ...". */
export function classResourceErrorMessage(err: unknown, what: string): string {
  const blocked = resourceBlocked(err);
  if (blocked) {
    return resourceBlockedMessage(blocked.reason, blocked.needed, blocked.available);
  }
  if (sessionClosed(err)) {
    return 'A sessão acabou: os recursos da classe só valem durante a sessão.';
  }
  return combatErrorMessage(err, what);
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
