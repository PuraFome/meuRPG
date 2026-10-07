import { InjectionToken } from '@angular/core';
import { Code, ConnectError } from '@connectrpc/connect';
import type { Interceptor, Transport } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';

import { isRateLimited, rateLimitedMessage } from './connect-errors';

/**
 * A call the server turned away for asking too often (`resource_exhausted`
 * with a `Retry-After` header) becomes `unavailable`, with the wait in its
 * message. Every screen already treats `unavailable` as "try again in a
 * moment" and never as a final refusal, so none of them shows the copy of a
 * full gallery or of a campaign at its limit (also `resource_exhausted`) for
 * what is only "slow down". Nothing retries on its own: a person asks again,
 * and the live stream, which does retry, backs off (live-stream.ts).
 * `isRateLimited` still recognises the converted error, by the same header.
 */
export const rateLimitInterceptor: Interceptor = (next) => async (req) => {
  try {
    return await next(req);
  } catch (err) {
    const e = ConnectError.from(err);
    if (e.code === Code.ResourceExhausted && isRateLimited(e)) {
      throw new ConnectError(rateLimitedMessage(e), Code.Unavailable, e.metadata, undefined, e);
    }
    throw err;
  }
};

/**
 * The Connect transport every generated client uses to reach the backend.
 *
 * `baseUrl: '/'` relies on the app being served from the same origin as the
 * API (see docs/arquitetura.md#frontend-web): in production the Go server
 * serves both, and in dev `proxy.conf.json` forwards RPC paths to it. There
 * is deliberately no separate "API URL" to configure per environment.
 *
 * `fetch` is overridden only to force `credentials: 'same-origin'`: the
 * session lives in the `__Host-meurpg_session` cookie (see identity.proto),
 * so every RPC needs it sent. Same-origin `fetch` already defaults to that
 * per the Fetch spec, but the session cookie is exactly the kind of thing
 * that must not depend on a runtime default — this makes it explicit and
 * pins it against ever becoming cross-origin by accident.
 *
 * Every unary call also carries `Connect-Protocol-Version: 1` already,
 * unconditionally, from `@connectrpc/connect`'s own request-header code —
 * nothing to add here for that (see docs/arquitetura.md#csrf).
 */
export const CONNECT_TRANSPORT = new InjectionToken<Transport>('CONNECT_TRANSPORT', {
  providedIn: 'root',
  factory: () =>
    createConnectTransport({
      baseUrl: '/',
      interceptors: [rateLimitInterceptor],
      fetch: (input, init) => fetch(input, { ...init, credentials: 'same-origin' }),
    }),
});
