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
 */
export const CONNECT_TRANSPORT = new InjectionToken<Transport>('CONNECT_TRANSPORT', {
  providedIn: 'root',
  factory: () => createConnectTransport({ baseUrl: '/' }),
});
