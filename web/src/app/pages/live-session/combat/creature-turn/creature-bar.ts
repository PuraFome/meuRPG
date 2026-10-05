import { Component, computed, input, output, signal } from '@angular/core';

import { endLabel, partLeftSentence } from '../../../../core/combat/creature-names';
import { type MineTab } from '../../../../core/combat/mine';
import { EndPart } from '../joint-turn/end-part';
import { MineTabs } from '../mine-tabs/mine-tabs';
import { EndTurn } from '../turn-panel/end-turn';

/**
 * The bar pinned to the bottom of the page of a creature's turn, on a phone and a tablet (MR-037, E9-12 states 3
 * and 6): the tabs of what the player plays and, when the creatures act, the one button that ends their turn
 * ("Encerrar a parte dos Lobos", "Encerrar a vez do Nanquim"), outlined, 48 px and the full width. From 1024px
 * the tabs sit above the page and the button is in the card (`CreatureHero`), so this bar is not drawn.
 */
@Component({
  selector: 'app-creature-bar',
  imports: [EndPart, EndTurn, MineTabs],
  template: `
    <app-mine-tabs [tabs]="tabs()" [selected]="tab().id" [inert]="asking()" (select)="select.emit($event)" />
    @if (tab().state === 'turn') {
      @if (tab().members.length > 1) {
        <app-end-part
          [label]="label()"
          [heading]="label() + '?'"
          [confirmLabel]="label()"
          warning="Não dá para reabrir esta parte depois."
          [detail]="left()"
          [busy]="busy()"
          (asked)="asking.set($event)"
          (endPart)="endPart.emit()"
        />
      } @else {
        <app-end-turn [own]="tab().members[0]" [label]="label()" [busy]="busy()" [waiting]="waiting()" [block]="true" (endTurn)="endPart.emit()" />
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
  `,
})
export class CreatureBar {
  readonly tabs = input.required<readonly MineTab[]>();
  readonly tab = input.required<MineTab>();
  readonly busy = input(false);
  readonly waiting = input('');

  readonly select = output<string>();
  readonly endPart = output<void>();

  protected readonly label = computed(() => endLabel(this.tab().members));
  /** The question to end the part is open: the tabs are inert until it is answered. */
  protected readonly asking = signal(false);
  protected readonly left = computed(() => partLeftSentence(this.tab().acting));
}
