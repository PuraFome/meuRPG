import { inject } from '@angular/core';

import {
  FAVOR_MODE_ASKED,
  FavorSpend,
  circumstancesOfSources,
  favorWho,
} from '../../../../core/play/favor-spend';
import { FavorState } from '../../../../core/play/favor-state';
import type { AttackMode } from './attack-mode';
import type { AttackSheetData } from './attack-sheet-types';
import type { ModeRequests } from './mode-requests';

/**
 * The master's Inspiração (RN-43) on the d20 of an attack: the box above "Como rolar o d20". It reads the attacker, the master's
 * own sheet and the target's circumstances; an attack that has no circumstances to read (an opportunity attack) offers none, and
 * a mode asked of the master leaves the Inspiração out of the roll. Called in an injection context, by the sheet's field.
 */
export function attackFavor(
  data: AttackSheetData,
  mode: () => AttackMode,
  modes: () => ModeRequests,
): FavorSpend {
  return new FavorSpend({
    favor: inject(FavorState),
    who: () =>
      mode().hasMode()
        ? favorWho(data.state.encounter()?.combatants.find((c) => c.id === data.attackerId))
        : null,
    byMaster: () => !!data.master,
    circumstances: () => circumstancesOfSources(mode().suggested(), mode().sources()),
    sheetReason: () => (modes().request() ? FAVOR_MODE_ASKED : ''),
  });
}
