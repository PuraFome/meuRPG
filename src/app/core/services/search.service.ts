import { Injectable } from '@angular/core';
import { BehaviorSubject, Observable } from 'rxjs';
import { map } from 'rxjs/operators';
import type { SearchResult } from '../models/shared';
import type { EntityType } from '../models/shared';

@Injectable({
  providedIn: 'root',
})
export class SearchService {
  private index = new BehaviorSubject<SearchResult[]>([]);

  /** Observable of all indexed entities. */
  index$: Observable<SearchResult[]> = this.index.asObservable();

  /** Add or update an entity in the search index. */
  indexEntity(result: SearchResult): void {
    const current = this.index.value;
    const existing = current.findIndex((r) => r.id === result.id && r.type === result.type);
    if (existing !== -1) {
      const updated = [...current];
      updated[existing] = result;
      this.index.next(updated);
    } else {
      this.index.next([...current, result]);
    }
  }

  /** Remove an entity from the index. */
  removeEntity(id: string, type: EntityType): void {
    const filtered = this.index.value.filter((r) => !(r.id === id && r.type === type));
    this.index.next(filtered);
  }

  /** Clear the entire index. */
  clear(): void {
    this.index.next([]);
  }

  /** Search entities by query text (matches name, description). */
  search(query: string): Observable<SearchResult[]> {
    const q = query.toLowerCase().trim();
    if (!q) {
      return this.index.asObservable().pipe(map((results) => results.slice(0, 50)));
    }
    return this.index.asObservable().pipe(
      map((results) =>
        results
          .filter(
            (r) =>
              r.name.toLowerCase().includes(q) ||
              r.description.toLowerCase().includes(q),
          )
          .sort((a, b) => b.score - a.score),
      ),
    );
  }

  /** Get indexed entities filtered by type. */
  getByType(type: EntityType): Observable<SearchResult[]> {
    return this.index.asObservable().pipe(
      map((results) => results.filter((r) => r.type === type)),
    );
  }

  /** Snapshot of current index. */
  snapshot(): SearchResult[] {
    return this.index.value;
  }

  searchGrouped(query: string): Observable<SearchResultGroup[]> {
    const typeOrder: EntityType[] = [
      'character',
      'campaign',
      'gallery',
      'rules',
      'map',
      'session',
    ];
    const typeLabels: Record<EntityType, string> = {
      character: 'Personagens',
      campaign: 'Campanha',
      gallery: 'Galeria',
      map: 'Mapas',
      session: 'Sessões',
      rules: 'Regras',
    };
    const typeIcons: Record<EntityType, string> = {
      character: 'person',
      campaign: 'folder',
      gallery: 'collections_bookmark',
      map: 'map',
      session: 'event',
      rules: 'menu_book',
    };

    return this.search(query).pipe(
      map((results) => {
        const grouped = new Map<EntityType, SearchResult[]>();
        for (const result of results) {
          const group = grouped.get(result.type);
          if (group) {
            group.push(result);
          } else {
            grouped.set(result.type, [result]);
          }
        }
        return typeOrder
          .filter((type) => grouped.has(type))
          .map((type) => ({
            type,
            label: typeLabels[type],
            icon: typeIcons[type],
            items: grouped.get(type)!,
          }));
      }),
    );
  }
}

export interface SearchResultGroup {
  type: EntityType;
  label: string;
  icon: string;
  items: SearchResult[];
}
