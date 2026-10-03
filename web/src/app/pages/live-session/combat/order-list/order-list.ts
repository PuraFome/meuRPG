import { Component, ElementRef, computed, effect, input, output, signal, viewChild } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatMenuModule } from '@angular/material/menu';

import { type Combatant, type Encounter, EncounterStatus } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { combatantInitial, isPlayer } from '../../../../core/combat/combat-view';
import { CombatantToken } from '../../../../shared/combatant-token/combatant-token';
import type { CombatantInfo } from '../combat-info';

/**
 * The master's order of initiative while the combat runs (E6-11, E6-12): who
 * goes when, with the hit points the master sees, the "Vez" row, the
 * reveal/hide switch of each NPC, and "Dano/Cura" (the E5-05 adjust dialog)
 * for a player's character. An NPC's own damage comes with the actions.
 * "Remover do combate" asks in place, with the focus on "Voltar".
 */
@Component({
  selector: 'app-order-list',
  imports: [CombatantToken, MatButtonModule, MatIconModule, MatMenuModule],
  templateUrl: './order-list.html',
  styleUrl: './order-list.scss',
})
export class OrderList {
  readonly encounter = input.required<Encounter>();
  readonly info = input<ReadonlyMap<string, CombatantInfo>>(new Map());
  /** Whether "Dano/Cura" can open for this character (a living player). */
  readonly adjustable = input<ReadonlySet<string>>(new Set());
  readonly busy = input(false);

  readonly adjust = output<string>();
  readonly reveal = output<{ id: string; hidden: boolean }>();
  readonly remove = output<string>();
  readonly add = output<void>();

  protected readonly removing = signal<string | null>(null);
  private readonly back = viewChild('back', { read: ElementRef<HTMLButtonElement> });
  protected readonly rows = computed(() => this.encounter().combatants);
  /** Everything is read-only after the combat ends. */
  protected readonly live = computed(() => this.encounter().status !== EncounterStatus.ENDED);

  constructor() {
    // Opening the confirmation puts the focus on the safe button.
    effect(() => this.back()?.nativeElement.focus());
  }

  protected initial(c: Combatant): string {
    return combatantInitial(c.label);
  }

  protected npc(c: Combatant): boolean {
    return !isPlayer(c);
  }

  protected sub(c: Combatant): string {
    const info = this.info().get(c.characterId);
    if (isPlayer(c)) {
      return info?.classSummary ?? '';
    }
    return info?.kindLabel ?? 'NPC';
  }

  protected percent(c: Combatant): number {
    const max = c.hitPointsMax ?? 0;
    return max > 0 ? Math.min(100, Math.round(((c.hitPointsCurrent ?? 0) / max) * 100)) : 0;
  }

  protected current(c: Combatant): boolean {
    return c.id === this.encounter().currentCombatantId;
  }

  protected canAdjust(c: Combatant): boolean {
    return isPlayer(c) && this.adjustable().has(c.characterId);
  }

  protected ask(id: string): void {
    this.removing.set(id);
  }

  protected confirmRemove(id: string): void {
    this.removing.set(null);
    this.remove.emit(id);
  }
}
