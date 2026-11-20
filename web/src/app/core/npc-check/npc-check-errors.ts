import { Code, ConnectError } from '@connectrpc/connect';

import { ResourceBlockedReason } from '../../../gen/meurpg/play/v1/resources_pb';
import { sessionClosed } from '../combat/combat-errors';
import { resourceBlocked } from '../resources/resources-errors';
import { ofName } from './npc-check-picker';

/** A refusal of the master's roll, in the sheet's notice: a bold lead and the sentence after it. */
export interface RollError {
  readonly lead: string;
  readonly text: string;
  /** A combat is open: the entry "Teste de um NPC" stays in place and says so. */
  readonly combat: boolean;
}

/** A refusal of the bard's answer. */
export interface CutError {
  readonly lead: string;
  readonly text: string;
  /** The question is over (answered or gone): the sheet has nothing left to try and only closes. */
  readonly closed: boolean;
}

/** What `RollNpcCheck` refused, worded so that nothing looks rolled: a combat in the way, no open session, or any failure. */
export function rollErrorOf(err: unknown, creature: string): RollError {
  if (resourceBlocked(err)?.reason === ResourceBlockedReason.COMBAT_OPEN) {
    return {
      lead: 'Um combate começou.',
      text: `O teste ${ofName(creature)} não foi rolado: os testes agora são do combate.`,
      combat: true,
    };
  }
  if (sessionClosed(err)) {
    return { lead: 'Só numa sessão aberta.', text: 'Nada foi rolado.', combat: false };
  }
  return {
    lead: 'Não deu para rolar agora.',
    text: 'Nada foi rolado. Tente de novo.',
    combat: false,
  };
}

/** "Continuar sem esperar" that did not get through. */
export function skipErrorText(): string {
  return 'Não deu para continuar sem esperar agora. Tente de novo.';
}

/** Said beside the entry while a combat is open. */
export const COMBAT_OPEN_TEXT = 'Há um combate aberto: os testes das criaturas são do combate.';
/** The picker that could not be read. */
export const PICKER_ERROR_TEXT = 'Não deu para ler as criaturas.';

/** What the bard's answer that did not get through says. Nothing is spent in any of them. */
export function cutErrorOf(err: unknown, cut: boolean): CutError {
  const connectErr = ConnectError.from(err, Code.Unavailable);
  const blocked = resourceBlocked(err);
  if (blocked && cut) {
    return {
      lead: 'Não dá para cortar agora.',
      text: 'Nada foi gasto. Você ainda pode deixar passar.',
      closed: false,
    };
  }
  // not_found, or no open session: the question is gone (closed, passed on by the server, or the bard's player left the scene).
  if (connectErr.code === Code.NotFound || sessionClosed(err)) {
    return { lead: 'A pergunta já foi encerrada.', text: 'Nada foi gasto.', closed: true };
  }
  // failed_precondition: this bard answered it already.
  if (connectErr.code === Code.FailedPrecondition && !blocked) {
    return { lead: 'Essa pergunta já foi respondida.', text: 'Nada foi gasto.', closed: true };
  }
  return cut
    ? { lead: 'Não deu para cortar agora.', text: 'Nada foi gasto. Tente de novo.', closed: false }
    : { lead: 'Não deu para deixar passar agora.', text: 'Tente de novo.', closed: false };
}
