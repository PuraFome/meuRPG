import { Injectable } from '@angular/core';
import {
  DND_ALIGNMENTS,
  DND_BACKGROUNDS,
  DND_CLASSES,
  DND_HIT_DICE,
  DND_RACES,
} from './dnd-data';

export type DndOptionCategory =
  | 'race'
  | 'class'
  | 'background'
  | 'alignment'
  | 'hitDice';

const STANDARD_OPTIONS: Record<DndOptionCategory, string[]> = {
  race: DND_RACES.map((r) => r.name),
  class: DND_CLASSES.map((c) => c.name),
  background: [...DND_BACKGROUNDS],
  alignment: [...DND_ALIGNMENTS],
  hitDice: [...DND_HIT_DICE],
};

const STORAGE_KEY = 'meurpg_dnd_options';

function emptyCustom(): Record<DndOptionCategory, string[]> {
  return { race: [], class: [], background: [], alignment: [], hitDice: [] };
}

@Injectable({
  providedIn: 'root',
})
export class DndOptionsService {
  private custom = emptyCustom();

  constructor() {
    this.load();
  }

  /** All options for a category: standard D&D 5e + user-registered. */
  getOptions(category: DndOptionCategory): string[] {
    return [...STANDARD_OPTIONS[category], ...this.custom[category]];
  }

  /** Register a new custom option (e.g. expansion content), persisted to localStorage. */
  addOption(category: DndOptionCategory, value: string): void {
    const trimmed = value.trim();
    if (!trimmed) return;
    if (this.getOptions(category).includes(trimmed)) return;
    this.custom[category].push(trimmed);
    this.save();
  }

  private load(): void {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return;
    try {
      const parsed = JSON.parse(raw) as Partial<Record<DndOptionCategory, string[]>>;
      const loaded = emptyCustom();
      for (const category of Object.keys(loaded) as DndOptionCategory[]) {
        const values = parsed[category];
        if (Array.isArray(values)) {
          loaded[category] = values.filter(
            (v): v is string => typeof v === 'string' && v.trim() !== '',
          );
        }
      }
      this.custom = loaded;
    } catch {
      // ignore corrupt entries
    }
  }

  private save(): void {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(this.custom));
  }
}
