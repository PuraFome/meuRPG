import { Injectable, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter, Router, UrlTree } from '@angular/router';
import type { Observable } from 'rxjs';
import { firstValueFrom } from 'rxjs';

import { authGuard } from './auth.guard';
import { AuthState, AuthService } from './auth.service';

/** A stand-in for AuthService whose state this spec fully controls,
 * instead of driving the real GetMe call through a fake transport. */
@Injectable()
class FakeAuthService {
  private readonly stateSignal = signal<AuthState>({ status: 'unknown' });
  readonly state = this.stateSignal.asReadonly();
  readonly signIn = vi.fn();

  set(state: AuthState): void {
    this.stateSignal.set(state);
  }
}

describe('authGuard', () => {
  let auth: FakeAuthService;
  let router: Router;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideRouter([]), { provide: AuthService, useClass: FakeAuthService }],
    });
    auth = TestBed.inject(AuthService) as unknown as FakeAuthService;
    router = TestBed.inject(Router);
  });

  function runGuard(url: string): Observable<boolean | UrlTree> {
    return TestBed.runInInjectionContext(() =>
      authGuard({} as never, { url } as never),
    ) as Observable<boolean | UrlTree>;
  }

  it('allows the navigation when signed in', async () => {
    auth.set({
      status: 'signed-in',
      user: { id: 'u1', displayName: null },
      sessionExpiresAt: null,
    });

    await expect(firstValueFrom(runGuard('/campanhas'))).resolves.toBe(true);
  });

  it('sends the browser through sign-in when signed out, and cancels the navigation', async () => {
    auth.set({ status: 'signed-out' });

    await expect(firstValueFrom(runGuard('/campanhas'))).resolves.toBe(false);
    expect(auth.signIn).toHaveBeenCalledWith('/campanhas');
  });

  it('redirects to /indisponivel, keeping return_to, when the server is unavailable', async () => {
    auth.set({ status: 'unavailable' });

    const decision = await firstValueFrom(runGuard('/campanhas'));
    expect(decision).toBeInstanceOf(UrlTree);
    expect(router.serializeUrl(decision as UrlTree)).toBe('/indisponivel?return_to=%2Fcampanhas');
    expect(auth.signIn).not.toHaveBeenCalled();
  });

  it('waits for a resolved state instead of guessing while unknown', async () => {
    const decision = firstValueFrom(runGuard('/campanhas'));
    let settled = false;
    void decision.then(() => (settled = true));

    // Give any pending microtask a chance to run; the guard must still not
    // have decided anything while AuthService says 'unknown'.
    await Promise.resolve();
    await Promise.resolve();
    expect(settled).toBe(false);

    auth.set({
      status: 'signed-in',
      user: { id: 'u1', displayName: null },
      sessionExpiresAt: null,
    });

    await expect(decision).resolves.toBe(true);
  });
});
