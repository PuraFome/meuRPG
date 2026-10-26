/* eslint-disable @typescript-eslint/no-magic-numbers -- timings and proportions of the flourishes, tuned by eye */
import { TAU, clamp, mulberry32 } from './dice-math';
import { RIM } from './dice-sim';
import { css } from './dice-theme';
import type {
  Effects,
  LabelLook,
  PlacedLabel,
  Rgb,
  Scene,
  SceneBody,
  Spark,
  Theme,
  View,
} from './dice-types';

/** Seconds of animation after the dice settle (glow, sparks, labels). */
export const AFTERGLOW = 1.6;

/** What the flourishes draw on, and with what. */
export interface Stage {
  readonly ctx: CanvasRenderingContext2D;
  readonly theme: Theme;
  readonly reduced: boolean;
  readonly width: number;
  readonly height: number;
}

export function makeSparks(rng: () => number, R: number): Spark[] {
  return Array.from({ length: 18 }, () => ({
    ang: rng() * TAU,
    speed: (2.4 + rng() * 4.6) * R,
    life: 0.55 + rng() * 0.4,
    size: (0.1 + rng() * 0.1) * R,
    rot: rng() * TAU,
  }));
}

/** The sparks of a critical die come from a generator of their own, so they never move the dice. */
export const sparkRng = (seed: number): (() => number) => mulberry32((seed ^ 0x85ebca6b) >>> 0);

/** A rounded rectangle added to the current path of a context. */
export function roundRect(
  c: Pick<CanvasPath, 'moveTo' | 'arcTo' | 'closePath'>,
  x: number,
  y: number,
  w: number,
  h: number,
  r: number,
): void {
  c.moveTo(x + r, y);
  c.arcTo(x + w, y, x + w, y + h, r);
  c.arcTo(x + w, y + h, x, y + h, r);
  c.arcTo(x, y + h, x, y, r);
  c.arcTo(x, y, x + w, y, r);
  c.closePath();
}

/** The glow, number and edge of a critical or a fumble, and the shake: how far along they are. */
export function effectsOf(bd: SceneBody, oT: number, reduced: boolean): Effects {
  const settled = oT >= 0;
  const k = settled ? Math.min(1, oT / 0.25) : 0;
  const shaking = bd.fumble && settled && !reduced && Number.isFinite(oT) && oT < 0.35;
  return {
    settle: k,
    alpha: bd.discarded ? 1 - 0.5 * k : 1,
    crit: bd.critical ? k : 0,
    ash: bd.fumble ? k : 0,
    dx: shaking ? Math.sin(oT * 75) * 2.2 * (bd.R / 24) * (1 - oT / 0.35) : 0,
  };
}

export function drawGlow(s: Stage, bd: SceneBody, v: View, oT: number): void {
  const { ctx, theme } = s;
  const k = Math.min(1, oT / 0.3);
  const live = Number.isFinite(oT) && oT < AFTERGLOW && !s.reduced;
  const r = bd.R * (2.1 + (live ? 0.14 * Math.sin(oT * 7) : 0));
  const g = ctx.createRadialGradient(v.x, v.y, bd.R * 0.6, v.x, v.y, r);
  g.addColorStop(0, css(theme.gold, 0.7 * k));
  g.addColorStop(1, css(theme.gold, 0));
  ctx.fillStyle = g;
  ctx.beginPath();
  ctx.arc(v.x, v.y, r, 0, TAU);
  ctx.fill();
  if (live && oT < 0.7) {
    const e = 1 - (1 - oT / 0.7) ** 3;
    ctx.strokeStyle = css(theme.gold, 0.85 * (1 - e));
    ctx.lineWidth = 2.5;
    ctx.beginPath();
    ctx.arc(v.x, v.y, bd.R * (1.1 + 1.8 * e), 0, TAU);
    ctx.stroke();
  }
}

