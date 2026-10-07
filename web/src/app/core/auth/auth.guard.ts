import { inject } from '@angular/core';
import { toObservable } from '@angular/core/rxjs-interop';
import { CanActivateFn, Router } from '@angular/router';
import { filter, map, take } from 'rxjs';

import { AuthState, AuthService } from './auth.service';

/**
 * Guards a route that needs a signed-in session (e.g. "Minhas campanhas").
 *
 * - **signed-in**: lets the navigation through.
 * - **signed-out**: there is no session, so this sends the browser through
 *   the server's sign-in flow with `return_to` set to the page the user
 *   asked for (a full navigation, not an Angular route — see
 *   `AuthService.signIn`), then cancels this navigation.
 * - **unavailable**: the server didn't answer, which is not the same as
 *   being signed out — routing through sign-in here could bounce someone
 *   who is in fact signed in. Redirects to the shared "server unavailable"
 *   page instead, keeping the target path so that page can retry into it.
 * - **unknown**: `AuthService` hasn't heard back from the first `GetMe`
 *   yet. Waits for the first resolved state rather than guessing.
 */
export const authGuard: CanActivateFn = (_route, routerState) => {
  const auth = inject(AuthService);
  const router = inject(Router);

  return toObservable(auth.state).pipe(
    filter(
      (state): state is Exclude<AuthState, { status: 'unknown' }> => state.status !== 'unknown',
    ),
    take(1),
    map((state) => {
      if (state.status === 'signed-in') {
        return true;
      }
      if (state.status === 'unavailable') {
        return router.createUrlTree(['/unavailable'], {
          queryParams: { return_to: routerState.url },
        });
      }
      auth.signIn(routerState.url);
      return false;
    }),
  );
};
