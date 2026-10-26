import { ChangeDetectionStrategy, Component, input, model } from '@angular/core';

import type { RollDie } from '../../../gen/meurpg/rules/v1/rules_pb';
import { rollDieCost } from '../../core/effects/roll-die';

let nextId = 0;

/**
 * "Usar <nome>" on a roll sheet (MR-025): a die the character's table features add to this roll (a maneuver on a check, a
 * save or the initiative). Only the dice that fit the roll are given, so nothing shows when none does. One checkbox for each;
 * a roll takes one die, so ticking one unticks the other. The cost is written under the name. Space ticks.
 */
@Component({
  selector: 'app-roll-die-offer',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (dice().length > 0) {
      <fieldset class="offer">
        <legend class="mr-visually-hidden">Dados das suas características para esta jogada</legend>
        @for (d of dice(); track d.key) {
          <label class="offer__row" [class.offer__row--on]="picked() === d.key">
            <input
              type="checkbox"
              [checked]="picked() === d.key"
              [attr.aria-describedby]="id + '-' + $index"
              (change)="picked.set(picked() === d.key ? '' : d.key)"
            />
            <span class="offer__text">
              <b>Usar {{ d.namePt }}</b>
              <span class="offer__sub" [id]="id + '-' + $index">{{ cost(d) }}</span>
            </span>
          </label>
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

    .offer__row {
      display: flex;
      align-items: flex-start;
      gap: var(--mr-space-3);
      box-sizing: border-box;
      min-height: 44px;
      padding: 8px 14px;
      border: 1px solid var(--mr-control-line);
      border-radius: var(--mr-radius-md);
      cursor: pointer;

      &--on {
        padding: 7px 13px;
        border: 2px solid var(--mr-accent);
        background: var(--mr-accent-soft);
      }

      &:has(input:focus-visible) {
        outline: 2px solid var(--mr-focus);
        outline-offset: 2px;
      }
    }

    .offer__text {
      display: flex;
      flex-direction: column;
    }

    .offer__sub {
      font-size: 14px;
      line-height: 19px;
      color: var(--mr-ink-muted);
    }
  `,
})
export class RollDieOffer {
  /** The dice that fit this roll (`rollDiceFor`). */
  readonly dice = input<readonly RollDie[]>([]);
  /** The key of the die ticked; empty for none. */
  readonly picked = model('');

  protected readonly id = `roll-die-${nextId++}`;
  protected readonly cost = rollDieCost;
}
