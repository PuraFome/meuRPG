import { NgTemplateOutlet } from '@angular/common';
import { Component, computed, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { AbilityResult } from '../../../../core/dice/dice';

export type DiceResultState = 'free' | 'chosen' | 'placed';

/** "6, 6, 5 e 2". */
function listWords(items: readonly number[]): string {
  return items.length < 2
    ? items.join('')
    : `${items.slice(0, -1).join(', ')} e ${items[items.length - 1]}`;
}

/**
 * One rolled ability score (E6-20): the total, the four dice with the lowest
 * struck through, and a state line in words, never by colour alone: "em
 * Força" with a check, "Livre", or "Escolhido". A standard-array value has
 * no dice. Read as a whole by screen readers ("17: dados 6, 6, 5 e 2; o 2
 * foi descartado. Em Força."), so the little squares are decoration.
 *
 * On a phone it is a button (`interactive`, with `aria-pressed`) the player
 * taps before tapping an ability; on a desktop it only reports.
 */
@Component({
  selector: 'app-dice-result',
  imports: [MatIconModule, NgTemplateOutlet],
  templateUrl: './dice-result.html',
  styleUrl: './dice-result.scss',
})
export class DiceResult {
  readonly result = input.required<AbilityResult>();
  readonly state = input.required<DiceResultState>();
  /** The ability's name while placed: "Força". */
  readonly placedOn = input<string | null>(null);
  readonly interactive = input(false);
  readonly pressed = output<void>();

  protected readonly stateText = computed(() => {
    switch (this.state()) {
      case 'placed':
        return `em ${this.placedOn()}`;
      case 'chosen':
        return 'Escolhido';
      default:
        return 'Livre';
    }
  });

  protected readonly label = computed(() => {
    const { total, dice, dropped } = this.result();
    const dieWords =
      dice.length > 0 ? `: dados ${listWords(dice)}; o ${dice[dropped]} foi descartado` : '';
    const state = this.stateText();
    return `${total}${dieWords}. ${state.charAt(0).toUpperCase()}${state.slice(1)}.`;
  });
}
