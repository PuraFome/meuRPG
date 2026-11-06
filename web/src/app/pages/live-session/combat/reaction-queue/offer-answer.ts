import { Code, ConnectError } from '@connectrpc/connect';

import { combatErrorMessage } from '../../../../core/combat/combat-errors';

/** What the master's offer says when the answer never reached the server: it is still open, so a retry is safe. */
export const CONNECTION_TEXT =
  'Sem conexão. A resposta não foi enviada; a oferta continua esperando você.';

/** Which button of an offer was pressed and is waiting for the server. */
export type Sending = 'use' | 'pass' | null;

/** Why an answer to an offer failed: no connection (retry the same answer) or a refusal (the text says why). */
export interface OfferFailure {
  readonly text: string;
  readonly connection: boolean;
}

/** The failure of an answer: a lost connection keeps the offer open and offers "Tentar de novo"; a refusal says why. */
export function offerFailure(err: unknown, what: string): OfferFailure {
  const code = ConnectError.from(err, Code.Unavailable).code;
  return code === Code.Unavailable || code === Code.DeadlineExceeded
    ? { text: CONNECTION_TEXT, connection: true }
    : { text: combatErrorMessage(err, what), connection: false };
}
