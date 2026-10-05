import { NgTemplateOutlet } from '@angular/common';
import { Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { Combatant, Encounter } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { CombatantState } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { combatantInitial, isDown, isPlayer, playerWord, stateWord } from '../../../../core/combat/combat-view';
import { isCreature, ofOwner } from '../../../../core/combat/creature-names';
import { type OrderItem, jointTurn, listNames, orderItems } from '../../../../core/combat/joint-turn';
import { conditionTags } from '../../../../core/combat/conditions';
import { coverMarkTags, sideTags } from '../../../../core/combat/cover';
import { CombatantTags } from '../combatant-tags/combatant-tags';
import { OrderGroup } from '../joint-turn/order-group';
import { PartState } from '../joint-turn/part-state';
import { CombatantToken } from '../../../../shared/combatant-token/combatant-token';

/**
 * "Ordem" (E6-14): the order of the turns as a column on the left, from
 * 1280px, numbered, with each one's token, name and one word: an NPC's state
 * ("Ileso", "Ferido"), "Jogador" for another player, "Você" for the person's
 * own, "✕ Derrotado"; a character at 0 hit points reads "Caída". The one on turn has the "Vez" pill on an `accent-soft`
 * row. Below 1280px the same is the strip (`order-strip`).
 */
@Component({
  selector: 'app-order-column',
  imports: [CombatantTags, CombatantToken, MatIconModule, NgTemplateOutlet, OrderGroup, PartState],
  template: `
    <section class="panel" aria-labelledby="order-col-title">
      <h2 class="panel__title" id="order-col-title">Ordem</h2>
      <ol class="rows">
        @for (item of items(); track key(item)) {
          @if (item.kind === 'group') {
            <li>
              <app-order-group class="dense" [total]="item.total" [onTurn]="item.onTurn" [label]="groupLabel(item)">
                <ol class="rows">
                  @for (c of item.members; track c.id) {
                    <ng-container *ngTemplateOutlet="rowTpl; context: { $implicit: c, grouped: true }" />
                  }
                </ol>
              </app-order-group>
            </li>
          } @else {
            <ng-container *ngTemplateOutlet="rowTpl; context: { $implicit: item.combatant, grouped: false }" />
          }
        }
      </ol>
      <ng-template #rowTpl let-c let-grouped="grouped">
        <li class="row" [class.row--turn]="current(c)">
          <span class="row__n" aria-hidden="true">{{ number(c) }}</span>
          <app-combatant-token [initial]="initial(c)" [npc]="npc(c)" [creature]="creature(c)" [defeated]="c.defeated" [mine]="c.mine" [current]="current(c)" [size]="30" />
          <span class="row__text">
            <span class="row__name" [class.row__name--out]="c.defeated">
              {{ c.label }}
              @if (current(c)) {<span class="row__turn">Vez</span>}
              @if (grouped && inTurn(c)) {<app-part-state [ended]="c.turnPartEnded" />}
            </span>
            <span class="row__word" [class.row__word--out]="c.defeated">
              @if (c.defeated) {<mat-icon aria-hidden="true">close</mat-icon>}{{ word(c) }}
            </span>
            <app-combatant-tags [names]="tags(c)" [label]="c.label" />
          </span>
        </li>
      </ng-template>
    </section>
  `,
  styleUrl: './order-column.scss',
})
export class OrderColumn {
  readonly encounter = input.required<Encounter>();

  protected initial(c: Combatant): string {
    return combatantInitial(c.label);
  }

  /** The conditions, "Aliado" and the master's cover mark under the name: what everyone who sees it may read. */
  protected tags(c: Combatant): string[] {
    return [...conditionTags(c), ...coverMarkTags(c)];
  }

  protected npc(c: Combatant): boolean {
    return !isPlayer(c) && !isCreature(c);
  }

  protected creature(c: Combatant): boolean {
    return isCreature(c);
  }

  /** Boxes only for groups that hold a player's character (RN-20). */
  protected readonly items = computed(() => orderItems(this.encounter(), false));
  private readonly joint = computed(() => jointTurn(this.encounter()));

  protected current(c: Combatant): boolean {
    return !this.joint() && c.id === this.encounter().currentCombatantId;
  }

  /** Its place in the whole order, from 1. */
  protected number(c: Combatant): number {
    return this.encounter().combatants.indexOf(c) + 1;
  }

  protected inTurn(c: Combatant): boolean {
    return !!this.joint()?.members.some((m) => m.id === c.id);
  }

  protected key(item: OrderItem): string {
    return item.kind === 'group' ? item.members.map((m) => m.id).join('+') : item.combatant.id;
  }

  protected groupLabel(item: OrderItem): string {
    return item.kind === 'group' ? `Turno conjunto: ${listNames(item.members.map((m) => m.label))}, iniciativa ${item.total}` : '';
  }

  protected word(c: Combatant): string {
    if (c.defeated) {
      return isPlayer(c) ? stateWord(CombatantState.DEAD, c.label) : 'Derrotado';
    }
    // A character at 0 hit points says so, even the person's own ("Você, caída").
    const down = isPlayer(c) && isDown(c) ? stateWord(c.state, c.label) : '';
    if (c.mine) {
      return down ? `Você, ${down.toLowerCase()}` : 'Você';
    }
    const word = isPlayer(c) ? playerWord(c) : stateWord(c.state);
    if (isCreature(c)) {
      // "da Sálvia · Ileso": whose it is and how hurt, in a word (RN-20: never its hit points).
      return [ofOwner(this.encounter(), c), word].filter(Boolean).join(' · ');
    }
    return sideTags(c).length > 0 ? `${word} · Aliado` : word;
  }
}
