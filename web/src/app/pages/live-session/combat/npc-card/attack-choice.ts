import { Component, input, output } from '@angular/core';

import type { Attack } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { attackDetail, attackTitle } from '../../../../core/combat/combat-options';

/** The NPC's attacks as radio rows (E6-11): "Cimitarra +5" and under it the
 * dice, the type and the reach. Native radios under the cards, so the arrows
 * and screen readers work as in any radio group. */
@Component({
  selector: 'app-attack-choice',
  template: `
    <div class="atks" role="radiogroup" [attr.aria-label]="label()">
      @for (a of attacks(); track a.key) {
        <label class="atk">
          <input
            type="radio"
            class="mr-visually-hidden"
            name="npc-attack"
            [value]="a.key"
            [checked]="a.key === selected()"
            (change)="pick.emit(a.key)"
          />
          <span class="atk__dot" aria-hidden="true"></span>
          <span class="atk__text">
            <b class="atk__name">{{ title(a) }}</b>
            <span class="atk__detail">{{ detail(a) }}</span>
          </span>
        </label>
      }
    </div>
  `,
  styleUrl: './attack-choice.scss',
})
export class AttackChoice {
  readonly attacks = input.required<readonly Attack[]>();
  readonly selected = input('');
  readonly label = input('Ataque');
  readonly pick = output<string>();

  protected readonly title = attackTitle;
  protected readonly detail = (a: Attack) => attackDetail(a, true).split(' · ').slice(1).join(' · ');
}
