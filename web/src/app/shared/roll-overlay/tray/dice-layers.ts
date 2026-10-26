/* eslint-disable @typescript-eslint/no-magic-numbers -- the proportions of the felt and the rim */
import { roundRect } from './dice-effects';
import { RIM } from './dice-sim';
import { css } from './dice-theme';
import type { Theme } from './dice-types';

type Layer = HTMLCanvasElement | OffscreenCanvas;

export interface Layers {
  readonly floor: Layer;
  readonly rim: Layer;
}

/** A canvas to draw a layer on: off screen where the browser has it. */
function layer(doc: Document, win: Window & typeof globalThis, pw: number, ph: number): Layer {
  if (typeof win.OffscreenCanvas === 'function') {
    return new win.OffscreenCanvas(pw, ph);
  }
  const c = doc.createElement('canvas');
  c.width = pw;
  c.height = ph;
  return c;
}

function context(
  l: Layer,
  dev: number,
): CanvasRenderingContext2D | OffscreenCanvasRenderingContext2D {
  const c = l.getContext('2d') as CanvasRenderingContext2D | OffscreenCanvasRenderingContext2D;
  c.setTransform(dev, 0, 0, dev, 0, 0);
  return c;
}

type Ctx = CanvasRenderingContext2D | OffscreenCanvasRenderingContext2D;

/** The felt: its colour, and the walls' shade on it, wider along the top and the left (the light comes from there). */
function paintFelt(f: Ctx, theme: Theme, W: number, H: number): void {
  f.beginPath();
  roundRect(f, 0, 0, W, H, 16);
  f.fillStyle = css(theme.floor);
  f.fill();
  const band = (x0: number, y0: number, x1: number, y1: number, a: number): CanvasGradient => {
    const g = f.createLinearGradient(x0, y0, x1, y1);
    g.addColorStop(0, css(theme.shadow, a));
    g.addColorStop(1, css(theme.shadow, 0));
    return g;
  };
  f.fillStyle = band(0, RIM, 0, RIM + 16, 0.16);
  f.fillRect(RIM, RIM, W - 2 * RIM, 16);
  f.fillStyle = band(RIM, 0, RIM + 14, 0, 0.12);
  f.fillRect(RIM, RIM, 14, H - 2 * RIM);
  f.fillStyle = band(0, H - RIM, 0, H - RIM - 6, 0.07);
  f.fillRect(RIM, H - RIM - 6, W - 2 * RIM, 6);
  f.fillStyle = band(W - RIM, 0, W - RIM - 6, 0, 0.07);
  f.fillRect(W - RIM - 6, RIM, 6, H - 2 * RIM);
}

/** The wooden rim, with a light edge outside and a dark one inside. */
function paintRim(r: Ctx, theme: Theme, W: number, H: number): void {
  r.beginPath();
  roundRect(r, 0, 0, W, H, 16);
  roundRect(r, RIM, RIM, W - 2 * RIM, H - 2 * RIM, 8);
  r.fillStyle = css(theme.rim);
  r.fill('evenodd');
  r.beginPath();
  roundRect(r, 0.75, 0.75, W - 1.5, H - 1.5, 15.5);
  r.strokeStyle = css(theme.rimHi);
  r.lineWidth = 1.5;
  r.stroke();
  r.beginPath();
  roundRect(r, RIM - 0.75, RIM - 0.75, W - 2 * RIM + 1.5, H - 2 * RIM + 1.5, 8.5);
  r.strokeStyle = css(theme.rimLo);
  r.lineWidth = 1.5;
  r.stroke();
}

/** The felt and the rim are drawn once per size and theme. */
export function buildLayers(
  doc: Document,
  win: Window & typeof globalThis,
  theme: Theme,
  size: { pw: number; ph: number; dev: number; W: number; H: number },
): Layers {
  const floor = layer(doc, win, size.pw, size.ph);
  paintFelt(context(floor, size.dev), theme, size.W, size.H);
  const rim = layer(doc, win, size.pw, size.ph);
  paintRim(context(rim, size.dev), theme, size.W, size.H);
  return { floor, rim };
}
