import type { Combatant } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { CombatantStateKind } from '../../../../../gen/meurpg/play/v1/combat_rolls_pb';

/** Whether the combatant is in a rage right now (the server sends the state; the app works out nothing). */
export function isRaging(c: Pick<Combatant, 'states'> | null | undefined): boolean {
  return !!c?.states.some((s) => s.kind === CombatantStateKind.RAGE);
}
