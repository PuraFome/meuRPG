import { Code, ConnectError } from '@connectrpc/connect';

import {
  CharacterBlockedReason,
  CharacterBlockedSchema,
  type ItemRecharge,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import type { ConsumableKind } from '../characters/consumables';
import { blockedMessage, encounterBlocked, sessionClosed } from '../combat/combat-errors';
import { EncounterBlockedReason } from '../../../gen/meurpg/play/v1/combat_pb';
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

/** Who and what a refused use of a consumable is about, for the words of its refusals. */
export interface ConsumableErrorContext {
  readonly kind: ConsumableKind;
  /** The receiver the call named, when it was a gift: the refusals that are about them say their name. */
  readonly targetName?: string;
  /** The character the master acts for: "Só na vez de Tor". Absent for the player's own use. */
  readonly actingFor?: string;
}

const KIND_WORDS: Record<ConsumableKind, { use: string }> = {
  potion: { use: 'a poção' },
  scroll: { use: 'o pergaminho' },
};

/** What a refusal of the combat says: the turn, the action, the receiver down or dead, and the reasons the combat has sentences for. */
function combatMessage(
  encounter: NonNullable<ReturnType<typeof encounterBlocked>>,
  ctx: ConsumableErrorContext,
  words: { use: string },
): string {
  switch (encounter.reason) {
    case EncounterBlockedReason.NOT_YOUR_TURN:
      return `Só na ${ctx.actingFor ? `vez de ${ctx.actingFor}` : 'sua vez'}: beber ou dar gasta a ação.`;
    case EncounterBlockedReason.ACTION_USED:
      return `${ctx.actingFor ?? 'Você'} já usou a ação desta vez.`;
    case EncounterBlockedReason.COMBATANT_DOWN:
      return 'Quem está caído não age, mas ainda pode receber uma poção.';
    // A creature at three failed death saves still reads "Caída" in the list of a player, and the server refuses it.
    case EncounterBlockedReason.TARGET_DEAD:
      return `${ctx.targetName ?? 'Quem ia receber'} já morreu: não dá para dar ${words.use} a quem morreu. Nada foi gasto.`;
  }
  return blockedMessage(encounter);
}

/**
 * What a refused or failed use of a consumable says in the "Usar" sheet, by the typed reason and never by the server's
 * message: the turn and the action of a combat, then
 * the sentences of the combat (the receiver out of reach and the item gone among them) and of the character for the rest. Every refusal says that nothing was spent.
 */
export function consumableErrorMessage(err: unknown, ctx: ConsumableErrorContext): string {
  const words = KIND_WORDS[ctx.kind];
  const encounter = encounterBlocked(err);
  if (encounter) {
    return combatMessage(encounter, ctx, words);
  }
  if (sessionClosed(err)) {
    return 'Sem sessão aberta: só dá para usar durante a sessão. Nada foi gasto.';
  }
  const connectErr = ConnectError.from(err, Code.Unavailable);
  if (connectErr.code === Code.FailedPrecondition) {
    const reason = connectErr.findDetails(CharacterBlockedSchema)[0]?.reason;
    if (reason === CharacterBlockedReason.CHARACTER_DEAD) {
      return 'Esse personagem está morto: não usa nem recebe itens. Nada foi gasto.';
    }
  }
  switch (connectErr.code) {
    case Code.InvalidArgument:
      return 'Os dados digitados não valem: cada um vai de 1 até o número de faces dele. Nada foi gasto.';
    case Code.NotFound:
      return `${ctx.targetName ?? 'Esse personagem'} não está mais na mesa. Nada foi gasto.`;
  }
  if (chargesDidNotGetThrough(err)) {
    return `Não deu para usar ${words.use}. Nada foi gasto. Tente de novo.`;
  }
  return describeCharacterError(err);
}
