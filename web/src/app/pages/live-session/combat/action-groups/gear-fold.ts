import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { Combatant } from '../../../../../gen/meurpg/play/v1/combat_pb';
import type { GearVm } from '../../../../core/characters/gear';
import { type GearFlow, gearBlocked, gearCost } from '../../../../core/combat/gear-flow';
import { Fold } from '../../../../shared/fold/fold';
import { GearList } from '../../../../shared/gear-list/gear-list';

/**
 * "Sacar ou guardar": the player's gear in the "Outras" tab, folded. The list is the sheet's (`GearList`), each row with
 * what the change costs now; the flow (`GearFlow`) asks the server and reads the gear again. The fold is not drawn while
 * the gear is not read.
 */
@Component({
  selector: 'app-gear-fold',
  imports: [Fold, GearList, MatIconModule],
  templateUrl: './gear-fold.html',
  styleUrl: './gear-fold.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class GearFold {
  readonly flow = input.required<GearFlow>();
  readonly own = input.required<Combatant>();
  /** The master changes gear without cost or turn: the rows say so. */
  readonly master = input(false);
  /** Why the turn is held for something else; the rows have no buttons meanwhile. */
  readonly locked = input('');

  private readonly turn = computed(() => ({
    objectInteractionUsed: this.own().objectInteractionUsed,
    master: this.master(),
  }));
  protected readonly costOf = computed(() => {
    const turn = this.turn();
    return (piece: GearVm) => gearCost(piece, turn);
  });
  protected readonly blockedOf = computed(() => {
    const turn = this.turn();
    return (piece: GearVm) => gearBlocked(piece, turn);
  });
}
