import { Code, ConnectError } from '@connectrpc/connect';

import {
  CharacterBlockedReason,
  CharacterBlockedSchema,
  type ItemRecharge,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { noChargesWithRecharge } from '../characters/recharge-text';
import { articleOf } from '../characters/charges';
import { describeCharacterError } from '../characters/character-errors';
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

/** The item an error is about: the words of the refusal name it and say how many charges it holds. */
export interface ChargesErrorItem {
  readonly itemName: string;
  readonly left: number;
  /** The item's recharge rules, when the sheet has them: "Sem cargas" then says when the item comes back. */
  readonly recharge?: ItemRecharge;
}

const NOT_GOT_THROUGH = new Set<Code>([
  Code.Unavailable,
  Code.Unknown,
  Code.Internal,
  Code.DeadlineExceeded,
]);

/** The call did not reach the server or was cut off: nothing changed, so there is nothing to read again. */
export function chargesDidNotGetThrough(err: unknown): boolean {
  return NOT_GOT_THROUGH.has(ConnectError.from(err, Code.Unavailable).code);
}

/**
 * What a refused or failed charges call says on the sheet, by the typed reason and never by the server's message: the
 * `ChargesBlocked` sentences, then the item's own refusals (not enough charges, none, destroyed) with the item named,
 * "Nada mudou" for a call that did not get through (`doing` says what the call was: "gastar a carga", "salvar", "amanhecer"), and the character sentences for the rest.
 */
export function chargesErrorMessage(
  err: unknown,
  item: ChargesErrorItem,
  doing = 'gastar a carga',
): string {
  const blocked = chargesBlocked(err);
  if (blocked) {
    return chargesBlockedMessage(blocked);
  }
  const connectErr = ConnectError.from(err, Code.Unavailable);
  if (connectErr.code === Code.FailedPrecondition) {
    const reason = connectErr.findDetails(CharacterBlockedSchema)[0]?.reason;
    switch (reason) {
      case CharacterBlockedReason.CHARGES_NOT_ENOUGH:
        return `Cargas a menos. ${articleOf(item.itemName)} ${item.itemName} só tem ${item.left} ${item.left === 1 ? 'carga' : 'cargas'}.`;
      case CharacterBlockedReason.ITEM_NO_CHARGES:
        return item.recharge
          ? noChargesWithRecharge(item.itemName, item.recharge)
          : `Sem cargas. ${articleOf(item.itemName)} ${item.itemName} não tem cargas agora.`;
      case CharacterBlockedReason.ITEM_DESTROYED:
        return `Item destruído. ${articleOf(item.itemName)} ${item.itemName} não tem mais uso.`;
    }
  }
  if (chargesDidNotGetThrough(err)) {
    return `Não foi possível ${doing}. Nada mudou. Tente de novo.`;
  }
  return describeCharacterError(err);
}
