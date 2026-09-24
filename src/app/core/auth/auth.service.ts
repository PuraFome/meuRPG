import { Injectable, inject, signal } from '@angular/core';
import { HttpClient, HttpHeaders } from '@angular/common/http';
import { firstValueFrom, of } from 'rxjs';
import { catchError } from 'rxjs/operators';

import { environment } from '../../../environments/environment';

export interface AuthUser {
  sub: string;
  email: string;
  name: string;
}

/** Bearer token delivered by the OAuth callback via the URL fragment. */
export const AUTH_TOKEN_KEY = 'meurpg.auth.token';

/** localStorage key used by PersistenceService to cache characters. */
export const CHARACTERS_CACHE_KEY = 'meurpg_characters';

/** localStorage key holding the in-app route to resume after a Google login round-trip. */
export const PENDING_REDIRECT_KEY = 'meurpg.auth.pendingRedirect';

/**
 * Bearer-token session store. The OAuth callback lands back on the SPA with
 * `#token=<opaque>`; browsers never send fragments to servers, so the token
 * survives cross-site hosting (github.io page -> onrender.com API) where
 * third-party cookies are blocked.
 */
@Injectable({ providedIn: 'root' })
export class AuthService {
  private readonly http = inject(HttpClient);

  readonly user = signal<AuthUser | null>(null);

  private readyPromise: Promise<void> | null = null;

  /** Resolve once the persisted session has been validated against the API. */
  ready(): Promise<void> {
    if (!this.readyPromise) {
      this.readyPromise = this.load();
    }
    return this.readyPromise;
  }

  token(): string | null {
    return localStorage.getItem(AUTH_TOKEN_KEY);
  }

  /** Kick off the Google OAuth flow; the server redirects back with a token. */
  login(): void {
    window.location.href = `${environment.apiBaseUrl}/api/auth/google`;
  }

  /** Validate the persisted token and hydrate the current user profile. */
  async load(): Promise<void> {
    if (!this.token()) {
      this.user.set(null);
      return;
    }
    try {
      const payload = await firstValueFrom(
        this.http.get<AuthUser>(`${environment.apiBaseUrl}/api/me`, {
          headers: this.authHeaders(),
        }),
      );
      this.user.set(payload);
    } catch {
      // Unknown/expired token or backend down: treat as unauthenticated.
      this.clearSession();
    }
  }

  async logout(): Promise<void> {
    await firstValueFrom(
      this.http
        .post(
          `${environment.apiBaseUrl}/api/auth/logout`,
          {},
          { headers: this.authHeaders() },
        )
        .pipe(catchError(() => of(null))),
    );
    this.clearSession();
  }

  async deleteAccount(): Promise<void> {
    await firstValueFrom(
      this.http.delete(`${environment.apiBaseUrl}/api/auth/account`, {
        headers: this.authHeaders(),
      }),
    );
    this.clearSession();
  }

  /** Drop the token + user and evict the per-user character cache. */
  clearSession(): void {
    localStorage.removeItem(AUTH_TOKEN_KEY);
    localStorage.removeItem(CHARACTERS_CACHE_KEY);
    this.user.set(null);
  }

  private authHeaders(): HttpHeaders {
    const token = this.token();
    return token
      ? new HttpHeaders({ Authorization: `Bearer ${token}` })
      : new HttpHeaders();
  }
}