function drawSpark(
  ctx: CanvasRenderingContext2D,
  sp: Spark,
  bd: SceneBody,
  v: View,
  oT: number,
): void {
  const u = oT / sp.life;
  const dist = bd.R * 1.05 + sp.speed * (oT - (oT * oT) / (2 * sp.life));
  const size = sp.size * (1 - 0.5 * u);
  ctx.save();
  ctx.globalAlpha = 1 - u;
  ctx.translate(v.x + Math.cos(sp.ang) * dist, v.y + Math.sin(sp.ang) * dist);
  ctx.rotate(sp.rot + oT * 3);
  ctx.beginPath();
  for (let i = 0; i < 8; i += 1) {
    const r = i % 2 === 0 ? size : size * 0.38;
    const a = (i * Math.PI) / 4;
    if (i === 0) {
      ctx.moveTo(r * Math.cos(a), r * Math.sin(a));
    } else {
      ctx.lineTo(r * Math.cos(a), r * Math.sin(a));
    }
  }
  ctx.closePath();
  ctx.fill();
  ctx.restore();
}

export function drawSparks(s: Stage, bd: SceneBody, v: View, oT: number): void {
  if (s.reduced || !Number.isFinite(oT) || !bd.sparks) {
    return;
  }
  s.ctx.fillStyle = css(s.theme.gold);
  for (const sp of bd.sparks) {
    if (oT <= sp.life) {
      drawSpark(s.ctx, sp, bd, v, oT);
    }
  }
}

// -- the words beside the dice --------------------------------------------------------------------

const PILL_FONT = {
  crit: (t: Theme) => `800 15px ${t.display}`,
  other: (t: Theme) => `700 12.5px ${t.text}`,
};

const fontOf = (look: LabelLook, t: Theme): string =>
  look === 'crit' ? PILL_FONT.crit(t) : PILL_FONT.other(t);

function pillSize(s: Stage, text: string, look: LabelLook): { w: number; h: number } {
  s.ctx.font = fontOf(look, s.theme);
  const big = look === 'crit';
  return { w: s.ctx.measureText(text).width + (big ? 20 : 16), h: big ? 24 : 20 };
}

interface PillLook {
  readonly fill: Rgb;
  readonly line: Rgb;
  readonly ink: Rgb;
  readonly width: number;
}

function pillLook(look: LabelLook, t: Theme): PillLook {
  switch (look) {
    case 'crit':
      return { fill: t.gold, line: t.goldOn, ink: t.goldOn, width: 1 };
    case 'vale':
    case 'fumble':
      return { fill: t.surface, line: t.accent, ink: t.accent, width: 1.5 };
    case 'quiet':
      return { fill: t.surface, line: t.line, ink: t.muted, width: 1 };
    default:
      return { fill: t.surface, line: t.line, ink: t.ink, width: 1 };
  }
}

function pill(s: Stage, l: PlacedLabel, alpha: number, pop: number): void {
  const { ctx, theme } = s;
  const { w, h } = l.size;
  const look = pillLook(l.look, theme);
  ctx.save();
  ctx.globalAlpha = alpha;
  ctx.font = fontOf(l.look, theme);
  ctx.translate(l.x, l.y);
  if (pop !== 1) {
    ctx.scale(pop, pop);
  }
  ctx.beginPath();
  roundRect(ctx, -w / 2, -h / 2, w, h, h / 2);
  ctx.fillStyle = css(look.fill);
  ctx.fill();
  ctx.lineWidth = look.width;
  ctx.strokeStyle = css(look.line);
  ctx.stroke();
  ctx.fillStyle = css(look.ink);
  ctx.textAlign = 'center';
  ctx.textBaseline = 'middle';
  ctx.fillText(l.text, 0, 0.5);
  ctx.restore();
}

function labelOf(bd: SceneBody): { text: string; look: LabelLook } | null {
  if (bd.critical) {
    return { text: 'Crítico!', look: 'crit' };
  }
  if (bd.fumble) {
    return { text: 'Falha crítica', look: 'fumble' };
  }
  if (bd.tag) {
    return { text: bd.tag, look: 'vale' };
  }
  if (bd.discarded) {
    return { text: 'descartado', look: 'quiet' };
  }
  return bd.part ? { text: bd.part, look: 'part' } : null;
}

