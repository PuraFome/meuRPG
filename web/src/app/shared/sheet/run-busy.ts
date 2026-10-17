import type { WritableSignal } from '@angular/core';

/**
 * One request in the air: the error of the last is cleared, the flag stays up until the work ends (done or failed), and
 * `fail` words what went wrong (the flag stays up while an async `fail` finishes). The guard against a second request (`busy()` already up) belongs to the caller, which
 * knows what else must be present before it asks.
 */
export async function runBusy(
  busy: WritableSignal<boolean>,
  error: WritableSignal<string>,
  work: () => Promise<void>,
  fail: (err: unknown) => void | Promise<void>,
): Promise<void> {
  busy.set(true);
  error.set('');
  try {
    await work();
  } catch (err) {
    await fail(err);
  } finally {
    busy.set(false);
  }
}
