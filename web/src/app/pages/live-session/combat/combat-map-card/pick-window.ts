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
  // A creature on the map's edge keeps the whole map: the gutter around the full map holds its ring, which a crop cannot give.
  if (tokens.some((t) => t.col <= 0 || t.row <= 0 || t.col >= columns - 1 || t.row >= rows - 1)) {
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

/** The card's shape on the player's own turn on a phone: as wide as the screen and about 125px tall (354 × 125), so the first action row ends well above the sticky bar. */
const TURN_WIDTH = 354;
const TURN_HEIGHT = 125;
const TURN_WIDTH_OVER_HEIGHT = TURN_WIDTH / TURN_HEIGHT;

/** The fewest squares across the player's window: a lone token still gets room around it. */
const TURN_MIN_COLS = 12;

/**
 * The part of the map a phone shows on the player's turn (C4-03): wide and short, so the turn card and the action tabs start on the
 * first screen. It holds the player's token and the creatures that fit with a margin, centred on them (on the player's own token
 * when they do not all fit); "Ver mapa" has the whole map.
 */
export function turnWindow(
  own: Pick<Combatant, 'col' | 'row'>,
  others: readonly Pick<Combatant, 'col' | 'row'>[],
  columns: number,
  rows: number,
): SquareWindow {
  const near = others.filter(
    (t) => Math.abs(t.col - own.col) <= columns / 2 && Math.abs(t.row - own.row) <= rows / 2,
  );
  const all = [own, ...near];
  const c0 = Math.min(...all.map((t) => t.col)) - MARGIN;
  const c1 = Math.max(...all.map((t) => t.col)) + MARGIN;
  const r0 = Math.min(...all.map((t) => t.row)) - MARGIN;
  const r1 = Math.max(...all.map((t) => t.row)) + MARGIN;
  let high = Math.max(r1 - r0 + 1, Math.ceil(TURN_MIN_COLS / TURN_WIDTH_OVER_HEIGHT));
  let cols = Math.max(c1 - c0 + 1, Math.ceil(high * TURN_WIDTH_OVER_HEIGHT), TURN_MIN_COLS);
  cols = Math.min(columns, cols);
  high = Math.min(rows, Math.max(1, Math.round(cols / TURN_WIDTH_OVER_HEIGHT)));
  const fits = r1 - r0 + 1 <= high && c1 - c0 + 1 <= cols;
  const midC = fits ? (c0 + c1) / 2 : own.col;
  const midR = fits ? (r0 + r1) / 2 : own.row;
  return {
    col: Math.min(Math.max(0, Math.round(midC - cols / 2)), columns - cols),
    row: Math.min(Math.max(0, Math.round(midR - high / 2)), rows - high),
    cols,
    rows: high,
  };
}
