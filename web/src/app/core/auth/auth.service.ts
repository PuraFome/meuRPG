import { Injectable, computed, inject, signal } from '@angular/core';
import { Code, ConnectError, createClient } from '@connectrpc/connect';
import { timestampDate } from '@bufbuild/protobuf/wkt';

import { IdentityService } from '../../../gen/meurpg/identity/v1/identity_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';

/**
 * The signed-in user, as far as the UI needs to know.
 *
 * `displayName` is always `null` today: `meurpg.identity.v1.User` carries
 * only an opaque account ID (see identity.proto and docs/privacidade.md —
 * no e-mail, name or photo from the sign-in provider). The campaigns module
 * is adding a display name and an `IdentityService.UpdateProfile` RPC;
 * once `GetMe` returns one, map it in `refresh()` below and every screen
 * that reads `displayName` (the user menu, in particular) picks it up with
 * no other change.
 */
export interface AuthUser {
  readonly id: string;
  readonly displayName: string | null;
}

/**
 * Where the app stands on "who is signed in", as a single value so a
 * template only ever renders one of these four cases — no separate
 * loading/error booleans that could disagree with each other.
 *
 * `unavailable` is deliberately distinct from `signed-out`: it means the
 * server (or its database) did not answer, not that there is no session.
 * Showing it as signed-out would be actively misleading for someone who is
 * in fact still signed in.
 */
export type AuthState =
  | { readonly status: 'unknown' }
  | { readonly status: 'signed-out' }
  | {
      readonly status: 'signed-in';
      readonly user: AuthUser;
      readonly sessionExpiresAt: Date | null;
    }
  | { readonly status: 'unavailable' };

/**
 * Session state for the whole app, backed by `IdentityService`.
 *
 * There is no separate "login" RPC to call: signing in is a browser
 * redirect to the server (see `signIn`), started fresh every time, and this
 * service only ever *reads* the result through `GetMe`. `refresh()` runs
 * once eagerly (see the constructor) — in practice as soon as anything in
 * the app shell injects this service, which happens at startup because the
 * user menu is always in the toolbar.
 */
@Injectable({ providedIn: 'root' })
export class AuthService {
  private readonly client = createClient(IdentityService, inject(CONNECT_TRANSPORT));

  private readonly stateSignal = signal<AuthState>({ status: 'unknown' });

  /** The current session state. Read this, never call GetMe directly. */
  readonly state = this.stateSignal.asReadonly();

  readonly isSignedIn = computed(() => this.stateSignal().status === 'signed-in');

  constructor() {
    void this.refresh();
  }

  /**
   * Calls `GetMe` and updates `state` from the result. Safe to call again
   * later (e.g. to retry after `unavailable`, or once a session's
   * `sessionExpiresAt` has passed) — it always settles to one of the three
   * resolved states, never throws.
   */
  async refresh(): Promise<void> {
    try {
      const res = await this.client.getMe({});
      this.stateSignal.set({
        status: 'signed-in',
        user: { id: res.user?.id ?? '', displayName: null },
        sessionExpiresAt: res.sessionExpiresAt ? timestampDate(res.sessionExpiresAt) : null,
      });
    } catch (err) {
      // Code.Unauthenticated is the one case GetMe documents as "there is
      // no session" (see identity.proto). Everything else — a network
      // failure, the database being down, a future error we didn't
      // anticipate — must not read as signed-out, so it falls back to
      // `unavailable`. ConnectError.from's own default (Code.Unknown) is
      // overridden here for exactly that reason: a plain thrown error
      // (e.g. fetch rejecting before it reaches the server) should land on
      // `unavailable` too, not on some third, unhandled bucket.
      const connectErr = ConnectError.from(err, Code.Unavailable);
      this.stateSignal.set(
        connectErr.code === Code.Unauthenticated
          ? { status: 'signed-out' }
          : { status: 'unavailable' },
      );
    }
  }

  /**
   * Sends the browser to the server's sign-in flow with a full-page
   * navigation (this is a redirect dance with the OIDC provider, not
   * something the Angular router can do). `returnTo` must be a path on
   * this site: the server's `safeReturnTo` rejects anything else — a
   * malformed value here would only turn into the server's plain-text 400.
   */
  signIn(returnTo: string): void {
    const path = returnTo.startsWith('/') && !returnTo.startsWith('//') ? returnTo : '/';
    window.location.assign(`/auth/login?return_to=${encodeURIComponent(path)}`);
  }

  /**
   * Ends the session on the server, then always returns to the home page
   * with a full navigation, so the next `AuthService` starts clean and
   * `GetMe` reports the real state. The navigation still happens even if
   * `SignOut` itself fails (e.g. the server is `unavailable`): the user
   * asked to leave, and a failed revoke on the server is not something a
   * client-side error message would let them fix anyway.
   */
  async signOut(): Promise<void> {
    try {
      await this.client.signOut({});
    } catch {
      // Best-effort — see the doc comment above.
    }
    window.location.assign('/');
  }
}
