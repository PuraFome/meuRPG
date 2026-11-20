import { Injectable, computed, signal } from '@angular/core';

/**
 * What asks for the sheet "Teste de um NPC" without owning it: the button in the head of "Ações e rolagens", beside "Teste em grupo"
 * or "Pedir teste em grupo", and "Teste de Bartolo" in the "⋮" of a stage NPC. The master's host (`app-npc-check-master`) opens the
 * sheet and takes the request. `characterId` is the NPC to pick first (`''`: none chosen).
 */
@Injectable({ providedIn: 'root' })
export class NpcCheckLauncher {
  private readonly pending = signal<{ readonly characterId: string } | null>(null);
  /** A combat is open: the entry says so and waits, dashed (a refusal of the roll with `COMBAT_OPEN`). */
  readonly combatOpen = signal(false);
  /** The page shows a combat: set by the live session from what its stream says. */
  readonly pageCombat = signal(false);
  /**
   * The entry waits: the server refused a roll for a combat, and the page still shows one. When the page shows none (the combat
   * ended, or the page had not heard of it), the next click opens the sheet again and the server decides.
   */
  readonly blocked = computed(() => this.combatOpen() && this.pageCombat());
  readonly request = this.pending.asReadonly();

  ask(characterId = ''): void {
    this.pending.set({ characterId });
  }

  /** The host took the request. */
  take(): void {
    this.pending.set(null);
  }
}
