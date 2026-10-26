/* eslint-disable @typescript-eslint/no-magic-numbers -- drawing proportions, tuned by eye */
import { clamp, m3 } from './dice-math';
import { LIGHT, css, mix } from './dice-theme';
import type {
  DieStyle,
  Effects,
  Face,
  Rgb,
  SceneBody,
  Shape,
  Theme,
  Vec3,
  View,
} from './dice-types';

interface Metric {
  readonly w: number;
  readonly asc: number;
  readonly desc: number;
}

const METRICS_LIMIT = 600;
const WHITE: Rgb = [255, 255, 255];

/** The die's pose now: the world rotation, and its vertices and face normals on screen (px, y down). */
export function shapeOf(bd: SceneBody, v: View): Shape {
  const seg = bd.segs[v.seg];
  const O = v.p > 0 ? m3.mul(m3.rot(seg.u, seg.beta * v.p), seg.P) : seg.P;
  const M = v.psi !== 0 ? m3.mul(m3.rotZ(v.psi), O) : O;
  const S = bd.R * bd.poly.size;
  const pts = bd.poly.verts.map((p): [number, number] => {
    const w = m3.apply(M, p);
    return [w[0] * S, -w[1] * S];
  });
  return { M, S, pts, normals: bd.poly.faces.map((f) => m3.apply(M, f.n)) };
}

/** What `printOn` needs to put one number on a face. */
interface Print {
  readonly at: Vec3;
  readonly up: Vec3;
  readonly right: Vec3;
  readonly label: string;
  readonly px: number;
  readonly maxW: number;
  readonly color: string;
  readonly alpha: number;
  readonly upright?: boolean;
}

/** Draws the dice: the faces, their numbers, the edges and the shadows. Colours and sizes come from the theme. */
export class DiePainter {
  private tones = new Map<string, string>();
  private metrics = new Map<string, Metric>();

  constructor(
    private readonly ctx: CanvasRenderingContext2D,
    public theme: Theme,
  ) {}

  /** Forget what depended on the colours or the fonts. */
  reset(theme?: Theme): void {
    if (theme) {
      this.theme = theme;
    }
    this.tones = new Map();
    this.metrics = new Map();
  }

  /** Flat shading in bands: the face's tone depends on its normal only, a few steps from the die's colour. */
  tone(style: DieStyle, n: Vec3): string {
    const b = n[0] * LIGHT[0] + n[1] * LIGHT[1] + n[2] * LIGHT[2] - LIGHT[2];
    const q = Math.round(b / 0.09);
    const key = `${style}${q}`;
    let c = this.tones.get(key);
    if (!c) {
      const t = this.theme;
      const base = style === 'garnet' ? t.garnet : t.paper;
      const dark = style === 'garnet' ? t.garnetShade : t.paperShade;
      const v = q * 0.09;
      c = css(
        v >= 0
          ? mix(base, WHITE, Math.min(0.5, v * 1.3))
          : mix(base, dark, Math.min(0.9, -v * 0.75)),
      );
      this.tones.set(key, c);
    }
    return c;
  }

  /** The width and height of a label in the current font, remembered. */
  measure(label: string): Metric {
    const { ctx } = this;
    const key = `${ctx.font}|${label}`;
    let m = this.metrics.get(key);
    if (!m) {
      const t = ctx.measureText(label);
      const size = parseFloat(ctx.font.split('px')[0].split(' ').pop() ?? '') || 12;
      m = {
        w: t.width,
        asc: Number.isFinite(t.actualBoundingBoxAscent) ? t.actualBoundingBoxAscent : size * 0.7,
        desc: Number.isFinite(t.actualBoundingBoxDescent) ? t.actualBoundingBoxDescent : 0,
      };
      if (this.metrics.size > METRICS_LIMIT) {
        this.metrics.clear();
      }
      this.metrics.set(key, m);
    }
    return m;
  }

  /** Scale by `along` in direction (ux, uy) and by `across` perpendicular to it. */
  private axisScale(ux: number, uy: number, along: number, across: number): void {
    const d = along - across;
    this.ctx.transform(across + d * ux * ux, d * ux * uy, d * ux * uy, across + d * uy * uy, 0, 0);
  }

