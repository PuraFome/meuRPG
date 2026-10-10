import { Injectable, signal } from '@angular/core';

/**
 * Counts the master's "Desfazer última ação" that worked. A card that shows the result of the last roll ("2d20 (8,19)+4 = 12
 * Errou") clears itself when the count moves: the roll it shows may be the very action that was undone, and the result must
 * not outlive it. Root-provided, with no client behind it.
 */
@Injectable({ providedIn: 'root' })
export class CombatUndone {
  readonly count = signal(0);

  /** An undo worked. */
  mark(): void {
    this.count.update((n) => n + 1);
  }
}
