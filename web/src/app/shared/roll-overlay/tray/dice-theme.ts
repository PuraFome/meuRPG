/* eslint-disable @typescript-eslint/no-magic-numbers -- colour arithmetic and the light's direction */
import type { Rgb, Theme, Vec3 } from './dice-types';

/*
 * Colours: the page's tokens, read from the canvas. A token is a `light-dark()` pair, which a custom property keeps
 * as text, so each one is resolved through a real property (`color`) to the colour the canvas's own theme gives it.
 * The fallbacks are for a page without the tokens (a unit test); the app's values are in `styles.scss`.
 */
const TOKENS: Readonly<
  Record<Exclude<keyof Theme, 'rimHi' | 'rimLo' | 'display' | 'text'>, readonly [string, Rgb]>
> = {
  floor: ['--mr-tray-felt', [169, 189, 163]],
  rim: ['--mr-tray-rim', [122, 82, 50]],
  shadow: ['--mr-tray-shadow', [38, 53, 42]],
  paper: ['--mr-die-paper', [255, 255, 255]],
  paperInk: ['--mr-die-ink', [27, 34, 48]],
  paperShade: ['--mr-die-paper-shade', [141, 149, 163]],
  garnet: ['--mr-die-garnet', [158, 43, 59]],
  garnetInk: ['--mr-die-on-garnet', [255, 255, 255]],
  garnetShade: ['--mr-die-garnet-shade', [58, 14, 22]],
  edge: ['--mr-die-edge', [27, 34, 48]],
  gold: ['--mr-die-gold', [248, 211, 110]],
  goldOn: ['--mr-die-on-gold', [42, 29, 2]],
  surface: ['--mr-surface', [255, 255, 255]],
  ink: ['--mr-ink', [27, 34, 48]],
  line: ['--mr-line', [213, 218, 225]],
  accent: ['--mr-accent-text', [158, 43, 59]],
  muted: ['--mr-ink-muted', [82, 91, 107]],
};

const HEX3 = /^#([0-9a-f]{3})$/i;
const HEX6 = /^#([0-9a-f]{6})$/i;
const FN = /^rgba?\(\s*([\d.]+)[,\s]+([\d.]+)[,\s]+([\d.]+)/i;
const SRGB = /^color\(\s*srgb\s+([\d.]+)\s+([\d.]+)\s+([\d.]+)/i;

/** A colour as the browser reports it (`rgb()`, `rgba()`, `color(srgb …)`) or as a hex literal; null if it is none. */
export function parseColor(s: string | null | undefined): Rgb | null {
  const v = (s ?? '').trim();
  const h3 = HEX3.exec(v);
  if (h3) {
    const [r, g, b] = h3[1].split('').map((c) => parseInt(c + c, 16));
    return [r, g, b];
  }
  const h6 = HEX6.exec(v);
  if (h6) {
    const [r, g, b] = [0, 2, 4].map((i) => parseInt(h6[1].slice(i, i + 2), 16));
    return [r, g, b];
  }
  const fn = FN.exec(v);
  if (fn) {
    return [+fn[1], +fn[2], +fn[3]];
  }
  const sr = SRGB.exec(v);
  return sr ? [+sr[1] * 255, +sr[2] * 255, +sr[3] * 255] : null;
}

export function css(c: Rgb, a = 1): string {
  const [r, g, b] = c.map(Math.round);
  return a >= 1 ? `rgb(${r},${g},${b})` : `rgba(${r},${g},${b},${a.toFixed(3)})`;
}

export const mix = (p: Rgb, q: Rgb, t: number): Rgb => [
  p[0] + (q[0] - p[0]) * t,
  p[1] + (q[1] - p[1]) * t,
  p[2] + (q[2] - p[2]) * t,
];

const WHITE: Rgb = [255, 255, 255];
const BLACK: Rgb = [0, 0, 0];

/** The colour a token has on this canvas, or null when the page does not define it. */
function resolved(canvas: HTMLCanvasElement, cs: CSSStyleDeclaration, token: string): Rgb | null {
  if (!cs.getPropertyValue(token).trim()) {
    return null;
  }
  const previous = canvas.style.color;
  canvas.style.color = `var(${token})`;
  const color = parseColor(cs.color);
  canvas.style.color = previous;
  return color;
}

export function readTheme(canvas: HTMLCanvasElement): Theme {
  const cs = canvas.ownerDocument.defaultView?.getComputedStyle(canvas);
  const colors: Record<string, Rgb> = {};
  for (const [key, [token, fallback]] of Object.entries(TOKENS)) {
    colors[key] = (cs && resolved(canvas, cs, token)) || fallback;
  }
  const rim = colors['rim'];
  return {
    ...(colors as Omit<Theme, 'rimHi' | 'rimLo' | 'display' | 'text'>),
    rimHi: mix(rim, WHITE, 0.22),
    rimLo: mix(rim, BLACK, 0.35),
    display: cs?.getPropertyValue('--mr-font-display').trim() || 'Alegreya, Georgia, serif',
    text: cs?.getPropertyValue('--mr-font-sans').trim() || "'Alegreya Sans', system-ui, sans-serif",
  };
}

/** The light comes from the upper left and above; a face's tone depends on its normal only (flat shading). */
export const LIGHT: Vec3 = (() => {
  const v: Vec3 = [-0.45, 0.65, 1]; // x right, y up, z toward the viewer
  const l = Math.hypot(v[0], v[1], v[2]);
  return [v[0] / l, v[1] / l, v[2] / l];
})();
