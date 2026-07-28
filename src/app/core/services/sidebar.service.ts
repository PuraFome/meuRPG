import { Injectable } from '@angular/core';
import { BehaviorSubject, Observable } from 'rxjs';
import type { SidebarItem } from '../models/shared';

@Injectable({
  providedIn: 'root',
})
export class SidebarService {
  private itemsSubject = new BehaviorSubject<SidebarItem[]>([]);

  /** Observable of sidebar items. */
  items$: Observable<SidebarItem[]> = this.itemsSubject.asObservable();

  /** Set the full sidebar item list. */
  setItems(items: SidebarItem[]): void {
    this.itemsSubject.next(items);
  }

  /** Add a single item to the sidebar. */
  addItem(item: SidebarItem): void {
    this.itemsSubject.next([...this.itemsSubject.value, item]);
  }

  /** Remove an item by id. */
  removeItem(id: string): void {
    const filtered = this.itemsSubject.value.filter((item) => item.id !== id);
    this.itemsSubject.next(filtered);
  }

  /** Update badge count for an item. */
  setBadge(id: string, badge: number | undefined): void {
    const items = this.itemsSubject.value.map((item) =>
      item.id === id ? { ...item, badge } : item,
    );
    this.itemsSubject.next(items);
  }

  /** Get current sidebar items synchronously. */
  snapshot(): SidebarItem[] {
    return this.itemsSubject.value;
  }
}
