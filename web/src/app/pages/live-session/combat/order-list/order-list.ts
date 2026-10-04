import { NgTemplateOutlet } from '@angular/common';
import { Component, ElementRef, computed, effect, inject, input, output, signal, viewChild } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatMenuModule } from '@angular/material/menu';

import { type Combatant, CombatantState, type Encounter, EncounterStatus } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { joinDots } from '../../../../core/format/text';
import { conditionTags } from '../../../../core/combat/conditions';
import { combatantInitial, isPlayer, stateWord } from '../../../../core/combat/combat-view';
import { type OrderItem, jointTurn, listNames, orderItems } from '../../../../core/combat/joint-turn';
import { CombatantToken } from '../../../../shared/combatant-token/combatant-token';
import type { CombatantInfo } from '../combat-info';
import { CombatantTags } from '../combatant-tags/combatant-tags';
import { DeathRow } from '../death-saves/death-marks';
import { OrderGroup } from '../joint-turn/order-group';
import { PartState } from '../joint-turn/part-state';

/**
 * The master's order of initiative while the combat runs (E6-11, E6-12): who
 * goes when, with the hit points the master sees, the "Vez" row, the
 * reveal/hide switch of each NPC, and "Dano/Cura" (the E5-05 adjust dialog)
 * for a player's character. An NPC's own damage comes with the actions.
 * "Remover do combate" asks in place, with the focus on "Voltar". The ⋮ menu of
 * every row has "Condições…" (E6-29); under the name stand the condition tags
 * and the spell it concentrates on, and a character at 0 hit points shows its
 * word ("Caída", "Estável", "Morrendo · 3 falhas" on the danger surface, "✕
 * Morta") and the marks of its death saves (E6-30).
 */
@Component({
  selector: 'app-order-list',
  imports: [CombatantTags, CombatantToken, DeathRow, MatButtonModule, MatIconModule, MatMenuModule, NgTemplateOutlet, OrderGroup, PartState],
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
  /** "Dano/Cura" on an NPC: its combatant ID (`AdjustCombatantHitPoints`). */
  readonly adjustNpc = output<string>();
  readonly reveal = output<{ id: string; hidden: boolean }>();
  readonly remove = output<string>();
  readonly add = output<void>();
  /** "Condições…": the combatant's ID. */
  readonly conditions = output<string>();
  /** The characters whose "Confirmar a morte" question the master put away. */
  readonly deathDismissed = input<ReadonlySet<string>>(new Set());
  /** "Confirmar a morte": the question again. */
  readonly askDeath = output<string>();

  protected readonly Stable = CombatantState.STABLE;
  protected readonly removing = signal<string | null>(null);
  private readonly back = viewChild('back', { read: ElementRef<HTMLButtonElement> });
  protected readonly rows = computed(() => this.encounter().combatants);
  /** The order with the boxes of the joint turns (the master has every total). */
  protected readonly items = computed(() => orderItems(this.encounter(), true));
  private readonly joint = computed(() => jointTurn(this.encounter()));
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

  protected tags(c: Combatant): string[] {
    return conditionTags(c);
  }

  /** The spell it concentrates on, written out (the combat sends its name). */
  protected concentration(c: Combatant): string {
    return c.concentrationSpellNamePt;
  }

  /** A player's character at 0 hit points that is still in the story. */
  protected down(c: Combatant): boolean {
    return c.state === CombatantState.DOWN || c.state === CombatantState.DYING || c.state === CombatantState.STABLE;
  }

  protected dying(c: Combatant): boolean {
    return c.state === CombatantState.DYING;
  }

  /** "Caída", "Estável", "Morrendo · 3 falhas". */
  protected downText(c: Combatant): string {
    return this.dying(c) ? joinDots(['Morrendo', `${c.deathFailures} falhas`]) : stateWord(c.state, c.label);
  }

  protected word(c: Combatant): string {
    return stateWord(c.defeated && !this.npc(c) ? CombatantState.DEAD : c.state, c.label);
  }

  protected sub(c: Combatant): string {
    const info = this.info().get(c.characterId);
    const ac = c.armorClass === undefined ? '' : `CA ${c.armorClass}`;
    const first = isPlayer(c) ? (info?.classSummary ?? '') : (info?.kindLabel ?? 'NPC');
    return [first, ac].filter(Boolean).join(' · ');
  }

  protected percent(c: Combatant): number {
    const max = c.hitPointsMax ?? 0;
    return max > 0 ? Math.min(100, Math.round(((c.hitPointsCurrent ?? 0) / max) * 100)) : 0;
  }

  /** The row of the one on turn; in a joint turn the box is on turn instead. */
  protected current(c: Combatant): boolean {
    return !this.joint() && c.id === this.encounter().currentCombatantId;
  }

  /** The combatant is a member of the joint turn that is running. */
  protected inTurn(c: Combatant): boolean {
    return !!this.joint()?.members.some((m) => m.id === c.id);
  }

  protected key(item: OrderItem): string {
    return item.kind === 'group' ? item.members.map((m) => m.id).join('+') : item.combatant.id;
  }

  /** "Turno conjunto: Brisa e Toren, iniciativa 19", for a screen reader. */
  protected groupLabel(item: OrderItem): string {
    return item.kind === 'group' ? `Turno conjunto: ${listNames(item.members.map((m) => m.label))}, iniciativa ${item.total}` : '';
  }

  protected canAdjust(c: Combatant): boolean {
    return isPlayer(c) && this.adjustable().has(c.characterId);
  }

  /** An NPC's hit points are the master's to change, always (a defeated one
   * healed above 0 comes back). */
  protected canAdjustNpc(c: Combatant): boolean {
    return !isPlayer(c) && c.hitPointsMax !== undefined;
  }

  protected ask(id: string): void {
    this.removing.set(id);
  }

  protected confirmRemove(id: string): void {
    this.removing.set(null);
    this.remove.emit(id);
  }
}
