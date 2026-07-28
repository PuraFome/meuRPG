import { BaseRepository } from './base-repository';

export class LocalStorageRepository<T extends { id: string }> implements BaseRepository<T> {
  constructor(private readonly storageKey: string) {}

  async getAll(): Promise<T[]> {
    const raw = localStorage.getItem(this.storageKey);
    if (!raw) return [];
    try {
      return JSON.parse(raw) as T[];
    } catch {
      return [];
    }
  }

  async getById(id: string): Promise<T | null> {
    const items = await this.getAll();
    return items.find((item) => item.id === id) ?? null;
  }

  async create(item: T): Promise<T> {
    const items = await this.getAll();
    items.push(item);
    this.persist(items);
    return item;
  }

  async update(id: string, item: T): Promise<T> {
    const items = await this.getAll();
    const index = items.findIndex((i) => i.id === id);
    if (index !== -1) {
      items[index] = item;
      this.persist(items);
    }
    return item;
  }

  async delete(id: string): Promise<void> {
    const items = await this.getAll();
    const filtered = items.filter((item) => item.id !== id);
    this.persist(filtered);
  }

  private persist(items: T[]): void {
    localStorage.setItem(this.storageKey, JSON.stringify(items));
  }
}
