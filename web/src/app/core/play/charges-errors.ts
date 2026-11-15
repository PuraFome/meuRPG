import { Code, ConnectError } from '@connectrpc/connect';

import {
  ChargesBlockedReason,
  type ChargesBlocked,
  ChargesBlockedSchema,
} from '../../../gen/meurpg/play/v1/charges_pb';

/** The typed detail of a `failed_precondition` from a charges or ammunition call, or `null`. Never read from the message. */
export function chargesBlocked(err: unknown): ChargesBlocked | null {
  const connectErr = ConnectError.from(err, Code.Unavailable);
  if (connectErr.code !== Code.FailedPrecondition) {
    return null;
  }
  return connectErr.findDetails(ChargesBlockedSchema)[0] ?? null;
}

/** What each refusal says, in words the master or the player can act on. */
export function chargesBlockedMessage(blocked: ChargesBlocked): string {
  switch (blocked.reason) {
    case ChargesBlockedReason.REGAIN_FACE_NEEDED:
      return `Informe a face do d${blocked.needed} que conta as cargas devolvidas.`;
    case ChargesBlockedReason.COMBAT_RUNNING:
      return 'O combate ainda não acabou: recolha a munição depois dele.';
    case ChargesBlockedReason.ALREADY_RECOVERED:
      return 'A munição desse combate já foi recolhida.';
    default:
      return 'As cargas ou a munição não podem ser usadas agora.';
  }
}
