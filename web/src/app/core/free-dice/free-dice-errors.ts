import { Code, ConnectError } from '@connectrpc/connect';

import { describeConnectError, isRateLimited } from '../connect/connect-errors';

/** The wait the server asked for (`Retry-After`), in whole seconds; `null` when it sent none. */
export function rollWaitSeconds(err: unknown): number | null {
  const secs = Number(ConnectError.from(err, Code.Unavailable).metadata.get('Retry-After'));
  return Number.isFinite(secs) && secs >= 1 ? Math.ceil(secs) : null;
}

/** What the sheet says under "Não deu para rolar." for a roll the server turned away or could not make. */
export function freeRollError(err: unknown): string {
  if (isRateLimited(err)) {
    const wait = rollWaitSeconds(err);
    const after =
      wait === null ? 'em alguns segundos' : `em ${wait} ${wait === 1 ? 'segundo' : 'segundos'}`;
    return `Rolagens demais em pouco tempo. Tente de novo ${after}.`;
  }
  return describeConnectError(err, {
    [Code.FailedPrecondition]: 'Não há sessão aberta agora.',
    [Code.PermissionDenied]: 'Só o mestre pode rolar só para si.',
    [Code.InvalidArgument]: 'Esses dados não valem. Confira a quantidade e o modificador.',
    [Code.NotFound]: 'Você não está nesta campanha.',
  });
}
