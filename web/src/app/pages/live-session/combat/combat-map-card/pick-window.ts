import type { Combatant } from '../../../../../gen/meurpg/play/v1/combat_pb';
import type { SquareWindow } from '../../../../core/maps/vision';

/** The squares kept around the creatures the player may aim at. */
const MARGIN = 1;
/** The card's shape while the player chooses a target on a phone: about as tall as it is wide (354 × 340 on the board). */
const CARD_WIDTH = 354;
const CARD_HEIGHT = 340;
const WIDTH_OVER_HEIGHT = CARD_WIDTH / CARD_HEIGHT;
/** Above this share of the grid the whole map is shown: a zoom that barely zooms only moves things. */
const MAX_SHARE = 0.75;

/**
 * The part of the map a phone shows while the player taps a target (C7-04): the creatures that can be tapped and the player's own
 * token, with a margin, shaped like the card. `null` when they are so spread out that the whole map is the window.
 */
export function pickWindow(
  tokens: readonly Pick<Combatant, 'col' | 'row'>[],
  columns: number,
  rows: number,
): SquareWindow | null {
  if (tokens.length === 0) {
    return null;
  }
  const c0 = Math.min(...tokens.map((t) => t.col)) - MARGIN;
  const c1 = Math.max(...tokens.map((t) => t.col)) + MARGIN;
  const r0 = Math.min(...tokens.map((t) => t.row)) - MARGIN;
  const r1 = Math.max(...tokens.map((t) => t.row)) + MARGIN;
  let cols = Math.min(columns, c1 - c0 + 1);
  let high = Math.min(rows, r1 - r0 + 1);
  if (cols / high > WIDTH_OVER_HEIGHT) {
    high = Math.min(rows, Math.ceil(cols / WIDTH_OVER_HEIGHT));
  } else {
    cols = Math.min(columns, Math.ceil(high * WIDTH_OVER_HEIGHT));
  }
  if (cols * high >= MAX_SHARE * columns * rows) {
    return null;
  }
  const midC = (c0 + c1) / 2;
  const midR = (r0 + r1) / 2;
  return {
    col: Math.min(Math.max(0, Math.round(midC - cols / 2)), columns - cols),
    row: Math.min(Math.max(0, Math.round(midR - high / 2)), rows - high),
    cols,
    rows: high,
  };
}
