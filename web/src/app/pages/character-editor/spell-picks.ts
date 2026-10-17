import { signal } from '@angular/core';

import { toggledIn } from '../../core/format/sets';
import type { CasterSection } from './class-blocks';

/** What `SpellPicks` reads from the page. */
export interface SpellPicksContext {
  /** The spell lists the sheet reads from, one per casting class. */
  readonly sections: () => readonly CasterSection[];
}

/**
 * The spells the "Magias" step has picked: the cantrips, the known spells and the prepared ones, and the search of
 * each list. Plain state the editor builds: the three sets move together (a spell leaving a wizard's spellbook is no
 * longer prepared), and the draft, the pickers and the save all read them.
 */
export class SpellPicks {
  readonly cantrips = signal<ReadonlySet<string>>(new Set());
  readonly known = signal<ReadonlySet<string>>(new Set());
  readonly prepared = signal<ReadonlySet<string>>(new Set());
  /** The search of each list, by `<section>:<list>`: "0:cantrips", "1:known", "1:prepared". */
  readonly filters = signal<Readonly<Record<string, string>>>({});

  constructor(private readonly host: SpellPicksContext) {}

  /** The saved sheet's picks (an edit). */
  load(cantrips: readonly string[], known: readonly string[], prepared: readonly string[]): void {
    this.cantrips.set(new Set(cantrips));
    this.known.set(new Set(known));
    this.prepared.set(new Set(prepared));
  }

  /** A reused page starts from nothing. */
  reset(): void {
    this.load([], [], []);
    this.filters.set({});
  }

  toggleCantrip(key: string): void {
    this.cantrips.set(toggledIn(this.cantrips(), key));
  }

  toggleKnown(key: string): void {
    const next = toggledIn(this.known(), key);
    this.known.set(next);
    // A spell leaving the spellbook is no longer prepared (a wizard prepares from the book only).
    const sections = this.host.sections();
    if (
      !next.has(key) &&
      sections.length > 0 &&
      sections.every((c) => c.preparation === 'spellbook') &&
      this.prepared().has(key)
    ) {
      this.prepared.set(toggledIn(this.prepared(), key));
    }
  }

  togglePrepared(key: string): void {
    this.prepared.set(toggledIn(this.prepared(), key));
  }

  /** "Escolher Rajada Mística agora": the spell becomes one of the sheet's cantrips. */
  addCantrip(key: string): void {
    if (!this.cantrips().has(key)) {
      this.cantrips.set(new Set([...this.cantrips(), key]));
    }
  }

  /** The search of a list (`<section>:<list>`, or the id of an extra list). */
  filterOf(id: string): string {
    return this.filters()[id] ?? '';
  }

  setFilter(id: string, value: string): void {
    this.filters.update((f) => ({ ...f, [id]: value }));
  }
}
