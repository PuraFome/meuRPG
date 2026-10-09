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
 * tablet (E6-06): one slim row with the movement still left and "Encerrar
 * turno", which is an outline until the action and the bonus action are spent
 * (then it is the filled button). What else is available this turn is in the
 * tiles above, so the bar does not repeat it and leaves the initiative cards
 * visible. It sits at the end of the page's column so that it sticks to the
 * bottom of the screen while the page scrolls. From 1024px the button is in
 * the turn card instead.
 */
@Component({
  selector: 'app-turn-bar',
  imports: [EndPart, EndTurn, MineTabs],
  template: `
    @if (tabs().length > 1) {
      <app-mine-tabs [tabs]="tabs()" [selected]="selected()" (select)="select.emit($event)" />
    }
    @if (own(); as me) {
      @if (joint(); as who) {
        @if (!asking()) {
          <p class="what">{{ note(who) }}</p>
        }
        <app-end-part [left]="partLeft()" [busy]="busy()" (endPart)="endTurn.emit()" (asked)="asking.set($event)" />
      } @else {
        <div class="row">
          @if (waiting()) {
            <p class="what">{{ waiting() }}: a vez continua quando responderem.</p>
          } @else if (move(); as amount) {
            <p class="what">Mover&nbsp;<b>{{ amount }}</b></p>
          }
          <app-end-turn class="row__end" [own]="me" [attacksLeft]="attacksLeft()" [busy]="busy()" [waiting]="waiting()" [label]="surprised() ? 'Passar o turno' : 'Encerrar turno'" [surprised]="surprised()" (endTurn)="endTurn.emit()" />
        </div>
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
      padding: 8px var(--mr-gutter);
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

      b {
        color: var(--mr-ink);
      }
    }

    // One slim row: the movement left, when there is one, and "Encerrar turno" at the end. The
    // question "Encerrar mesmo?" takes the whole row while it is open.
    .row {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: 8px 12px;

      .what {
        flex: 1 1 0;
        min-width: 0;
      }

      &:has([role='alertdialog']) .what {
        display: none;
      }
    }

    .row__end {
      flex: none;
      margin-left: auto;

      &:has([role='alertdialog']) {
        flex: 1 1 100%;
      }
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
  /** The character is surprised (W7-X): the button only passes the turn. */
  readonly surprised = input(false);
  /** No map (RN-25): the movement is a number of meters, said as "Movimento 9,0 m". */
  readonly theatre = input(false);

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
    return passNote(
      others.filter((o) => o !== 'o mestre'),
      others.includes('o mestre'),
    );
  }

  /** "7,5 m · 5 quadrados": the movement still left; empty without a map (the Movimento tile says it) or when none is left. */
  protected readonly move = computed(() => {
    const c = this.own();
    if (!c || c.movementLeftFt <= 0 || this.theatre()) {
      return '';
    }
    return tight(
      joinDots([metersFixed(c.movementLeftDft / 10), squaresText(reachSquares(c.movementLeftFt))]),
    );
  });
}
