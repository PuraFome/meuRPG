/** A fresh idempotency key (a UUID). A screen makes one per action and sends
 * it again on a retry: a second call with the same key changes nothing and
 * answers with what the first one did. */
export function newKey(): string {
  return crypto.randomUUID();
}

/**
 * The idempotency key of one action on a screen (a create button, a form), kept across the retries of that
 * action and never reused for a new one.
 *
 * - `keyFor(what)` gives the key for the action `what` describes (the values the request carries). The same
 *   values again are a retry (a second tap, or a try after a lost answer), so the key stays: the server
 *   answers with what the first call made. Other values are another action, so the key is new: the server
 *   refuses ("invalid_argument") a key used again for a different request.
 * - `renew()` is called when the action worked: the next one, even with the same values (two notes with the
 *   same text), is a new action with a new key.
 */
export class ActionKey {
  private key = newKey();
  private last = '';

  keyFor(what: unknown): string {
    // BigInt values (a seed) have no JSON form of their own.
    const print = JSON.stringify(what, (_name, value: unknown) =>
      typeof value === 'bigint' ? value.toString() : value,
    );
    if (print !== this.last) {
      this.key = newKey();
      this.last = print;
    }
    return this.key;
  }

  renew(): void {
    this.key = newKey();
    this.last = '';
  }
}
