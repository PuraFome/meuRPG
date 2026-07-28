import { Injectable } from '@angular/core';
import { BehaviorSubject, Observable, Subject } from 'rxjs';
import { map } from 'rxjs/operators';

export interface StoreEvent {
  type: 'created' | 'updated' | 'deleted';
  collection: string;
  id: string;
  payload?: unknown;
}

@Injectable({
  providedIn: 'root',
})
export class StoreService<T extends { id: string }> {
  private collections = new Map<string, BehaviorSubject<Map<string, T>>>();
  private eventsSubject = new Subject<StoreEvent>();

  /** Subscribe to cross-module store events. */
  events$: Observable<StoreEvent> = this.eventsSubject.asObservable();

  private getSubject(collection: string): BehaviorSubject<Map<string, T>> {
    if (!this.collections.has(collection)) {
      this.collections.set(collection, new BehaviorSubject(new Map<string, T>()));
    }
    return this.collections.get(collection)!;
  }

  /** Observe all items in a collection. */
  getAll(collection: string): Observable<T[]> {
    return this.getSubject(collection).pipe(map((map) => Array.from(map.values())));
  }

  /** Observe a single item by id. */
  get(collection: string, id: string): Observable<T | undefined> {
    return this.getSubject(collection).pipe(map((map) => map.get(id)));
  }

  /** Set (create or overwrite) an item. */
  set(collection: string, item: T): void {
    const subject = this.getSubject(collection);
    const current = new Map(subject.value);
    current.set(item.id, item);
    subject.next(current);
    this.eventsSubject.next({ type: 'created', collection, id: item.id, payload: item });
  }

  /** Replace an existing item. */
  update(collection: string, id: string, item: T): void {
    const subject = this.getSubject(collection);
    const current = new Map(subject.value);
    current.set(id, item);
    subject.next(current);
    this.eventsSubject.next({ type: 'updated', collection, id, payload: item });
  }

  /** Partially update an existing item. */
  patch(collection: string, id: string, partial: Partial<T>): void {
    const subject = this.getSubject(collection);
    const current = new Map(subject.value);
    const existing = current.get(id);
    if (existing) {
      const updated = { ...existing, ...partial };
      current.set(id, updated);
      subject.next(current);
      this.eventsSubject.next({ type: 'updated', collection, id, payload: updated });
    }
  }

  /** Delete an item by id. */
  delete(collection: string, id: string): void {
    const subject = this.getSubject(collection);
    const current = new Map(subject.value);
    current.delete(id);
    subject.next(current);
    this.eventsSubject.next({ type: 'deleted', collection, id });
  }

  /** Subscribe to collection changes with a callback (returns unsubscribe handle). */
  subscribe(collection: string, callback: (items: T[]) => void): { unsubscribe: () => void } {
    const subscription = this.getAll(collection).subscribe(callback);
    return { unsubscribe: () => subscription.unsubscribe() };
  }

  /** Synchronous snapshot of a collection. */
  snapshot(collection: string): T[] {
    const subject = this.getSubject(collection);
    return Array.from(subject.value.values());
  }
}