interface Rect {
  readonly x0: number;
  readonly x1: number;
  readonly y0: number;
  readonly y1: number;
}

interface Disc {
  readonly x: number;
  readonly y: number;
  readonly r: number;
}

/** Whether a pill at (x, y) leaves the tray, covers a die other than `self`, or covers a label already placed. */
function blocked(
  s: Stage,
  dice: readonly Disc[],
  placed: readonly Rect[],
  box: { x: number; y: number; w: number; h: number },
  self: number,
): boolean {
  const x0 = box.x - box.w / 2 - 2;
  const x1 = box.x + box.w / 2 + 2;
  const y0 = box.y - box.h / 2 - 2;
  const y1 = box.y + box.h / 2 + 2;
  if (x0 < RIM || x1 > s.width - RIM || y0 < RIM || y1 > s.height - RIM) {
    return true;
  }
  const covers = dice.some((d, j) => {
    if (j === self) {
      return false;
    }
    const dx = d.x - clamp(d.x, x0, x1);
    const dy = d.y - clamp(d.y, y0, y1);
    return dx * dx + dy * dy < d.r * d.r;
  });
  return covers || placed.some((p) => x0 < p.x1 && x1 > p.x0 && y0 < p.y1 && y1 > p.y0);
}

/** The spots to try for a label, in order: under, over, under and shifted, beside. */
function spotsFor(look: LabelLook, size: { w: number; h: number }, R: number): [number, number][] {
  const dy = R * 1.2 + size.h / 2 + 2;
  const dx = R * 1.2 + size.w / 2 + 2;
  const sh = size.w * 0.45;
  return look === 'crit'
    ? [
        [0, -dy],
        [0, dy],
        [dx, 0],
        [-dx, 0],
      ]
    : [
        [0, dy],
        [0, -dy],
        [sh, dy],
        [-sh, dy],
        [sh, -dy],
        [-sh, -dy],
        [dx, 0],
        [-dx, 0],
      ];
}

/**
 * Where each label goes: the first spot that stays inside the tray and covers no other die and no label already
 * placed. With none free, under (clamped).
 */
export function placeLabels(s: Stage, sc: Scene): PlacedLabel[] {
  const { views, bodies } = sc;
  const dice = views.map((v, i) => ({ x: v.x, y: v.y, r: bodies[i].R * 1.12 }));
  const placed: Rect[] = [];
  const out: PlacedLabel[] = [];
  bodies.forEach((bd, i) => {
    const label = labelOf(bd);
    if (!label) {
      return;
    }
    const size = pillSize(s, label.text, label.look);
    const v = views[i];
    const spots = spotsFor(label.look, size, bd.R);
    const spot =
      spots.find(
        ([ox, oy]) => !blocked(s, dice, placed, { x: v.x + ox, y: v.y + oy, ...size }, i),
      ) ?? ([0, bd.R * 1.2 + size.h / 2 + 2] as const);
    const x = clamp(v.x + spot[0], RIM + size.w / 2 + 2, s.width - RIM - size.w / 2 - 2);
    const y = clamp(v.y + spot[1], RIM + size.h / 2 + 2, s.height - RIM - size.h / 2 - 2);
    placed.push({
      x0: x - size.w / 2 - 2,
      x1: x + size.w / 2 + 2,
      y0: y - size.h / 2 - 2,
      y1: y + size.h / 2 + 2,
    });
    out.push({ i, x, y, size, ...label });
  });
  return out;
}

/** The words beside the dice once they settle: never colour alone. Placed once per settled scene. */
export function drawLabels(s: Stage, sc: Scene, oT: number): void {
  const a = Math.min(1, oT / 0.2);
  sc.labels ??= placeLabels(s, sc);
  for (const l of sc.labels) {
    let pop = 1;
    if (l.look === 'crit' && !s.reduced && Number.isFinite(oT)) {
      const e = Math.min(1, oT / 0.35);
      pop = 0.6 + 0.4 * (1 + 2.2 * (e - 1) ** 3 + 1.2 * (e - 1) ** 2); // a small overshoot
    }
    pill(s, l, a, pop);
  }
}
