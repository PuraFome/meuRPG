import { Code, ConnectError } from '@connectrpc/connect';

import { CharacterBlockedReason } from '../../../gen/meurpg/characters/v1/characters_pb';
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

/** What the character's refusals of a charges call say (CharacterBlocked, characters.proto), or `null` for another reason. */
export function itemBlockedMessage(reason: CharacterBlockedReason): string | null {
  switch (reason) {
    case CharacterBlockedReason.CHARGES_NOT_ENOUGH:
      return 'O item não tem tantas cargas.';
    case CharacterBlockedReason.ITEM_DESTROYED:
      return 'O item foi destruído.';
    case CharacterBlockedReason.ITEM_NO_CHARGES:
      return 'O item não tem cargas.';
    default:
      return null;
  }
}
