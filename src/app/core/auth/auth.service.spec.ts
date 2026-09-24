import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import {
  provideHttpClientTesting,
  HttpTestingController,
} from '@angular/common/http/testing';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import {
  AuthService,
  AUTH_TOKEN_KEY,
  CHARACTERS_CACHE_KEY,
} from './auth.service';
import { environment } from '../../../environments/environment';

describe('AuthService', () => {
  let service: AuthService;
  let httpMock: HttpTestingController;

  beforeEach(() => {
    localStorage.clear();
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting()],
    });
    service = TestBed.inject(AuthService);
    httpMock = TestBed.inject(HttpTestingController);
  });

  afterEach(() => {
    httpMock.verify();
    localStorage.clear();
  });

  it('stays unauthenticated when no token is stored', async () => {
    await service.load();
    expect(service.user()).toBeNull();
    httpMock.expectNone(`${environment.apiBaseUrl}/api/me`);
  });

  it('hydrates the user from /api/me when a token exists', async () => {
    localStorage.setItem(AUTH_TOKEN_KEY, 'ma_test-token');

    const pending = service.load();
    const req = httpMock.expectOne(`${environment.apiBaseUrl}/api/me`);
    expect(req.request.headers.get('Authorization')).toBe(
      'Bearer ma_test-token',
    );
    req.flush({ sub: 'sub-1', email: 'hero@example.com', name: 'Hero' });
    await pending;

    expect(service.user()).toEqual({
      sub: 'sub-1',
      email: 'hero@example.com',
      name: 'Hero',
    });
  });

  it('clears the session and character cache on an invalid token', async () => {
    localStorage.setItem(AUTH_TOKEN_KEY, 'ma_expired');
    localStorage.setItem(CHARACTERS_CACHE_KEY, '[]');

    const pending = service.load();
    httpMock
      .expectOne(`${environment.apiBaseUrl}/api/me`)
      .flush({}, { status: 401, statusText: 'Unauthorized' });
    await pending;

    expect(service.user()).toBeNull();
    expect(service.token()).toBeNull();
    expect(localStorage.getItem(CHARACTERS_CACHE_KEY)).toBeNull();
  });

  it('clearSession drops token, user and cache', async () => {
    localStorage.setItem(AUTH_TOKEN_KEY, 'ma_x');
    localStorage.setItem(CHARACTERS_CACHE_KEY, '[1]');
    service.clearSession();

    expect(service.token()).toBeNull();
    expect(service.user()).toBeNull();
    expect(localStorage.getItem(CHARACTERS_CACHE_KEY)).toBeNull();
  });
});
