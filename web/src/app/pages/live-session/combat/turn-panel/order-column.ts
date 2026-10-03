import { Component, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { Combatant, Encounter } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { combatantInitial, isPlayer, stateWord } from '../../../../core/combat/combat-view';
import { CombatantToken } from '../../../../shared/combatant-token/combatant-token';

/**
 * "Ordem" (E6-14): the order of the turns as a column on the left, from
 * 1280px, numbered, with each one's token, name and one word: an NPC's state
 * ("Ileso", "Ferido"), "Jogador" for another player, "Você" for the person's
 * own, "✕ Derrotado". The one on turn has the "Vez" pill on an `accent-soft`
 * row. Below 1280px the same is the strip (`order-strip`).
 */
@Component({
  selector: 'app-order-column',
  imports: [CombatantToken, MatIconModule],
  template: `
    <section class="panel" aria-labelledby="order-col-title">
      <h2 class="panel__title" id="order-col-title">Ordem</h2>
      <ol class="rows">
        @for (c of encounter().combatants; track c.id; let i = $index) {
          <li class="row" [class.row--turn]="current(c)">
            <span class="row__n" aria-hidden="true">{{ i + 1 }}</span>
            <app-combatant-token [initial]="initial(c)" [npc]="!player(c)" [defeated]="c.defeated" [mine]="c.mine" [current]="current(c)" [size]="30" />
            <span class="row__text">
              <span class="row__name" [class.row__name--out]="c.defeated">
                {{ c.label }}
                @if (current(c)) {<span class="row__turn">Vez</span>}
              </span>
              <span class="row__word" [class.row__word--out]="c.defeated">
                @if (c.defeated) {<mat-icon aria-hidden="true">close</mat-icon>}{{ word(c) }}
              </span>
            </span>
          </li>
        }
      </ol>
    </section>
  `,
  styleUrl: './order-column.scss',
})
export class OrderColumn {
  readonly encounter = input.required<Encounter>();

  protected initial(c: Combatant): string {
    return combatantInitial(c.label);
  }

  protected player(c: Combatant): boolean {
    return isPlayer(c);
  }

  protected current(c: Combatant): boolean {
    return c.id === this.encounter().currentCombatantId;
  }

  protected word(c: Combatant): string {
    if (c.defeated) {
      return 'Derrotado';
    }
    return c.mine ? 'Você' : isPlayer(c) ? 'Jogador' : stateWord(c.state);
  }
}
