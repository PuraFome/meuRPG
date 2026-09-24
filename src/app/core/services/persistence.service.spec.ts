import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting, HttpTestingController } from '@angular/common/http/testing';
import { PersistenceService } from './persistence.service';
import { StoreService } from '../store/store.service';
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

describe('PersistenceService', () => {
  let service: PersistenceService;
  let store: StoreService<Character>;
  let httpMock: HttpTestingController;
  const url = `${environment.apiBaseUrl}/api/characters`;
  const mapsUrl = `${environment.apiBaseUrl}/api/maps`;

  beforeEach(() => {
    localStorage.clear();
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting()],
    });
    service = TestBed.inject(PersistenceService);
    store = TestBed.inject(StoreService);
    httpMock = TestBed.inject(HttpTestingController);
  });

  afterEach(() => {
    httpMock.verify();
    localStorage.clear();
  });

  it('hydrates the store from GET /api/characters without issuing POSTs', () => {
    const c1 = mockCharacter({ id: 'c1' });

    service.init();

    const req = httpMock.expectOne(url);
    expect(req.request.method).toBe('GET');
    req.flush([c1]);
    httpMock.expectOne(mapsUrl).flush([]);

    expect(store.snapshot('characters')).toEqual([c1]);
    httpMock.expectNone((r) => r.method === 'POST' && r.url === url);
  });

  it('POSTs exactly once when a character is created after hydration', () => {
    service.init();
    httpMock.expectOne(url).flush([]);
    httpMock.expectOne(mapsUrl).flush([]);

    const c2 = mockCharacter({ id: 'c2' });
    store.set('characters', c2);

    const req = httpMock.expectOne(url);
    expect(req.request.method).toBe('POST');
    const { createdAt: _c, updatedAt: _u, ...payload } = c2;
    expect(req.request.body).toEqual(payload);
    req.flush(c2);
  });

  it('PATCHes /api/characters/:id when a character is updated', () => {
    const c1 = mockCharacter({ id: 'c1' });
    service.init();
    httpMock.expectOne(url).flush([c1]);
    httpMock.expectOne(mapsUrl).flush([]);

    const updated = { ...c1, name: 'Megaera' };
    store.update('characters', c1.id, updated);

    const req = httpMock.expectOne(`${url}/c1`);
    expect(req.request.method).toBe('PATCH');
    const { createdAt: _c, updatedAt: _u, ...payload } = updated;
    expect(req.request.body).toEqual(payload);
    req.flush(updated);
  });

  it('DELETEs /api/characters/:id when a character is removed', () => {
    const c1 = mockCharacter({ id: 'c1' });
    service.init();
    httpMock.expectOne(url).flush([c1]);
    httpMock.expectOne(mapsUrl).flush([]);

    store.delete('characters', c1.id);

    const req = httpMock.expectOne(`${url}/c1`);
    expect(req.request.method).toBe('DELETE');
    req.flush(null);
  });

  it('keeps localStorage-loaded items and does not throw when the API fails', () => {
    const cached = mockCharacter({ id: 'cached' });
    localStorage.setItem('meurpg_characters', JSON.stringify([cached]));
    const errors: unknown[] = [];
    const originalError = console.error;
    console.error = (...args: unknown[]) => errors.push(args);

    try {
      expect(() => service.init()).not.toThrow();

      httpMock
        .expectOne(url)
        .flush({ message: 'boom' }, { status: 500, statusText: 'Server Error' });
      httpMock.expectOne(mapsUrl).flush([]);

      expect(store.snapshot('characters').map((c) => c.id)).toEqual(['cached']);

      const c2 = mockCharacter({ id: 'c2' });
      expect(() => store.set('characters', c2)).not.toThrow();

      const post = httpMock.expectOne(url);
      expect(post.request.method).toBe('POST');
      post.flush({ message: 'boom' }, { status: 500, statusText: 'Server Error' });

      const persisted = JSON.parse(localStorage.getItem('meurpg_characters')!) as Character[];
      expect(persisted.map((c) => c.id)).toEqual(['cached', 'c2']);
      expect(errors.length).toBeGreaterThan(0);
    } finally {
      console.error = originalError;
    }
  });
});
