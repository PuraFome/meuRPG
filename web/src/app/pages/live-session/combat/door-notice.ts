import { type Signal, effect, untracked } from '@angular/core';

import { CombatLogKind } from '../../../../gen/meurpg/play/v1/combat_pb';
import type { CombatLogState } from '../../../core/combat/combat-log-state';

/**
 * Says when the player's own move opened a door (RN-26): the log line and the map already do, but a line in a log nobody has open is
 * not a notice. The first read of the log is only the baseline; after it, a "door opened" entry of the player's own combatant that was not
 * there before calls `say` once. Call it in a constructor.
 */
export function noticeOpenedDoors(
  log: Pick<CombatLogState, 'rounds' | 'loaded'>,
  own: Signal<{ readonly id: string } | null | undefined>,
  say: () => void,
): void {
  const seen = new Set<string>();
  let primed = false;
  effect(() => {
    const read = log.rounds();
    const mine = own()?.id;
    if (!log.loaded()) {
      primed = false;
      untracked(() => seen.clear());
      return;
    }
    untracked(() => {
      let fresh = false;
      for (const round of read) {
        for (const entry of round.entries) {
          if (entry.kind === CombatLogKind.DOOR_OPENED && !seen.has(entry.id)) {
            seen.add(entry.id);
            fresh = fresh || (primed && mine !== undefined && entry.actorId === mine);
          }
        }
      }
      primed = true;
      if (fresh) {
        say();
      }
    });
  });
}
