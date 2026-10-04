import { Component, computed, input, output } from '@angular/core';
import type { Combatant } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { distanceText } from '../../../../core/units';
import { EndTurn } from './end-turn';

/**
 * The bar pinned to the bottom of a player's own turn on a phone and a
 * tablet (E6-06): what is still available this turn, as open circles and
 * words, and "Encerrar turno", which is an outline until the action and the
 * bonus action are spent (then it is the filled button). It sits at the end
 * of the page's column so that it sticks to the bottom of the screen while
 * the page scrolls. From 1024px the button is in the turn card instead.
 */
@Component({
  selector: 'app-turn-bar',
  imports: [EndTurn],
  template: `
    <p class="what">Ainda disponível neste turno</p>
    <ul class="left">
      @for (item of left(); track item.name) {
        <li>
          <span class="dot" aria-hidden="true"></span>
          <span>{{ item.name }}@if (item.amount) {&nbsp;<b>{{ item.amount }}</b>}</span>
        </li>
      } @empty {
        <li>Nada: só falta encerrar o turno.</li>
      }
    </ul>
    <app-end-turn [own]="own()" [attacksLeft]="attacksLeft()" [busy]="busy()" [block]="true" (endTurn)="endTurn.emit()" />
  `,
  styles: `
    :host {
      position: sticky;
      bottom: 0;
      z-index: 5;
      display: flex;
      flex-direction: column;
      gap: 8px;
      margin: 0 calc(var(--mr-gutter) * -1);
      padding: 10px var(--mr-gutter) 12px;
      border-top: 1px solid var(--mr-line);
      background: var(--mr-surface);

      @media (min-width: 1024px) {
        display: none;
      }
    }

    .what {
      margin: 0;
      font-size: 14px;
      color: var(--mr-ink-muted);
    }

    // Two columns, so "Mover 7,5 m · 5 quadrados" never wraps in the middle of
    // a number: it takes the cell, and the dot stays at the end of its line.
    .left {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 4px 16px;
      margin: 0;
      padding: 0;
      list-style: none;
      font-size: 15px;
      font-weight: 500;

      li {
        display: flex;
        align-items: center;
        gap: 6px;
      }
    }

    .dot {
      flex: none;
      box-sizing: border-box;
      width: 16px;
      height: 16px;
      border: 2px solid var(--mr-ink);
      border-radius: 50%;
    }
  `,
})
export class TurnBar {
  /** The player's own combatant. */
  readonly own = input.required<Combatant>();
  readonly busy = input(false);
  /** Extra Attack: the attacks that remain once the action is spent. */
  readonly attacksLeft = input(0);

  readonly endTurn = output<void>();

  protected readonly left = computed(() => {
    const c = this.own();
    const items: { name: string; amount?: string }[] = [];
    if (!c.actionUsed) {
      items.push({ name: 'Ação' });
    } else if (this.attacksLeft() > 0) {
      items.push({ name: `${this.attacksLeft()} ${this.attacksLeft() === 1 ? 'ataque' : 'ataques'}` });
    }
    if (!c.bonusActionUsed) {
      items.push({ name: 'Ação bônus' });
    }
    if (!c.reactionUsed) {
      items.push({ name: 'Reação' });
    }
    if (c.movementLeftFt > 0) {
      items.push({ name: 'Mover', amount: distanceText(c.movementLeftFt) });
    }
    return items;
  });
}
