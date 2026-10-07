import type { Transport } from '@connectrpc/connect';
import { describe, expect, it, vi } from 'vitest';

import { UNARY_DEADLINE_MS, withUnaryDeadline } from './transport';

/** A transport that only records the timeout each call was given. */
function recorder() {
  const unary = vi.fn().mockResolvedValue({});
  const stream = vi.fn().mockResolvedValue({});
  return { unary, stream, transport: { unary, stream } as unknown as Transport };
}

// The method, signal, headers and input do not matter here: the wrapper only
// touches the third argument, the timeout.
const method = {} as never;

describe('withUnaryDeadline', () => {
  it('gives a unary call that has no deadline the default one', async () => {
    const { unary, transport } = recorder();
    await withUnaryDeadline(transport, UNARY_DEADLINE_MS).unary(method, undefined, undefined, undefined, {});
    expect(unary.mock.calls[0][2]).toBe(UNARY_DEADLINE_MS);
  });

  it('keeps the deadline a call set for itself', async () => {
    const { unary, transport } = recorder();
    await withUnaryDeadline(transport, UNARY_DEADLINE_MS).unary(method, undefined, 5_000, undefined, {});
    expect(unary.mock.calls[0][2]).toBe(5_000);
  });

  it('never puts a deadline on a stream, which is meant to stay open', async () => {
    const { stream, transport } = recorder();
    await withUnaryDeadline(transport, UNARY_DEADLINE_MS).stream(method, undefined, undefined, undefined, (async function* () {})());
    expect(stream.mock.calls[0][2]).toBeUndefined();
  });

  it('stays above the longest long poll of the server (25 s)', () => {
    expect(UNARY_DEADLINE_MS).toBeGreaterThan(25_000);
  });
});
