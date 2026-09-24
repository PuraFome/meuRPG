import { Injector } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { Router } from '@angular/router';
import type { ActivatedRouteSnapshot, RouterStateSnapshot } from '@angular/router';
import { describe, expect, it, vi } from 'vitest';
import { authGuard, loginGuard } from './auth.guard';
import { AuthService } from './auth.service';

function routeWith(data: Record<string, unknown>): ActivatedRouteSnapshot {
  return { data } as unknown as ActivatedRouteSnapshot;
}

const state = {} as RouterStateSnapshot;

describe('authGuard', () => {
  let injector: Injector;

  function setup(user: unknown) {
    const authStub = {
      ready: vi.fn().mockResolvedValue(undefined),
      user: () => user,
    };
    const routerStub = { createUrlTree: vi.fn((commands: unknown[]) => ({ commands })) };
    TestBed.configureTestingModule({
      providers: [
        { provide: AuthService, useValue: authStub },
        { provide: Router, useValue: routerStub },
      ],
    });
    injector = TestBed.inject(Injector);
    return { authStub, routerStub };
  }

  it('allows a route flagged as public without any session', async () => {
    const { authStub } = setup(null);

    const result = await TestBed.runInInjectionContext(() =>
      authGuard(routeWith({ public: true }), state),
    );

    expect(result).toBe(true);
    expect(authStub.ready).not.toHaveBeenCalled();
  });

  it('redirects to /login when there is no authenticated user', async () => {
    const { authStub, routerStub } = setup(null);

    const result = await TestBed.runInInjectionContext(() =>
      authGuard(routeWith({}), state),
    );

    expect(authStub.ready).toHaveBeenCalled();
    expect(routerStub.createUrlTree).toHaveBeenCalledWith(['/login']);
    expect(result).toEqual({ commands: ['/login'] });
  });

  it('allows a protected route when a user is present', async () => {
    const { routerStub } = setup({ sub: 's', email: 'e', name: 'n' });

    const result = await TestBed.runInInjectionContext(() =>
      authGuard(routeWith({}), state),
    );

    expect(result).toBe(true);
    expect(routerStub.createUrlTree).not.toHaveBeenCalled();
  });

  it('loginGuard bounces an authenticated user to the home route', async () => {
    const { routerStub } = setup({ sub: 's', email: 'e', name: 'n' });

    const result = await TestBed.runInInjectionContext(() =>
      loginGuard(routeWith({}), state),
    );

    expect(routerStub.createUrlTree).toHaveBeenCalledWith(['/']);
    expect(result).toEqual({ commands: ['/'] });
  });

  it('loginGuard lets an anonymous visitor see the login page', async () => {
    setup(null);

    const result = await TestBed.runInInjectionContext(() =>
      loginGuard(routeWith({}), state),
    );

    expect(result).toBe(true);
    expect(injector).toBeDefined();
  });
});
