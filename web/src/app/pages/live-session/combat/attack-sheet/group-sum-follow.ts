import { effect, untracked } from '@angular/core';

import { PendingDamageStatus } from '../../../../../gen/meurpg/play/v1/combat_pb';
import type { CombatClient } from '../../../../core/combat/combat-client';
import { removalFor } from '../../../../core/combat/group-take-out';
import type { AttackState } from './attack-state';

/**
 * The player still had the sheet open on the damage (#835) when the master took an extra out of its group: the damage now
 * awaits one sum again. The master's removal reaches the page as a line of the combat log (the only place that says which
 * part and why), so the sheet reads the turn options once when such a line is there for its damage, and goes back to the group
 * field with the notice. What the player typed is not kept: it was the sum of dice that are no longer the same.
 * Call it from the sheet's constructor.
 */
export function watchGroupSum(s: AttackState, api: CombatClient): void {
  effect(() => {
    const removals = s.data.state.removals();
    const done = s.damage();
    if (done && s.stage() === 'done' && removalFor(done, removals)) {
      untracked(() => void follow(s, api, done.id));
    }
  });
}

async function follow(s: AttackState, api: CombatClient, id: string): Promise<void> {
  try {
    const res = await api.moves.turnOptions(
      s.data.campaignId,
      s.data.encounterId,
      s.data.attackerId,
    );
    const now = res.pendingDamages.find(
      (p) => p.id === id && p.status === PendingDamageStatus.AWAITING_GROUP_SUM,
    );
    if (now && s.damage()?.id === id && s.stage() === 'done') {
      s.pending.set(now);
      s.damage.set(null);
      s.typing.set(false);
      s.stage.set('damage');
    }
  } catch {
    // The sheet keeps what it shows; the next removal line asks again.
  }
}