  /** The number's colour: the die's ink, going to gold for a critical. */
  private inkOf(bd: SceneBody, crit: number): string {
    const t = this.theme;
    const ink = bd.style === 'garnet' ? t.garnetInk : t.paperInk;
    return crit > 0 ? css(mix(ink, t.gold, crit)) : css(ink);
  }

  private facePath(sh: Shape, idx: readonly number[]): void {
    const { ctx } = this;
    ctx.moveTo(sh.pts[idx[0]][0], sh.pts[idx[0]][1]);
    for (let i = 1; i < idx.length; i += 1) {
      ctx.lineTo(sh.pts[idx[i]][0], sh.pts[idx[i]][1]);
    }
    ctx.closePath();
  }

  /**
   * A number printed on a face: the face's own axes, projected, are the text's axes, so a face turning away squeezes
   * its number exactly as it would look from above.
   */
  private printOn(sh: Shape, p: Print): void {
    const { ctx } = this;
    const P = m3.apply(sh.M, p.at);
    const U = m3.apply(sh.M, p.up);
    const Rr = m3.apply(sh.M, p.right);
    ctx.save();
    ctx.globalAlpha *= p.alpha;
    if (p.upright) {
      // A d4's faces are steep: seen from above its corner numbers would be squashed to a third. They keep their
      // place and their turn, but not the squash.
      const l = Math.hypot(U[0], U[1]) || 1;
      const ux = U[0] / l;
      const uy = -U[1] / l;
      ctx.transform(-uy, ux, -ux, -uy, P[0] * sh.S, -P[1] * sh.S);
    } else {
      ctx.transform(Rr[0], -Rr[1], -U[0], U[1], P[0] * sh.S, -P[1] * sh.S);
    }
    let px = p.px;
    ctx.font = `800 ${px.toFixed(2)}px ${this.theme.display}`;
    let m = this.measure(p.label);
    if (m.w > p.maxW) {
      px *= p.maxW / m.w;
      ctx.font = `800 ${px.toFixed(2)}px ${this.theme.display}`;
      m = this.measure(p.label);
    }
    ctx.fillStyle = p.color;
    ctx.textAlign = 'center';
    ctx.textBaseline = 'alphabetic';
    const base = (m.asc - m.desc) / 2;
    ctx.fillText(p.label, 0, base);
    if (p.label === '6' || p.label === '9') {
      // the underline that tells a 6 from a 9
      ctx.fillRect(-m.w * 0.34, base + px * 0.1, m.w * 0.68, Math.max(1, px * 0.075));
    }
    ctx.restore();
  }

  /** The index of the corner of a d4 that points up. */
  private topCorner(bd: SceneBody, sh: Shape): number {
    let top = -1;
    let high = -Infinity;
    bd.poly.verts.forEach((p, vi) => {
      const z = m3.apply(sh.M, p)[2];
      if (z > high) {
        high = z;
        top = vi;
      }
    });
    return top;
  }

  /** The numbers of every face turned toward the viewer. Once the dice settle the result stays strong. */
  private printNumbers(bd: SceneBody, sh: Shape, fx: Effects): void {
    const poly = bd.poly;
    const plain = this.inkOf(bd, 0);
    const strong = this.inkOf(bd, fx.crit);
    const side = 1 - 0.72 * fx.settle;
    const topCorner = poly.corners ? this.topCorner(bd, sh) : -1;
    poly.faces.forEach((f, i) => {
      const nz = sh.normals[i][2];
      // a face seen edge-on shows no number (a d4's side faces are steep even at rest: a lower bar)
      const alpha = poly.corners ? clamp((nz - 0.05) / 0.2, 0, 1) : clamp((nz - 0.12) / 0.3, 0, 1);
      if (alpha <= 0) {
        return;
      }
      const px = f.inr * sh.S * poly.digits;
      const corners = poly.corners?.[i];
      if (corners) {
        for (const c of corners) {
          const top = c.vi === topCorner;
          this.printOn(sh, {
            ...c,
            label: bd.layout.labels[c.vi],
            px,
            maxW: f.inr * sh.S,
            color: top ? strong : plain,
            alpha: alpha * (top ? 1 : side),
            upright: true,
          });
        }
      } else {
        const top = nz > 0.98;
        this.printOn(sh, {
          at: f.c,
          up: f.up,
          right: f.right,
          label: bd.layout.labels[i],
          px,
          maxW: f.inr * sh.S * 1.85,
          color: top ? strong : plain,
          alpha: alpha * (top ? 1 : side),
        });
      }
    });
  }

