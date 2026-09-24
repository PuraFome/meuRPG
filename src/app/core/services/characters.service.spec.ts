import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting, HttpTestingController } from '@angular/common/http/testing';
import { CharactersService } from './characters.service';
import type { JoinTokenInfo } from './characters.service';
import { environment } from '../../../environments/environment';
import type { Character } from '../models/character';

function mockCharacter(overrides: Partial<Character> = {}): Character {
  return {
    id: '1',
    name: 'Zagreus',
    description: 'Filho de Hades',
    type: 'player',
    attributes: {},
    skills: [],
    inventory: [],
    quotes: [],
    createdAt: new Date('2026-01-01'),
    updatedAt: new Date('2026-01-01'),
    ...overrides,
  };
}

describe('CharactersService', () => {
  let service: CharactersService;
  let httpMock: HttpTestingController;
  const base = environment.apiBaseUrl;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting()],
    });
    service = TestBed.inject(CharactersService);
    httpMock = TestBed.inject(HttpTestingController);
  });

  afterEach(() => {
    httpMock.verify();
  });

  it('create() emits a POST to /api/characters with the character body', () => {
    const character = mockCharacter();
    let result: Character | undefined;

    service.create(character).subscribe((c) => (result = c));

    const req = httpMock.expectOne(`${base}/api/characters`);
    expect(req.request.method).toBe('POST');
    expect(req.request.url).toBe(`${base}/api/characters`);
    const { createdAt: _c, updatedAt: _u, ...payload } = character;
    expect(req.request.body).toEqual(payload);
    req.flush(character);
    expect(result).toEqual(character);
  });

  it('createJoinToken() emits a POST to /api/characters/join-tokens', () => {
    const tokenInfo: JoinTokenInfo = {
      token: 'abc123',
      expiresAt: '2026-01-02T00:00:00.000Z',
      type: 'player',
    };
    let result: JoinTokenInfo | undefined;

    service.createJoinToken().subscribe((t) => (result = t));

    const req = httpMock.expectOne(`${base}/api/characters/join-tokens`);
    expect(req.request.method).toBe('POST');
    expect(req.request.url).toBe(`${base}/api/characters/join-tokens`);
    req.flush(tokenInfo);
    expect(result).toEqual(tokenInfo);
  });

  it('join() emits a POST to /api/characters/join/:token with the character body', () => {
    const character = mockCharacter();
    const token = 'abc123';
    let result: Character | undefined;

    service.join(token, character).subscribe((c) => (result = c));

    const req = httpMock.expectOne(`${base}/api/characters/join/${token}`);
    expect(req.request.method).toBe('POST');
    expect(req.request.url).toBe(`${base}/api/characters/join/${token}`);
    const { createdAt: _c, updatedAt: _u, ...payload } = character;
    expect(req.request.body).toEqual(payload);
    req.flush(character);
    expect(result).toEqual(character);
  });

  it('validateJoinToken() surfaces a 410 HTTP error to the subscriber', () => {
    let error: unknown;

    service.validateJoinToken('x').subscribe({
      next: () => {
        throw new Error('should not emit next');
      },
      error: (e) => (error = e),
    });

    const req = httpMock.expectOne(`${base}/api/characters/join/x`);
    expect(req.request.method).toBe('GET');
    req.flush({ message: 'Token expired' }, { status: 410, statusText: 'Gone' });

    expect(error).toBeTruthy();
    expect((error as { status: number }).status).toBe(410);
  });
});
