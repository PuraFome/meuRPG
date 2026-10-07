import { Code, ConnectError } from '@connectrpc/connect';

import { describeConnectError, isRateLimited, rateLimitedMessage } from './connect-errors';

describe('describeConnectError', () => {
  it('uses the message given for the error code', () => {
    const err = new ConnectError('nope', Code.NotFound);
    expect(describeConnectError(err, { [Code.NotFound]: 'Campanha não encontrada.' })).toBe(
      'Campanha não encontrada.',
    );
  });

  it('falls back to the unavailable message for a code with no specific wording', () => {
    const err = new ConnectError('nope', Code.Internal);
    expect(
      describeConnectError(err, {
        [Code.Unavailable]: 'Servidor indisponível.',
      }),
    ).toBe('Servidor indisponível.');
  });

  it('falls back to a generic message when nothing else applies', () => {
    const err = new ConnectError('nope', Code.Internal);
    expect(describeConnectError(err, {})).toBe(
      'Não foi possível falar com o servidor agora. Tente de novo em instantes.',
    );
  });

  it('treats a plain network failure the same as "unavailable" (never as some other code)', () => {
    const err = new TypeError('Failed to fetch');
    expect(
      describeConnectError(err, {
        [Code.Unavailable]: 'Servidor indisponível.',
        [Code.NotFound]: 'Não encontrado.',
      }),
    ).toBe('Servidor indisponível.');
  });

  it('leaves an already-specific ConnectError as is (no re-wrapping)', () => {
    const err = new ConnectError('sessão inválida', Code.PermissionDenied);
    expect(
      describeConnectError(err, { [Code.PermissionDenied]: 'Só o mestre pode fazer isso.' }),
    ).toBe('Só o mestre pode fazer isso.');
  });
});

describe('a rate-limited call', () => {
  const limited = (code: Code, retryAfter = '3') =>
    new ConnectError('slow down', code, new Headers({ 'Retry-After': retryAfter }));

  it('is told apart from the other resource_exhausted answers by Retry-After', () => {
    expect(isRateLimited(limited(Code.ResourceExhausted))).toBe(true);
    expect(isRateLimited(limited(Code.Unavailable))).toBe(true);
    expect(isRateLimited(new ConnectError('gallery full', Code.ResourceExhausted))).toBe(false);
    expect(isRateLimited(new TypeError('Failed to fetch'))).toBe(false);
  });

  it('says how long to wait, and never uses the screen wording of the code', () => {
    expect(rateLimitedMessage(limited(Code.ResourceExhausted, '3'))).toBe(
      'Muitas ações em pouco tempo. Espere 3 segundos e tente de novo.',
    );
    expect(rateLimitedMessage(limited(Code.ResourceExhausted, '1'))).toContain(
      'Espere 1 segundo e',
    );
    expect(rateLimitedMessage(limited(Code.ResourceExhausted, 'soon'))).toContain(
      'Espere alguns segundos',
    );
    expect(
      describeConnectError(limited(Code.ResourceExhausted), {
        [Code.ResourceExhausted]: 'A galeria está cheia.',
      }),
    ).toBe('Muitas ações em pouco tempo. Espere 3 segundos e tente de novo.');
  });
});
