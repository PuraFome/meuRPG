import type { AwardInput } from '../../core/progression/progression-client';

/**
 * What "Dar XP" asks the server for: the gold itself in a gold campaign; the combat's own award while the
 * total is still the combat's; an avulso (manual) award once the master changed it.
 */
export function awardInputOf(
  gold: boolean,
  total: number,
  combat: { readonly encounterId?: string; readonly amount?: number },
): AwardInput {
  if (gold) {
    return { mode: 'gold', gold: total };
  }
  if (combat.encounterId && total === combat.amount) {
    return { mode: 'enemies', encounterId: combat.encounterId };
  }
  return { mode: 'manual', amount: total };
}
