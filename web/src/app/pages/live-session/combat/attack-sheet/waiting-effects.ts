import type { Encounter } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { waitingHitLines } from '../../../../core/effects/hit-effect-text';

/** After a miss: the attacker's own hit effects that still wait for a weapon hit, one sentence each (#577). */
export function waitingEffectsOf(
  encounter: Encounter | null | undefined,
  attackerId: string,
  hit: boolean | undefined,
): readonly string[] {
  const attacker = encounter?.combatants.find((c) => c.id === attackerId);
  return hit === false && attacker ? waitingHitLines(attacker) : [];
}
