import { Component, input, model } from '@angular/core';

import type { DiceOption } from '../../core/campaigns/dice-labels';

/**
 * The radio cards of RN-18, shared by the master's "Dados" panel (E6-17)
 * and the player's "Como você rola os dados" (E6-18): a title and one line
 * for each choice, the checked one outlined in the accent. Native radios
 * under the cards, so the keyboard (arrows) and screen readers work as on
 * any radio group.
 *
 * `disabled` locks the group (the player's view when the master decided for
 * everyone); `notes` replaces the line under an option then.
 */
@Component({
  selector: 'app-dice-choice',
  templateUrl: './dice-choice.html',
  styleUrl: './dice-choice.scss',
})
export class DiceChoice<T extends number> {
  /** Names the radio group; every instance on a page needs its own. */
  readonly name = input.required<string>();
  readonly label = input.required<string>();
  readonly options = input.required<readonly DiceOption<T>[]>();
  readonly value = model.required<T>();
  readonly disabled = input(false);
  /** Per-option replacement of the description, e.g. "Indisponível nesta
   * campanha.", keyed by option value. */
  readonly notes = input<Partial<Record<number, string>>>({});
  /** Options that show what is, but cannot be chosen (the "Personalizado" style): dashed, never the one that takes the click. */
  readonly inert = input<readonly T[]>([]);

  protected isInert(value: T): boolean {
    return this.inert().includes(value);
  }
}
