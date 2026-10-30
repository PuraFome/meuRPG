import { ChangeDetectionStrategy, Component, input, model } from '@angular/core';

import type { RollDie } from '../../../gen/meurpg/rules/v1/rules_pb';
import { rollDieCost } from '../../core/effects/roll-die';
import { OptionCard } from '../option-card/option-card';

/**
 * "Usar <nome>" on a roll sheet (MR-025): a die the character's table features add to this roll (a maneuver on a check, a
 * save or the initiative). Only the dice that fit the roll are given, so nothing shows when none does. One checkbox for each;
 * a roll takes one die, so ticking one unticks the other. The cost is written under the name. Space ticks.
 */
@Component({
  selector: 'app-roll-die-offer',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [OptionCard],
  template: `
    @if (dice().length > 0) {
      <fieldset class="offer">
        <legend class="mr-visually-hidden">Dados das suas características para esta jogada</legend>
        @for (d of dice(); track d.key) {
          <app-option-card
            [title]="'Usar ' + d.namePt"
            [sub]="cost(d)"
            [checked]="picked() === d.key"
            (chosen)="picked.set(picked() === d.key ? '' : d.key)"
          />
        }
      </fieldset>
    }
  `,
  styles: `
    :host {
      display: block;
    }

    .offer {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-2);
      margin: 0;
      padding: 0;
      border: 0;
    }
  `,
})
export class RollDieOffer {
  /** The dice that fit this roll (`rollDiceFor`). */
  readonly dice = input<readonly RollDie[]>([]);
  /** The key of the die ticked; empty for none. */
  readonly picked = model('');

  protected readonly cost = rollDieCost;
}
