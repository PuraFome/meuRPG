/**
 * The map view's arithmetic, kept apart from the component so it is tested
 * without a DOM. Positions are basis points of the image (0 to 10000, the
 * API's unit); the view shows them as percentages and moves them in steps.
 */

export const BP_MAX = 10000;
export const MIN_SCALE = 1;
export const MAX_SCALE = 4;
/** The −/+ buttons zoom in 25 % steps (README-B, "Zoom"). */
export const ZOOM_STEP = 0.25;
/** An arrow key moves the selected item 0,5 %; Shift 5 %. */
export const NUDGE_BP = 50;
export const NUDGE_SHIFT_BP = 500;

/** The kinds of point, with the API's number kept out of the view's way. */
export type PointKind = 'battle' | 'submap' | 'scene';

/** What the view reads of a point (the generated `MapPoint` fits it). */
export interface ViewPoint {
  readonly id: string;
  readonly name: string;
  readonly kind: number;
  readonly xBp: number;
  readonly yBp: number;
  readonly revealed: boolean;
  /** A fog map: the point is on a square the viewer saw before and does not see now (drawn darkened). */
  readonly remembered?: boolean;
}

/** What the view reads of a token (the generated `MapToken` fits it). */
export interface ViewToken {
  readonly characterId: string;
  readonly name: string;
  readonly mine: boolean;
  readonly xBp: number;
  readonly yBp: number;
  readonly hidden: boolean;
  /** `CharacterKind`: a player's character, or an NPC kind (the fog map draws NPCs as squares). */
  readonly kind?: number;
  /** Set on a character's creature (a familiar, a summoned animal): drawn round with a dashed ring. */
  readonly creatureId?: string;
}

/** The map's pan and zoom: `x` and `y` are the stage's offset in pixels of
 * the viewport, `scale` goes from 1 (the whole image fits) to 4. */
export interface ViewTransform {
  readonly scale: number;
  readonly x: number;
  readonly y: number;
}

export const IDENTITY: ViewTransform = { scale: 1, x: 0, y: 0 };

export function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

/** A position as a CSS percentage: 5000 is 50 %. */
export function bpToPercent(bp: number): number {
  return bp / 100;
}

/** Keeps a position inside the image, as a whole number of basis points. */
export function clampBp(bp: number): number {
  return clamp(Math.round(bp), 0, BP_MAX);
}

/** The zoom level the buttons and the wheel reach: 1 to 4. */
export function clampScale(scale: number): number {
  return clamp(scale, MIN_SCALE, MAX_SCALE);
}

/** Keeps the image covering the viewport: no empty edge shows when zoomed. */
export function clampTransform(t: ViewTransform, width: number, height: number): ViewTransform {
  const scale = clampScale(t.scale);
  return {
    scale,
    x: clamp(t.x, width - width * scale, 0),
    y: clamp(t.y, height - height * scale, 0),
  };
}

/** Zooms to `scale` keeping the point under (`px`, `py`), in viewport
 * pixels, where it is: what the wheel and a pinch do. */
export function zoomAround(
  t: ViewTransform,
  scale: number,
  px: number,
  py: number,
  width: number,
  height: number,
): ViewTransform {
  const next = clampScale(scale);
  const ratio = next / t.scale;
  return clampTransform(
    { scale: next, x: px - (px - t.x) * ratio, y: py - (py - t.y) * ratio },
    width,
    height,
  );
}

/** The next zoom level of the − and + buttons, snapped to 25 % steps. */
export function stepScale(scale: number, direction: 1 | -1): number {
  const steps = Math.round(scale / ZOOM_STEP) + direction;
  return clampScale(steps * ZOOM_STEP);
}

/** "125 %", as the zoom readout says it (a no-break space before the %). */
export function scaleLabel(scale: number): string {
  return `${Math.round(scale * 100)} %`;
}

/** Where a screen point falls on the image, in basis points, whatever the
 * pan and zoom are. `rect` is the viewport's box on screen. */
export function screenToBp(
  clientX: number,
  clientY: number,
  rect: { left: number; top: number; width: number; height: number },
  t: ViewTransform,
): { xBp: number; yBp: number } {
  const localX = (clientX - rect.left - t.x) / t.scale;
  const localY = (clientY - rect.top - t.y) / t.scale;
  return {
    xBp: clampBp((localX / rect.width) * BP_MAX),
    yBp: clampBp((localY / rect.height) * BP_MAX),
  };
}

/** What an arrow key does to a position (0,5 %, Shift 5 %); `null` for a
 * key that is not an arrow. */
