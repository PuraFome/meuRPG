import { Injectable, OnDestroy, inject } from '@angular/core';
import { Subscription, forkJoin, of } from 'rxjs';
import { catchError } from 'rxjs/operators';
import { HttpErrorResponse } from '@angular/common/http';
import { StoreService } from '../store/store.service';
import type { StoreEvent } from '../store/store.service';
import { SearchService } from './search.service';
import { CharactersService } from './characters.service';
import { MapsService } from './maps.service';
import { normalizeMap } from './map-payload';

import type {
  Character,
  CampaignFolder,
  GalleryItem,
  MapData,
  SessionState,
  RuleBook,
} from '../models';

type PersistableEntity = Character | CampaignFolder | GalleryItem | MapData | SessionState | RuleBook;

const STORE_KEYS = ['characters', 'campaigns', 'gallery', 'maps', 'sessions', 'rules'];

/** Collections cached in localStorage (maps now live in the database). */
const LOCAL_PERSIST_KEYS = ['characters', 'campaigns', 'gallery', 'sessions', 'rules'];

@Injectable({
  providedIn: 'root',
})
export class PersistenceService implements OnDestroy {
  private subscriptions: Subscription[] = [];
  private hydrating = false;
  private readonly mapBackgroundCache = new Map<string, string | undefined>();

  private readonly store = inject<StoreService<PersistableEntity>>(StoreService);
  private readonly search = inject(SearchService);
  private readonly characters = inject(CharactersService);
  private readonly maps = inject(MapsService);

  /** Initialize persistence: load local data, hydrate from the API, sync changes. */
  init(): void {
    // Load cached (non-map) collections from localStorage
    for (const key of LOCAL_PERSIST_KEYS) {
      const raw = localStorage.getItem(`meurpg_${key}`);
      if (raw) {
        try {
          const items = JSON.parse(raw) as PersistableEntity[];
          for (const item of items) {
            this.store.set(key, item);
          }
        } catch {
          // skip corrupt entries
        }
      }
    }

    // Subscribe to store events and persist changes
    this.subscriptions.push(
      this.store.events$.subscribe((event) => {
        const collection = event.collection;
        if (LOCAL_PERSIST_KEYS.includes(collection)) {
          try {
            const snapshot = this.store.snapshot(collection);
            localStorage.setItem(`meurpg_${collection}`, JSON.stringify(snapshot));
          } catch (err) {
            // Quota exceeded (e.g. large base64 images) must never roll back the
            // in-memory save; the API remains the source of truth for maps.
            console.warn('[persistence] localStorage write skipped', err);
          }
        }
        if (this.hydrating) {
          return;
        }
        if (collection === 'characters') {
          this.syncCharacter(event);
        } else if (collection === 'maps') {
          this.syncMap(event);
        }
      }),
    );

    // Index searchable items from store collections
    this.subscriptions.push(
      ...STORE_KEYS.map((key) =>
        this.store.getAll(key).subscribe((items) => {
          for (const item of items) {
            this.search.indexEntity({
              id: item.id,
              type: keyToEntityType(key),
              name: ((item as unknown as Record<string, unknown>)['name'] as string) ?? item.id,
              description: ((item as unknown as Record<string, unknown>)['description'] as string) ?? '',
              score: 1,
              entity: item,
            });
          }
        }),
      ),
    );

    // One-time migration: legacy maps cached in localStorage before they were
    // stored in the database. Re-emitting them as store events POSTs them.
    this.migrateLegacyMaps();

    this.hydrating = true;
    forkJoin({
      characters: this.characters.list().pipe(
        catchError((e) => {
          console.error('[persistence] character hydration failed', e);
          return of([] as Character[]);
        }),
      ),
      maps: this.maps.list().pipe(
        catchError((e) => {
          console.error('[persistence] map hydration failed', e);
          return of([] as MapData[]);
        }),
      ),
    }).subscribe({
      next: ({ characters, maps }) => {
        for (const item of characters) {
          this.store.set('characters', item);
        }
        for (const raw of maps) {
          const item = normalizeMap(raw as unknown as Record<string, unknown>);
          this.store.set('maps', item);
          this.mapBackgroundCache.set(item.id, item.backgroundImage);
        }
        this.hydrating = false;
      },
      error: () => {
        this.hydrating = false;
      },
    });
  }

  private migrateLegacyMaps(): void {
    const raw = localStorage.getItem('meurpg_maps');
    if (!raw) {
      return;
    }
    localStorage.removeItem('meurpg_maps');
    try {
      const legacy = JSON.parse(raw) as MapData[];
      for (const map of legacy) {
        this.store.set('maps', normalizeMap(map as unknown as Record<string, unknown>));
      }
    } catch {
      // skip corrupt legacy cache
    }
  }

  private syncCharacter(event: StoreEvent): void {
    const onError = (e: unknown) => console.error('[persistence] API write failed', e);
    switch (event.type) {
      case 'created':
        this.characters.create(event.payload as Character).subscribe({ error: onError });
        break;
      case 'updated':
        this.characters.update(event.id, event.payload as Character).subscribe({ error: onError });
        break;
      case 'deleted':
        this.characters.remove(event.id).subscribe({ error: onError });
        break;
    }
  }

  private syncMap(event: StoreEvent): void {
    const onError = (e: unknown) => {
      if (e instanceof HttpErrorResponse) {
        console.error('[persistence] map API write failed', e.status, e.error);
      } else {
        console.error('[persistence] map API write failed', e);
      }
    };
    switch (event.type) {
      case 'created': {
        const payload = event.payload as MapData;
        this.mapBackgroundCache.set(payload.id, payload.backgroundImage);
        this.maps.create(payload).subscribe({ error: onError });
        break;
      }
      case 'updated': {
        const payload = { ...(event.payload as MapData) };
        // A imagem de fundo (base64) é o campo mais pesado; só reenvia quando muda,
        // para que pontos/desenho salvem rápido e imediatamente.
        if (
          this.mapBackgroundCache.has(event.id) &&
          payload.backgroundImage === this.mapBackgroundCache.get(event.id)
        ) {
          delete payload.backgroundImage;
        } else {
          this.mapBackgroundCache.set(event.id, payload.backgroundImage);
        }
        this.maps.update(event.id, payload).subscribe({ error: onError });
        break;
      }
      case 'deleted':
        this.mapBackgroundCache.delete(event.id);
        this.maps.remove(event.id).subscribe({ error: onError });
        break;
    }
  }

  ngOnDestroy(): void {
    this.subscriptions.forEach((s) => s.unsubscribe());
  }
}

function keyToEntityType(key: string): 'character' | 'campaign' | 'gallery' | 'map' | 'session' | 'rules' {
  switch (key) {
    case 'characters': return 'character';
    case 'campaigns': return 'campaign';
    case 'gallery': return 'gallery';
    case 'maps': return 'map';
    case 'sessions': return 'session';
    case 'rules': return 'rules';
    default: return 'character';
  }
}
