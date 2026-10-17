import type { Provider } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import { appConfig } from './app.config';
import { AuthService } from './core/auth/auth.service';
import { SESSION_REFRESHER } from './core/auth/session-refresher';

describe('appConfig', () => {
  it("binds the transport's session refresher to the AuthService instance", () => {
    const binding = (appConfig.providers as Provider[]).find(
      (p) => typeof p === 'object' && 'provide' in p && p.provide === SESSION_REFRESHER,
    );
    const auth = { refresh: () => Promise.resolve() };
    TestBed.configureTestingModule({
      providers: [binding as Provider, { provide: AuthService, useValue: auth }],
    });
    expect(TestBed.inject(SESSION_REFRESHER)).toBe(auth);
  });
});
