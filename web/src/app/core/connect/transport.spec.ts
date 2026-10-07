import { Code, ConnectError } from '@connectrpc/connect';
import type { UnaryRequest, UnaryResponse } from '@connectrpc/connect';

import { isRateLimited } from './connect-errors';
import { rateLimitInterceptor } from './transport';

describe('rateLimitInterceptor', () => {
  const call = (fail: unknown) =>
    rateLimitInterceptor(async () => {
      throw fail;
    })({} as UnaryRequest) as Promise<UnaryResponse>;

  it('turns a rate-limited answer into unavailable, so no screen shows a "limit reached" wording', async () => {
    const limited = new ConnectError(
      'too many requests',
      Code.ResourceExhausted,
      new Headers({ 'Retry-After': '4' }),
    );
    const err = (await call(limited).catch((e: unknown) => e)) as ConnectError;
    expect(err.code).toBe(Code.Unavailable);
    expect(err.rawMessage).toContain('Espere 4 segundos');
    expect(isRateLimited(err)).toBe(true);
  });

  it('leaves every other failure as it is', async () => {
    const full = new ConnectError('gallery full', Code.ResourceExhausted);
    await expect(call(full)).rejects.toBe(full);
    const network = new TypeError('Failed to fetch');
    await expect(call(network)).rejects.toBe(network);
  });
});
