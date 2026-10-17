import { signal } from '@angular/core';

import { runBusy } from './run-busy';

describe('runBusy', () => {
  it('raises the flag and clears the last error before the work starts', async () => {
    const busy = signal(false);
    const error = signal('antes');
    let seen: [boolean, string] | null = null;
    await runBusy(
      busy,
      error,
      async () => {
        seen = [busy(), error()];
      },
      () => undefined,
    );
    expect(seen).toEqual([true, '']);
    expect(busy()).toBe(false);
  });

  it('hands a failure to `fail` and still lowers the flag', async () => {
    const busy = signal(false);
    const error = signal('');
    const boom = new Error('boom');
    await runBusy(
      busy,
      error,
      () => Promise.reject(boom),
      (err) => error.set(`falhou: ${(err as Error).message}`),
    );
    expect(error()).toBe('falhou: boom');
    expect(busy()).toBe(false);
  });

  it('keeps the flag up while the work is pending', async () => {
    const busy = signal(false);
    let finish!: () => void;
    const pending = runBusy(
      busy,
      signal(''),
      () => new Promise<void>((resolve) => (finish = resolve)),
      () => undefined,
    );
    expect(busy()).toBe(true);
    finish();
    await pending;
    expect(busy()).toBe(false);
  });
});
