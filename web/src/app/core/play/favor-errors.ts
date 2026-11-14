import { Code, ConnectError } from '@connectrpc/connect';

import {
  FavorBlockedReason,
  type FavorBlocked,
  FavorBlockedSchema,
} from '../../../gen/meurpg/play/v1/favor_pb';

/** The typed detail of a `failed_precondition` from a favor call (the Inspiração the master grants), or
 * `null`. Never read from the message. */
export function favorBlocked(err: unknown): FavorBlocked | null {
  const connectErr = ConnectError.from(err, Code.Unavailable);
  if (connectErr.code !== Code.FailedPrecondition) {
    return null;
  }
  return connectErr.findDetails(FavorBlockedSchema)[0] ?? null;
}

/** What each refusal says, in words the master or the player can act on. */
export function favorBlockedMessage(reason: FavorBlockedReason): string {
  switch (reason) {
    case FavorBlockedReason.ALREADY_HAS:
      return 'Essa personagem já tem Inspiração.';
    case FavorBlockedReason.NOT_A_PLAYER_CHARACTER:
      return 'Só personagens de jogadores têm Inspiração.';
    case FavorBlockedReason.NOT_THE_HOLDER:
      return 'Só quem tem a Inspiração pode usá-la.';
    case FavorBlockedReason.HAS_NONE:
      return 'Você não tem Inspiração.';
    case FavorBlockedReason.CANNOT_RECEIVE:
      return 'Essa personagem não pode receber agora';
    case FavorBlockedReason.SELF:
      return 'Você não pode passar a Inspiração a si mesmo.';
    case FavorBlockedReason.ADDS_NOTHING:
      return 'A rolagem já tem vantagem: a Inspiração não mudaria nada.';
    case FavorBlockedReason.AFTER_THE_ROLL:
      return 'O d20 já foi rolado: a Inspiração não pode mais ser usada.';
    default:
      return 'A Inspiração não pode ser usada agora.';
  }
}
