import { inject } from '@angular/core';
import { Router } from '@angular/router';
import type { CanActivateFn } from '@angular/router';

import { AuthService } from './auth.service';

/**
 * Blocks a route until the session is resolved. Routes flagged with
 * `data: { public: true }` (the share-link join flow) are always allowed.
 */
export const authGuard: CanActivateFn = async (route) => {
  if (route.data?.['public'] === true) {
    return true;
  }

  const auth = inject(AuthService);
  const router = inject(Router);
  await auth.ready();

  return auth.user() ? true : router.createUrlTree(['/login']);
};

/** Keeps already-authenticated users out of the login page. */
export const loginGuard: CanActivateFn = async () => {
  const auth = inject(AuthService);
  const router = inject(Router);
  await auth.ready();

  return auth.user() ? router.createUrlTree(['/']) : true;
};
