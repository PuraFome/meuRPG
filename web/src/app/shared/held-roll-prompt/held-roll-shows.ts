import type { DiceRoll } from '../../../gen/meurpg/play/v1/combat_pb';
import { luckyReroll } from '../../core/held-rolls/held-roll-offer';
import type { HeldRolled } from '../../core/held-rolls/held-roll-offer';
import { type RollShow, showOfDie } from '../roll-overlay/roll-animator';

/** The d20 the app rolled again for Lucky, tumbling: the new face alone, "1 → 14" (the sheet tells the rest). Nothing for
 * a face the player typed, for the 1 kept, or for a roll that shows no Lucky reroll. */
export function rerollShow(roll: DiceRoll | undefined, rolled: HeldRolled | null): RollShow | null {
  const again = rolled && 'inApp' in rolled ? luckyReroll(roll) : undefined;
  return again
    ? showOfDie('Sortudo', roll?.diceSides ?? 0, again.to, `${again.from} → ${again.to}`)
    : null;
}
