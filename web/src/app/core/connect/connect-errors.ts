import { Code, ConnectError } from '@connectrpc/connect';

/**
 * Turns whatever a Connect call rejected with into a Portuguese message a
 * screen can show as-is.
 *
 * `messages` supplies the wording for the codes this call can meaningfully
 * fail with (see each `.proto` service comment for the list); anything else
 * — including a plain network failure, which `ConnectError.from` cannot tell
 * apart from "the server is down" — falls back to `messages[Code.Unavailable]`
 * if given, or a generic message otherwise. This mirrors how `AuthService`
 * treats an unrecognized failure as `unavailable` rather than a new,
 * unhandled bucket (see its `refresh()`).
 */
export function describeConnectError(
  err: unknown,
  messages: Partial<Record<Code, string>>,
): string {
  const connectErr = ConnectError.from(err, Code.Unavailable);
  // "Slow down" is the same for every call: no screen's own wording applies.
  if (isRateLimited(connectErr)) {
    return rateLimitedMessage(connectErr);
  }
  return (
    messages[connectErr.code] ??
    messages[Code.Unavailable] ??
    'Não foi possível falar com o servidor agora. Tente de novo em instantes.'
  );
}

/**
 * Whether the server turned the call away for asking too often (the per-user
 * and per-address limits, docs/architecture.md#abuse-limits). It tells
 * by the `Retry-After` header, which no other refusal carries: a full gallery
 * or a campaign at its limit is also `resource_exhausted`, and waiting does
 * not fix those.
 */
export function isRateLimited(err: unknown): boolean {
  const e = ConnectError.from(err, Code.Unavailable);
  return (
    (e.code === Code.ResourceExhausted || e.code === Code.Unavailable) &&
    e.metadata.has('Retry-After')
  );
}

/** "Muitas ações em pouco tempo. Espere 3 segundos e tente de novo." */
export function rateLimitedMessage(err: unknown): string {
  const secs = Number(ConnectError.from(err, Code.Unavailable).metadata.get('Retry-After'));
  const wait =
    Number.isFinite(secs) && secs >= 1
      ? `Espere ${Math.ceil(secs)} ${Math.ceil(secs) === 1 ? 'segundo' : 'segundos'}`
      : 'Espere alguns segundos';
  return `Muitas ações em pouco tempo. ${wait} e tente de novo.`;
}