export function nudge(
  key: string,
  shift: boolean,
  xBp: number,
  yBp: number,
): { xBp: number; yBp: number } | null {
  const step = shift ? NUDGE_SHIFT_BP : NUDGE_BP;
  switch (key) {
    case 'ArrowLeft':
      return { xBp: clampBp(xBp - step), yBp };
    case 'ArrowRight':
      return { xBp: clampBp(xBp + step), yBp };
    case 'ArrowUp':
      return { xBp, yBp: clampBp(yBp - step) };
    case 'ArrowDown':
      return { xBp, yBp: clampBp(yBp + step) };
    default:
      return null;
  }
}

/** The token's letter: the first of its name, or the first two when
 * another token on the map starts with the same one (README-B). */
export function tokenInitial(token: ViewToken, all: readonly ViewToken[]): string {
  const letters = [...token.name.trim()];
  const first = (letters[0] ?? '?').toLocaleUpperCase('pt-BR');
  const clash = all.some(
    (other) =>
      other.characterId !== token.characterId &&
      [...other.name.trim()][0]?.toLocaleUpperCase('pt-BR') === first,
  );
  if (!clash || letters.length < 2) {
    return first;
  }
  return `${first}${letters[1].toLocaleLowerCase('pt-BR')}`;
}

export type LabelSide = 'below' | 'above' | 'left' | 'right';

/** Where a marker's label goes: under it, unless that would leave the
 * image (then beside it, or above it near the bottom edge). */
export function labelSide(xBp: number, yBp: number): LabelSide {
  if (xBp > 7800) {
    return 'left';
  }
  if (xBp < 1800) {
    return 'right';
  }
  return yBp > 9000 ? 'above' : 'below';
}

/** How far to slide a label sideways so it stays on the image: its natural
 * left and right edges against the image's (screen pixels, with a margin
 * already taken off the image's). 0 when it fits; a label wider than the
 * image starts at its left edge. The side rule above works in percentages,
 * so on a narrow map a long name near an edge still needs this. */
export function labelShift(left: number, right: number, minLeft: number, maxRight: number): number {
  if (left < minLeft || right - left > maxRight - minLeft) {
    return minLeft - left;
  }
  if (right > maxRight) {
    return maxRight - right;
  }
  return 0;
}

/** Where a label may sit: inside the image and inside what the viewport shows of it
 * (a zoomed preview crops the image, and a label near the crop's edge would be cut),
 * with a margin off each side. Screen pixels. */
export function labelBounds(
  image: { left: number; right: number },
  viewport: { left: number; right: number },
  margin = 4,
): { minLeft: number; maxRight: number } {
  return {
    minLeft: Math.max(image.left, viewport.left) + margin,
    maxRight: Math.min(image.right, viewport.right) - margin,
  };
}

/** The master sees everything, each thing with its state; anyone else only
 * what is revealed or visible, even if a hidden thing slipped in (the
 * server never sends one: this is the second lock). */
export function visiblePoints<T extends ViewPoint>(points: readonly T[], isMaster: boolean): T[] {
  return points.filter((p) => isMaster || p.revealed);
}

export function visibleTokens<T extends ViewToken>(tokens: readonly T[], isMaster: boolean): T[] {
  return tokens.filter((t) => isMaster || !t.hidden);
}

/** The centre of what the map shows, for the phone's preview: it opens
 * zoomed in on the party (README-A, "The map preview"). `null` when there
 * is nothing to centre on. */
export function centroid(items: readonly { xBp: number; yBp: number }[]): {
  xBp: number;
  yBp: number;
} | null {
  if (items.length === 0) {
    return null;
  }
  const sum = items.reduce((acc, i) => ({ x: acc.x + i.xBp, y: acc.y + i.yBp }), { x: 0, y: 0 });
  return { xBp: sum.x / items.length, yBp: sum.y / items.length };
}

/** The transform that puts `center` (basis points) in the middle of the
 * viewport at `scale`. */
export function focusTransform(
  center: { xBp: number; yBp: number },
  scale: number,
  width: number,
  height: number,
): ViewTransform {
  const s = clampScale(scale);
  return clampTransform(
    {
      scale: s,
      x: width / 2 - (center.xBp / BP_MAX) * width * s,
      y: height / 2 - (center.yBp / BP_MAX) * height * s,
    },
    width,
    height,
  );
}

export function distance(a: { x: number; y: number }, b: { x: number; y: number }): number {
  return Math.hypot(a.x - b.x, a.y - b.y);
}
