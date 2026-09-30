import { InjectionToken } from '@angular/core';
import type { Transport } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';

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
      fetch: (input, init) => fetch(input, { ...init, credentials: 'same-origin' }),
    }),
});
