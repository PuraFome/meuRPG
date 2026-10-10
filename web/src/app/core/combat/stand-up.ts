import type { Combatant } from '../../../gen/meurpg/play/v1/combat_pb';
import { tight } from '../format/text';
import { metersFixed } from '../units';

/** The Prone condition, as the server keys it. */
const PRONE = 'condition:prone';

/** Tenths of a foot in a foot: the server keeps the movement in tenths. */
const TENTHS = 10;

/** "Levantar-se" as the Movimento group offers it: what it costs and, when it cannot be done, why. */
export interface StandUpRow {
  readonly detail: string;
  readonly off: boolean;
  readonly reason: string;
}

/**
 * Standing up from Prone costs half the speed (SRD 5.1, "Being Prone"), and a creature with no speed or not enough movement left
 * cannot. `null` when the combatant is not prone. The cost is the server's (`stand_up_cost_dft`); nothing is worked out here.
 */
export function standUpRow(c: Combatant): StandUpRow | null {
  if (!c.conditions.includes(PRONE)) {
    return null;
  }
  const cost = c.standUpCostDft;
  if (cost <= 0) {
    return {
      detail: 'Derrubado.',
      off: true,
      reason: 'Sem velocidade, não dá para se levantar.',
    };
  }
  const detail = tight(`Derrubado. Levantar-se gasta ${metersFixed(cost / TENTHS)} de movimento.`);
  if (c.movementLeftDft < cost) {
    return {
      detail,
      off: true,
      reason: tight(`Faltam ${metersFixed((cost - c.movementLeftDft) / TENTHS)} de movimento.`),
    };
  }
  return { detail, off: false, reason: '' };
}
