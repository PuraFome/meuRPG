import { Injectable, signal } from '@angular/core';

/**
 * What asks for the sheet "Teste de um NPC" without owning it: the button in the head of "Ações e rolagens", beside "Teste em grupo"
 * or "Pedir teste em grupo", and "Teste de Bartolo" in the "⋮" of a stage NPC. The master's host (`app-npc-check-master`) opens the
 * sheet and takes the request. `characterId` is the NPC to pick first (`''`: none chosen).
 */
@Injectable({ providedIn: 'root' })
export class NpcCheckLauncher {
  private readonly pending = signal<{ readonly characterId: string } | null>(null);
  /**
   * The result of the last roll request: set by a refusal for an open combat, cleared when a roll goes through.
   * The entry shows it, dashed, and stays usable: the server decides.
   */
  readonly combatOpen = signal(false);
  readonly request = this.pending.asReadonly();

  ask(characterId = ''): void {
    this.pending.set({ characterId });
  }

  /** The host took the request. */
  take(): void {
    this.pending.set(null);
  }
}
