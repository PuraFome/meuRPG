import { SpellAreaShape } from '../../../gen/meurpg/rules/v1/rules_pb';
import type { SpellZone } from '../../../gen/meurpg/play/v1/combat_pb';
import { distanceText } from '../units';
import type { Square } from './combat-grid';
import { centered, rangeFt, sphereSquares } from './spell-area';

/**
 * How far a player's caster moves an area with an action, in feet (18 m). The server owns the number
 * (`moveRangeFt` in backend/internal/play/zones_catalog.go) and sends it in no field, so the client keeps a copy
 * to dim the squares beyond it; the server's `TARGET_OUT_OF_REACH` (with `missing_ft`) is the rule. The master has
 * no limit.
 */
export const ZONE_MOVE_RANGE_FT = 60;

export interface ZoneMovePreview {
  /** The squares the area would cover around the chosen square (empty for a shape that is not centered). */
  readonly squares: readonly Square[];
  /** From the zone's current origin to the chosen square, in feet. */
  readonly distanceFt: number;
  /** "4,5 m · 3 quadrados". */
  readonly text: string;
  readonly inRange: boolean;
  /** How far past the range, in feet; 0 when in range. */
  readonly missingFt: number;
  /** Always false: the preview is the open-floor shape. No wall cuts it here (no call previews a placed zone); the
   * server cuts the area when the move is confirmed and the map then draws its squares. */
  readonly wallCut: false;
}

/** What moving `zone` to `to` would look like, drawn by the browser alone. Sphere and cylinder only (the zones that
 * move by an action are Moonbeam's cylinder): the other shapes have no preview squares. */
export function zoneMovePreview(zone: SpellZone, to: Square, master: boolean): ZoneMovePreview {
  const from = zone.origin ?? to;
  const distanceFt = rangeFt(from, to);
  const missingFt = master ? 0 : Math.max(0, distanceFt - ZONE_MOVE_RANGE_FT);
  const drawn = centered(zone.shape) && zone.shape !== SpellAreaShape.UNSPECIFIED;
  return {
    squares: drawn ? sphereSquares(to, zone.sizeFt) : [],
    distanceFt,
    text: distanceText(distanceFt),
    inRange: missingFt === 0,
    missingFt,
    wallCut: false,
  };
}
