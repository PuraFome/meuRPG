import { inject } from '@angular/core';
import { Router } from '@angular/router';
import type { CanActivateFn } from '@angular/router';

import { AuthService, PENDING_REDIRECT_KEY } from './auth.service';

/**
 * Blocks a route until the session is resolved. Routes flagged with
 * `data: { public: true }` are always allowed. When access is denied the
 * attempted URL is stashed so the OAuth round-trip can resume it.
 */
export const authGuard: CanActivateFn = async (route, state) => {
  if (route.data?.['public'] === true) {
    return true;
  }

  const auth = inject(AuthService);
  const router = inject(Router);
  await auth.ready();

  if (auth.user()) {
    return true;
  }

  if (state.url) {
    localStorage.setItem(PENDING_REDIRECT_KEY, state.url);
  }
  return router.createUrlTree(['/login']);
};

/** Keeps already-authenticated users out of the login page. */
export const loginGuard: CanActivateFn = async () => {
  const auth = inject(AuthService);
  const router = inject(Router);
  await auth.ready();

  return auth.user() ? router.createUrlTree(['/']) : true;
};