  private edgePath(bd: SceneBody, sh: Shape, visible: readonly boolean[], outline: boolean): void {
    const { ctx } = this;
    ctx.beginPath();
    for (const e of bd.poly.edges) {
      if ((visible[e.f0] !== visible[e.f1]) === outline && (outline || visible[e.f0])) {
        ctx.moveTo(sh.pts[e.a][0], sh.pts[e.a][1]);
        ctx.lineTo(sh.pts[e.b][0], sh.pts[e.b][1]);
      }
    }
  }

  /** The edges between visible faces, thin; the outline, where a visible face meets a hidden one, firm. */
  private strokeEdges(
    bd: SceneBody,
    sh: Shape,
    visible: readonly boolean[],
    fx: Effects,
    lw: number,
  ): void {
    const { ctx } = this;
    const t = this.theme;
    ctx.lineJoin = 'round';
    ctx.lineCap = 'round';
    this.edgePath(bd, sh, visible, false);
    ctx.strokeStyle = css(t.edge, 0.32);
    ctx.lineWidth = lw;
    ctx.stroke();
    if (fx.ash > 0) {
      ctx.beginPath();
      bd.poly.faces.forEach((f: Face, i) => {
        if (visible[i]) {
          this.facePath(sh, f.idx);
        }
      });
      ctx.fillStyle = css(t.edge, 0.22 * fx.ash);
      ctx.fill();
    }
    this.edgePath(bd, sh, visible, true);
    ctx.strokeStyle = fx.crit > 0.5 ? css(t.gold) : css(t.edge, 0.9);
    ctx.lineWidth = (fx.crit > 0.5 ? 2.2 : 1.5) * lw;
    ctx.stroke();
  }

  drawDie(bd: SceneBody, v: View, fx: Effects): void {
    const { ctx } = this;
    const sh = v.shape;
    if (!sh) {
      return;
    }
    ctx.save();
    ctx.globalAlpha = fx.alpha;
    ctx.translate(v.x + fx.dx, v.y);
    let s = 1 + v.z / (5.5 * bd.R); // higher is a little bigger: with the shadow, the only depth cue
    if (v.sq > 0.002) {
      if (Number.isNaN(v.sqAng)) {
        s *= 1 + v.sq * 0.6; // landing: it flattens, so it looks wider from above
      } else {
        this.axisScale(Math.cos(v.sqAng), Math.sin(v.sqAng), 1 - v.sq, 1 + v.sq * 0.5); // a wall or a die
      }
    }
    ctx.scale(s, s);
    const visible = sh.normals.map((n) => n[2] > 1e-3);
    bd.poly.faces.forEach((f, i) => {
      if (visible[i]) {
        ctx.beginPath();
        this.facePath(sh, f.idx);
        ctx.fillStyle = this.tone(bd.style, sh.normals[i]);
        ctx.fill();
      }
    });
    this.printNumbers(bd, sh, fx);
    this.strokeEdges(bd, sh, visible, fx, 1 / s);
    ctx.restore();
  }

  drawShadow(bd: SceneBody, v: View, fx: Effects): void {
    const { ctx } = this;
    const sh = v.shape;
    if (!sh) {
      return;
    }
    const R = bd.R;
    const off = 0.1 * R + v.z * 0.45;
    ctx.save();
    ctx.globalAlpha = 0.2 * (1 - Math.min(0.6, v.z / (4 * R))) * fx.alpha;
    ctx.translate(v.x + fx.dx + off * 0.6, v.y + off);
    const s = 1 + v.z / (9 * R);
    ctx.scale(s, s);
    ctx.beginPath();
    bd.poly.faces.forEach((f, i) => {
      if (sh.normals[i][2] > 1e-3) {
        this.facePath(sh, f.idx);
      }
    });
    ctx.fillStyle = css(this.theme.shadow);
    ctx.fill(); // one fill of every visible face: their union, no darker seams
    ctx.restore();
  }
}
