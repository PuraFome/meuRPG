/**
 * The map view's arithmetic, kept apart from the component so it is tested
 * without a DOM. Positions are basis points of the image (0 to 10000, the
 * API's unit); the view shows them as percentages and moves them in steps.
 */

import { MapPointKind } from '../../../gen/meurpg/maps/v1/maps_pb';
import { pointHidden } from './map-labels';

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
  /** A generated dungeon's stair (`StairDirection`: 1 up, 2 down), 0 for any other point. */
  readonly stairs?: number;
  /** A fog map: the point is on a square the viewer saw before and does not see now (drawn darkened). */
  readonly remembered?: boolean;
  /** A treasure marked found, a trap shown to some characters or already fired: the players know it (`pointHidden`). */
  readonly treasureFoundAt?: unknown;
  readonly trapRevealedTo?: readonly unknown[];
  readonly trap?: { readonly state: number } | undefined;
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

/** What tells one token from another: a creature's ID, or the character's (a creature's `characterId` is its owner's). */
export function tokenKey(token: { readonly characterId: string; readonly creatureId?: string }): string {
  return token.creatureId || token.characterId;
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
  return points.filter((p) => isMaster || (p.kind !== MapPointKind.LIGHT && !pointHidden(p)));
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

/** A box on screen, in pixels. */
export interface Box {
  readonly left: number;
  readonly top: number;
  readonly right: number;
  readonly bottom: number;
}

/** The smallest box holding all of `boxes` (`null` for none). */
export function unionBox(boxes: readonly Box[]): Box | null {
  if (boxes.length === 0) {
    return null;
  }
  return {
    left: Math.min(...boxes.map((b) => b.left)),
    top: Math.min(...boxes.map((b) => b.top)),
    right: Math.max(...boxes.map((b) => b.right)),
    bottom: Math.max(...boxes.map((b) => b.bottom)),
  };
}

/** The area two boxes share, in square pixels. */
export function overlapArea(a: Box, b: Box): number {
  const w = Math.min(a.right, b.right) - Math.max(a.left, b.left);
  const h = Math.min(a.bottom, b.bottom) - Math.max(a.top, b.top);
  return w > 0 && h > 0 ? w * h : 0;
}

/** Points on the same grid square share one label: the key of the square a point stands on (`columns` 0: no grid, the key is a coarse spot). */
export function squareKey(xBp: number, yBp: number, columns: number, aspect: number): string {
  if (columns > 0) {
    const rows = Math.max(1, Math.round(columns / aspect));
    const col = Math.min(columns - 1, Math.floor((xBp / BP_MAX) * columns));
    const row = Math.min(rows - 1, Math.floor((yBp / BP_MAX) * rows));
    return `${col}/${row}`;
  }
  return `${Math.round(xBp / 150)}/${Math.round(yBp / 150)}`;
}

/**
 * Where a label goes: beside what it names (its `anchor`: the marker and the area of its points), never over it, and off the other
 * things on the map (`obstacles`: areas, markers, tokens and the labels already placed). It tries the sides in turn (right, left,
 * below, above, each centred and then lined up with an edge) and takes the first that touches nothing; when every one does, the
 * one that covers least. It always stays inside `bounds`. Returns the pill's top-left corner on screen.
 */
export function placeLabel(
  anchor: Box,
  size: { width: number; height: number },
  obstacles: readonly Box[],
  bounds: { minLeft: number; maxRight: number; minTop: number; maxBottom: number },
  gap = 6,
): { left: number; top: number } {
  const { width: w, height: h } = size;
  const cx = (anchor.left + anchor.right) / 2;
  const cy = (anchor.top + anchor.bottom) / 2;
  // Each side in turn, centred and then lined up with an edge; and when a crowded map leaves none free, the same sides again, farther out.
  const around = (g: number): { left: number; top: number }[] => [
    { left: anchor.right + g, top: cy - h / 2 },
    { left: anchor.left - g - w, top: cy - h / 2 },
    { left: cx - w / 2, top: anchor.bottom + g },
    { left: cx - w / 2, top: anchor.top - g - h },
    { left: anchor.right + g, top: anchor.top },
    { left: anchor.right + g, top: anchor.bottom - h },
    { left: anchor.left - g - w, top: anchor.top },
    { left: anchor.left - g - w, top: anchor.bottom - h },
    { left: anchor.left, top: anchor.bottom + g },
    { left: anchor.right - w, top: anchor.bottom + g },
    { left: anchor.left, top: anchor.top - g - h },
    { left: anchor.right - w, top: anchor.top - g - h },
  ];
  const candidates = [...around(gap), ...around(gap + h), ...around(gap + 2 * h)];
  let best = candidates[0];
  let bestCost = Number.POSITIVE_INFINITY;
  for (const c of candidates) {
    // Slide it back inside the image (a wider label than the image starts at its left edge).
    const left = w > bounds.maxRight - bounds.minLeft ? bounds.minLeft : clamp(c.left, bounds.minLeft, bounds.maxRight - w);
    const top = clamp(c.top, bounds.minTop, Math.max(bounds.minTop, bounds.maxBottom - h));
    const box: Box = { left, top, right: left + w, bottom: top + h };
    // What it covers of its own anchor weighs most; then the others; then how far sliding moved it.
    const far = Math.max(0, Math.hypot(left + w / 2 - cx, top + h / 2 - cy) - Math.hypot(w / 2 + (anchor.right - anchor.left) / 2, h / 2 + (anchor.bottom - anchor.top) / 2)) / 100;
    // A label beside another mark reads as that mark's: keep a 12 px margin off the others (it weighs less than covering them).
    const crowd = obstacles.reduce((sum, o) => sum + overlapArea(box, { left: o.left - 12, top: o.top - 12, right: o.right + 12, bottom: o.bottom + 12 }), 0) * 0.5;
    const cost = overlapArea(box, anchor) * 4 + obstacles.reduce((sum, o) => sum + overlapArea(box, o), 0) + crowd + Math.abs(left - c.left) + Math.abs(top - c.top) + far;
    if (cost < bestCost) {
      bestCost = cost;
      best = { left, top };
      if (cost === 0) {
        break;
      }
    }
  }
  return best;
}
