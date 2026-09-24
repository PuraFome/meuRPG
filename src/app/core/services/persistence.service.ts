import { Injectable, OnDestroy, inject } from '@angular/core';
import { Subscription } from 'rxjs';
import { StoreService } from '../store/store.service';
import type { StoreEvent } from '../store/store.service';
import { SearchService } from './search.service';
import { CharactersService } from './characters.service';

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

@Injectable({
  providedIn: 'root',
})
export class PersistenceService implements OnDestroy {
  private subscriptions: Subscription[] = [];
  private hydrating = false;

  private readonly store = inject<StoreService<PersistableEntity>>(StoreService);
  private readonly search = inject(SearchService);
  private readonly characters = inject(CharactersService);

  /** Initialize persistence: load data from localStorage into the store. */
  init(): void {
    // Load each collection from localStorage
    for (const key of STORE_KEYS) {
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
        if (STORE_KEYS.includes(collection)) {
          const snapshot = this.store.snapshot(collection);
          localStorage.setItem(`meurpg_${collection}`, JSON.stringify(snapshot));
        }
        if (collection === 'characters' && !this.hydrating) {
          this.syncCharacter(event);
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

    this.hydrating = true;
    this.subscriptions.push(
      this.characters.list().subscribe({
        next: (items) => {
          for (const item of items) {
            this.store.set('characters', item);
          }
        },
        error: (e) => {
          console.error('[persistence] hydration failed', e);
          this.hydrating = false;
        },
        complete: () => {
          this.hydrating = false;
        },
      }),
    );
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
