/** A rectangle on the map, in px from the top left of its surface. */
interface Box {
  readonly x: number;
  readonly y: number;
  readonly w: number;
  readonly h: number;
}

/** Where a floating label sits against the token it is about (or the creature's square). */
export type Side = 'right' | 'left' | 'below' | 'above';

/** A token as the plan sees it: its square, and how many squares wide it is. */
export interface PlanToken {
  readonly id: string;
  readonly col: number;
  readonly row: number;
  readonly span: number;
}

/** What a token's tags need: the room of "Alvo" or "×2" and of "Longe" (whole and short), in px. */
export interface PlanTags {
  readonly id: string;
  readonly mark: number;
  readonly far: number;
  readonly farShort: number;
  readonly hasFar: boolean;
}

export interface PickPlanInput {
  /** Px per square; 0 while the map is not measured. */
  readonly unit: number;
  readonly columns: number;
  readonly rows: number;
  readonly tokens: readonly PlanToken[];
  readonly tags: readonly PlanTags[];
  /** The chip that says why a creature cannot be the target: its token and size. */
  readonly why: { readonly id: string; readonly w: number; readonly h: number } | null;
}

/** Where one label goes: the point it hangs from (px) and the side of the token. */
export interface Place {
  readonly x: number;
  readonly y: number;
  readonly side: Side;
  /** The "Longe" tag is written short ("Longe") here. */
  readonly short: boolean;
}

export interface PickPlan {
  readonly tags: ReadonlyMap<string, Place>;
  readonly why: Place | null;
}

/** The tallest token of a square (the map caps it), and the ring a creature on offer wears past it. */
const TOKEN_CAP = 40;
const TOKEN_SHARE = 0.86;
const RING = 7;
/** The air between a token's ring and its label, and the room a label keeps from the map's outer edge. */
const GAP = 4;
const BLEED = 8;
const TAG_HEIGHT = 20;
const TAG_GAP = 4;
const HALF = 2;
const SIDES: readonly Side[] = ['right', 'left', 'below', 'above'];

function overlaps(a: Box, b: Box): boolean {
  return a.x < b.x + b.w && b.x < a.x + a.w && a.y < b.y + b.h && b.y < a.y + a.h;
}

function boxAt(side: Side, x: number, y: number, w: number, h: number): Box {
  switch (side) {
    case 'right':
      return { x, y: y - h / HALF, w, h };
    case 'left':
      return { x: x - w, y: y - h / HALF, w, h };
    case 'below':
      return { x: x - w / HALF, y, w, h };
    default:
      return { x: x - w / HALF, y: y - h, w, h };
  }
}

/**
 * Lays the labels of the target choice out (C7-04): each token's tags ("Alvo", "Longe") beside it, on its right (its left when
 * the right has no room, below or above when neither has), "Alvo" first and nearest the token; then the chip that says why a
 * creature cannot be chosen, in free map. A label never touches a token's ring, another token's label or the map's outer
 * edge; the whole "Longe: desvantagem" gives way to "Longe" before a label is put over something, and a chip with no free
 * place comes back `null` (the bar's hint line says it instead).
 */
export function planPick(input: PickPlanInput): PickPlan {
  const { unit, columns, rows } = input;
  const tags = new Map<string, Place>();
  if (unit <= 0) {
    // Not measured yet: beside the token, as written.
    for (const t of input.tags) {
      tags.set(t.id, { x: 0, y: 0, side: 'right', short: false });
    }
    return { tags, why: input.why ? { x: 0, y: 0, side: 'right', short: false } : null };
  }
  const width = columns * unit;
  const height = rows * unit;
  const ring = (t: PlanToken): { cx: number; cy: number; r: number } => ({
    cx: (t.col + t.span / HALF) * unit,
    cy: (t.row + t.span / HALF) * unit,
    r: (Math.min(unit * TOKEN_SHARE, TOKEN_CAP) * t.span) / HALF + RING,
  });
  const rings = input.tokens.map((t) => {
    const g = ring(t);
    return { id: t.id, box: { x: g.cx - g.r, y: g.cy - g.r, w: g.r * HALF, h: g.r * HALF } };
  });
  const placed: Box[] = [];
  const inside = (b: Box): boolean =>
    b.x >= -BLEED && b.y >= -BLEED && b.x + b.w <= width + BLEED && b.y + b.h <= height + BLEED;
  const fit = (id: string, w: number, h: number, force = false): Place | null => {
    const token = input.tokens.find((t) => t.id === id);
    if (!token) {
      return null;
    }
    const g = ring(token);
    for (const side of SIDES) {
      const x = side === 'right' ? g.cx + g.r + GAP : side === 'left' ? g.cx - g.r - GAP : g.cx;
      const y = side === 'below' ? g.cy + g.r + GAP : side === 'above' ? g.cy - g.r - GAP : g.cy;
      const box = boxAt(side, x, y, w, h);
      if (
        inside(box) &&
        !rings.some((r) => r.id !== id && overlaps(box, r.box)) &&
        !placed.some((p) => overlaps(box, p))
      ) {
        placed.push(box);
        return { x, y, side, short: false };
      }
    }
    return force ? { x: g.cx + g.r + GAP, y: g.cy, side: 'right', short: false } : null;
  };
  for (const t of input.tags) {
    const whole = t.mark + (t.hasFar ? TAG_GAP + t.far : 0);
    const short = t.mark + (t.hasFar ? TAG_GAP + t.farShort : 0);
    const place = fit(t.id, whole, TAG_HEIGHT);
    if (place) {
      tags.set(t.id, place);
    } else {
      const small = fit(t.id, short, TAG_HEIGHT, true);
      if (small) {
        tags.set(t.id, { ...small, short: true });
      }
    }
  }
  const why = input.why ? fit(input.why.id, input.why.w, input.why.h) : null;
  return { tags, why };
}
