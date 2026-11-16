import { Code, ConnectError } from '@connectrpc/connect';

import {
  CharacterBlockedReason,
  CharacterBlockedSchema,
  type ItemRecharge,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import type { ConsumableKind, TurnBlock } from '../characters/consumables';
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

const KIND_WORDS: Record<ConsumableKind, { use: string; goneUsed: string }> = {
  potion: { use: 'a poção', goneUsed: 'Essa poção já foi usada' },
  scroll: { use: 'o pergaminho', goneUsed: 'Esse pergaminho já foi usado' },
};

/** The line of a turn that does not allow the use, the one the server's refusals say too: "Só na sua vez…", "Você já usou a ação desta vez.". */
export function turnBlockMessage(block: TurnBlock, actingFor: string | null): string {
  return block === 'turn'
    ? `Só na ${actingFor ? `vez de ${actingFor}` : 'sua vez'}: beber ou dar gasta a ação.`
    : `${actingFor ?? 'Você'} já usou a ação desta vez.`;
}

/** The sentence of an item that is no longer on the sheet: the sheet is read again after it. */
function itemGoneMessage(words: { goneUsed: string }): string {
  return `${words.goneUsed}. A ficha foi atualizada.`;
}

/** What a refusal of the combat says: the turn, the action, the receiver out of reach, down or dead, the item gone, and the reasons the combat has sentences for. */
function combatMessage(
  encounter: NonNullable<ReturnType<typeof encounterBlocked>>,
  ctx: ConsumableErrorContext,
  words: { use: string; goneUsed: string },
): string {
  switch (encounter.reason) {
    case EncounterBlockedReason.NOT_YOUR_TURN:
      return turnBlockMessage('turn', ctx.actingFor ?? null);
    case EncounterBlockedReason.ACTION_USED:
      return turnBlockMessage('action', ctx.actingFor ?? null);
    case EncounterBlockedReason.COMBATANT_DOWN:
      return 'Quem está caído não age, mas ainda pode receber uma poção.';
    // A creature at three failed death saves still reads "Caída" in the list of a player, and the server refuses it.
    case EncounterBlockedReason.TARGET_DEAD:
      return `${ctx.targetName ?? 'Quem ia receber'} já morreu: não dá para dar ${words.use} a quem morreu. Nada foi gasto.`;
    case EncounterBlockedReason.TARGET_NOT_ADJACENT:
      return `${ctx.targetName ?? 'Quem ia receber'} não está mais ${ctx.actingFor ? `ao lado de ${ctx.actingFor}` : 'ao seu lado'}. Nada foi gasto.`;
    case EncounterBlockedReason.ITEM_GONE:
      return itemGoneMessage(words);
  }
  return blockedMessage(encounter);
}

/**
 * What a refused or failed use of a consumable says in the "Usar" sheet, by the typed reason and never by the server's
 * message: the turn and the action of a combat, the receiver out of reach and the item gone (worded for this sheet; the
 * generic sentences of the combat stay for the other callers), then the sentences of the combat and of the character for the
 * rest. The refusals that spend nothing and could leave the player in doubt say so: the receiver gone or dead, the character
 * dead or no longer at the table, no open session, dice that do not fit the die and a call that did not get through.
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

/**
 * What a failed read of who may receive the item says, by the typed reason: the item is gone, the character is dead or no
 * longer at the table, and for anything else that the list could not be read. It never speaks of typed dice or of a
 * receiver, which the list read has none of.
 */
export function consumableTargetsErrorMessage(err: unknown, ctx: ConsumableErrorContext): string {
  const words = KIND_WORDS[ctx.kind];
  if (encounterBlocked(err)?.reason === EncounterBlockedReason.ITEM_GONE) {
    return itemGoneMessage(words);
  }
  const connectErr = ConnectError.from(err, Code.Unavailable);
  if (
    connectErr.code === Code.FailedPrecondition &&
    connectErr.findDetails(CharacterBlockedSchema)[0]?.reason ===
      CharacterBlockedReason.CHARACTER_DEAD
  ) {
    return 'Esse personagem está morto: não usa nem recebe itens.';
  }
  if (connectErr.code === Code.NotFound) {
    return 'Esse personagem não está mais na mesa.';
  }
  return `Não deu para ver quem pode receber ${words.use}. Tente de novo.`;
}
