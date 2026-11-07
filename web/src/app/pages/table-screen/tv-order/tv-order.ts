import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { TvCombatantVm } from '../../../core/play/table-screen-view';
import { CombatantToken } from '../../../shared/combatant-token/combatant-token';
import { orderRows } from './order-rows';

/** The words that mean a combatant is out of the fight, for the greyed token. */
const OUT = new Set(['Derrotado', 'Caído']);

/** "Ordem" and "Rodada N": the combatants the table sees, the one on turn marked, a word of state each (RN-20). */
@Component({
  selector: 'app-tv-order',
  imports: [CombatantToken, MatIconModule],
  templateUrl: './tv-order.html',
  styleUrl: './tv-order.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TvOrder {
  readonly order = input<readonly TvCombatantVm[]>([]);
  readonly round = input(1);
  /** The turn is one the table does not see: "Vez do mestre". */
  readonly mastersTurn = input(false);

  protected readonly list = computed(() => orderRows(this.order(), this.round()));

  protected out(who: TvCombatantVm): boolean {
    return who.stateWord !== null && OUT.has(who.stateWord);
  }
}
