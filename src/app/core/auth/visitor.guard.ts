import { inject } from '@angular/core';
import { Router } from '@angular/router';
import type { CanActivateFn } from '@angular/router';

import { AuthService } from './auth.service';

/** Bounces an invited player (visitor) to the only section they may access. */
export const visitorGuard: CanActivateFn = async () => {
  const auth = inject(AuthService);
  const router = inject(Router);
  await auth.ready();

  return auth.isVisitor() ? router.createUrlTree(['/personagens']) : true;
};