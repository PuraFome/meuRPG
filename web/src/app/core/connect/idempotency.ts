/** A fresh idempotency key (a UUID). A screen makes one per action and sends
 * it again on a retry: a second call with the same key changes nothing and
 * answers with what the first one did. */
export function newKey(): string {
  return crypto.randomUUID();
}
