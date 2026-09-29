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
  return (
    messages[connectErr.code] ??
    messages[Code.Unavailable] ??
    'Não foi possível falar com o servidor agora. Tente de novo em instantes.'
  );
}
