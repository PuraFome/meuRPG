import { Component, computed, input, output, signal } from '@angular/core';
import type { Combatant } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { metersFixed, reachSquares, squaresText } from '../../../../core/units';
import { joinDots, tight } from '../../../../core/format/text';
import { leftSentence, passNote } from '../../../../core/combat/joint-turn';
import type { MineTab } from '../../../../core/combat/mine';
import { EndPart } from '../joint-turn/end-part';
import { MineTabs } from '../mine-tabs/mine-tabs';
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
  imports: [EndPart, EndTurn, MineTabs],
  template: `
    @if (tabs().length > 1) {
      <app-mine-tabs [tabs]="tabs()" [selected]="selected()" (select)="select.emit($event)" />
    }
    @if (own(); as me) {
    @if (waiting()) {
      <p class="what">{{ waiting() }}: a vez continua quando responderem.</p>
    } @else if (!asking()) {
    <p class="what">{{ joint() ? 'Ainda disponível na sua parte' : 'Ainda disponível neste turno' }}</p>
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
    }
    @if (joint(); as who) {
      @if (!asking()) {
        <p class="what">{{ note(who) }}</p>
      }
      <app-end-part [left]="partLeft()" [busy]="busy()" (endPart)="endTurn.emit()" (asked)="asking.set($event)" />
    } @else {
      <app-end-turn [own]="me" [attacksLeft]="attacksLeft()" [busy]="busy()" [waiting]="waiting()" [block]="true" (endTurn)="endTurn.emit()" />
    }
    }
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
  /** The player's own character while it acts; `null` when only the tabs are drawn (it is not its turn, or a creature's page is open). */
  readonly own = input<Combatant | null>(null);
  /** The tabs of what the player plays (MR-037, E9-12), drawn above when there is more than one. */
  readonly tabs = input<readonly MineTab[]>([]);
  readonly selected = input('');
  readonly busy = input(false);
  /** Extra Attack: the attacks that remain once the action is spent. */
  readonly attacksLeft = input(0);
  /** An opportunity attack waits for an answer: the list of what is left and the end button say so instead. */
  readonly waiting = input('');

  /** In a joint turn, who else must end their part (labels; "o mestre" is the hidden one's). */
  readonly joint = input<readonly string[] | null>(null);

  readonly endTurn = output<void>();
  readonly select = output<string>();

  protected readonly asking = signal(false);
  protected readonly partLeft = computed(() => {
    const c = this.own();
    return c ? leftSentence(c) : '';
  });

  /** "O turno passa quando você e a Brisa encerrarem." */
  protected note(others: readonly string[]): string {
    return passNote(others.filter((o) => o !== 'o mestre'), others.includes('o mestre'));
  }

  protected readonly left = computed(() => {
    const c = this.own();
    if (!c) {
      return [];
    }
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
      items.push({ name: 'Mover', amount: tight(joinDots([metersFixed(c.movementLeftDft / 10), squaresText(reachSquares(c.movementLeftFt))])) });
    }
    return items;
  });
}
