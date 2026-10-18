import { Code, ConnectError } from '@connectrpc/connect';

import { freeRollError, rollWaitSeconds } from './free-dice-errors';

function limited(seconds: string): ConnectError {
  return new ConnectError('slow', Code.ResourceExhausted, new Headers({ 'Retry-After': seconds }));
}

describe('freeRollError', () => {
  it('says the wait in seconds when the server turned the roll away for asking too often', () => {
    expect(freeRollError(limited('3'))).toBe('Rolagens demais em pouco tempo. Tente de novo em 3 segundos.');
    expect(freeRollError(limited('1'))).toBe('Rolagens demais em pouco tempo. Tente de novo em 1 segundo.');
    expect(freeRollError(limited('2.2'))).toBe('Rolagens demais em pouco tempo. Tente de novo em 3 segundos.');
  });

  it('gives the wait as a number, and none when the server sent none', () => {
    expect(rollWaitSeconds(limited('4'))).toBe(4);
    expect(rollWaitSeconds(new ConnectError('x', Code.Internal))).toBeNull();
  });

  it('says there is no open session', () => {
    expect(freeRollError(new ConnectError('no session', Code.FailedPrecondition))).toBe(
      'Não há sessão aberta agora.',
    );
  });

  it('says only the master may roll for himself', () => {
    expect(freeRollError(new ConnectError('no', Code.PermissionDenied))).toBe(
      'Só o mestre pode rolar só para si.',
    );
  });

  it('asks to check the numbers on an invalid request', () => {
    expect(freeRollError(new ConnectError('bad', Code.InvalidArgument))).toBe(
      'Esses dados não valem. Confira a quantidade e o modificador.',
    );
  });

  it('falls back to the connection wording', () => {
    expect(freeRollError(new ConnectError('down', Code.Unavailable))).toContain('servidor');
  });
});
