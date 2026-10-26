/** A creature as the hit test sees it: its top left square and how many squares wide it is drawn. */
export interface HitBox {
  readonly id: string;
  readonly col: number;
  readonly row: number;
  readonly span: number;
}

/** The creature a tap meant: `exact` when its square holds the point, otherwise the nearest one within reach. */
export interface Hit {
  readonly id: string;
  readonly exact: boolean;
}

/** The tallest token of a square, and the share of the square it takes: the map's `--cm-tk`. */
export const TOKEN_CAP = 40;
export const TOKEN_SHARE = 0.86;
const HALF = 2;

/** The side of a token in px: never more than the room of the squares it is drawn on, so neighbours never overlap. */
export function tokenSize(unit: number, span: number): number {
  return Math.min(unit * TOKEN_SHARE, TOKEN_CAP) * span;
}

/**
 * The creature under a point of the map (px from the surface's top left): the one whose squares hold it, whatever token or
 * button is drawn over it. A button is at least 44 px wide to be tapped, so on small squares it reaches over the neighbours';
 * the square under the finger decides. On an empty square the nearest creature within `reach` px of its centre is meant, and
 * past that none.
 */
export function creatureAt(
  x: number,
  y: number,
  unit: number,
  boxes: readonly HitBox[],
  reach = 0,
): Hit | null {
  if (unit <= 0) {
    return null;
  }
  const col = Math.floor(x / unit);
  const row = Math.floor(y / unit);
  const held = boxes
    .filter((b) => col >= b.col && col < b.col + b.span && row >= b.row && row < b.row + b.span)
    .sort((a, b) => a.span - b.span)[0];
  if (held) {
    return { id: held.id, exact: true };
  }
  let best: { id: string; d: number } | null = null;
  for (const b of boxes) {
    const d = Math.hypot(x - (b.col + b.span / HALF) * unit, y - (b.row + b.span / HALF) * unit);
    if (d <= reach && (!best || d < best.d)) {
      best = { id: b.id, d };
    }
  }
  return best ? { id: best.id, exact: false } : null;
}

/** How far (px) from a creature's centre a tap on an empty square still means it: half of the 44 px button. */
const PICK_REACH = 22;

/**
 * The creature a click on the button of `buttonId` meant. The buttons are at least 44 px wide, so on small squares they reach over
 * the neighbours' and the topmost would win: the square under the pointer decides (`creatureAt`). A click from the keyboard, or
 * with no map to measure, is the button's own creature; a tap on the square of a creature that is not on offer is `null`.
 */
export function tappedCreature(
  event: MouseEvent,
  buttonId: string,
  boxes: readonly HitBox[],
  unit: number,
): string | null {
  const surface = (event.currentTarget as HTMLElement).closest('.cm__surface');
  const keyboard = event.detail === 0 && event.clientX === 0 && event.clientY === 0;
  if (!surface || keyboard) {
    return buttonId;
  }
  const at = surface.getBoundingClientRect();
  const hit = creatureAt(event.clientX - at.left, event.clientY - at.top, unit, boxes, PICK_REACH);
  return hit ? hit.id : buttonId;
}
