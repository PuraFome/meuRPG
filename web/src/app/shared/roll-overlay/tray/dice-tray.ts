/* eslint-disable @typescript-eslint/no-magic-numbers -- the canvas sizes and the playback limits */
/*
 * DiceTray: dice rolling in a small tray, drawn flat from above on a 2D canvas, with no dependencies and no Angular.
 *
 *   const tray = createTray(canvas, { width: 480, height: 260, reducedMotion: false });
 *   const out = await tray.roll([{ sides: 20, result: 17 }, { sides: 20, result: 4, discarded: true }], { seed: 42 });
 *
 * The results always come from the caller (the server): the animation never decides them.
 *
 * How a roll is made:
 * 1. The whole roll is simulated first, headless, at a fixed step (PHYS.dt), and every step is recorded (dice-sim,
 *    dice-physics). Each die is a circle on the table (x, y), with a height (z) for the throw, a flat spin, and a
 *    "tumble": a progress p from 0 to 1 through tipping over one edge onto the next face. Rolling on the felt drives
 *    p (rolling without slipping); in the air it keeps turning; balanced on an edge, it falls back before half-way
 *    and over after it. Every time p passes 1, the die lands on a new face (k counts them).
 * 2. Hindsight (dice-hindsight): the dice are real polyhedra (dice-polyhedra: d4 to d20, the d10 a pentagonal
 *    trapezohedron). Walking the recording backwards from the server's result at rest, each tumble becomes a real
 *    tip over a real edge of the face it was resting on. So the die lands flat on every face on the way and ends on
 *    the result with its number upright. The collisions are circles, so the orientation never changes the motion:
 *    nothing is steered and nothing snaps at the end.
 * 3. Playback draws the recorded steps with requestAnimationFrame, interpolated, at the chosen speed: each die
 *    projected straight from above (orthographic), with flat, banded shading (dice-draw), so it reads as a drawing.
 *    The loop never touches anything but the canvas.
 */
import { simulate } from './dice-hindsight';
import { clamp, randomSeed } from './dice-math';
import { PHYS } from './dice-physics';
import { AFTERGLOW, drawGlow, drawLabels, drawSparks, effectsOf, type Stage } from './dice-effects';
import { DiePainter, shapeOf } from './dice-draw';
import { buildLayers, type Layers } from './dice-layers';
import { animatedScene, bodiesOf, sample, staticScene } from './dice-scene';
import { TrayAudio, type RollSound } from './dice-sound';
import { expand, simulateBodies } from './dice-sim';
import { readTheme } from './dice-theme';
import type { DieSpec, RollOptions, RollSummary, Scene, Sim, TrayOptions } from './dice-types';

export type {
  DieSpec,
  RollOptions,
  RollSummary,
  SimulationReport,
  TrayOptions,
} from './dice-types';
export type { DiceSides, DieStyle } from './dice-types';

/** A roll in flight. */
interface Run {
  readonly dice: readonly DieSpec[];
  readonly seed: number;
  readonly speed: number;
  readonly sim: Sim;
  readonly resolve: (s: RollSummary) => void;
  resolved: boolean;
  start: number | null;
  settledAt: number | null;
  raf: number;
  sound: RollSound | null;
}

/** The most device pixels per CSS pixel the canvas uses. */
const MAX_DPR = 3;
const DEFAULT_WIDTH = 320;
const DEFAULT_HEIGHT = 200;

/** One tray on one canvas. */
export class Tray {
  private W: number;
  private H: number;
  private reduced: boolean;
  private soundOn = false;
  private dev = 1;
  private layers: Layers | null = null;
  private scene: Scene | null = null;
  private run: Run | null = null;
  private shown = { tSim: 0, oT: Infinity };
  private readonly doc: Document;
  private readonly win: Window & typeof globalThis;
  private readonly ctx: CanvasRenderingContext2D;
  private readonly painter: DiePainter;
  private readonly audio: TrayAudio;
  private readonly observer: ResizeObserver | null;

  constructor(
    private readonly canvas: HTMLCanvasElement,
    options: TrayOptions = {},
  ) {
    const ctx = typeof canvas.getContext === 'function' ? canvas.getContext('2d') : null;
    const win = canvas.ownerDocument.defaultView;
    if (!ctx || !win) {
      throw new TypeError('DiceTray.create: pass a <canvas> with a 2d context');
    }
    this.ctx = ctx;
    this.win = win;
    this.doc = canvas.ownerDocument;
    this.W = options.width || DEFAULT_WIDTH;
    this.H = options.height || DEFAULT_HEIGHT;
    this.reduced = !!options.reducedMotion;
    this.painter = new DiePainter(ctx, readTheme(canvas));
    this.audio = new TrayAudio(win);
    if (options.sound) {
      this.soundOn = this.audio.enable();
    }
    this.applySize();
    this.fit();
    this.paint(0, Infinity);
    this.observer =
      typeof win.ResizeObserver === 'function'
        ? new win.ResizeObserver(() => {
            this.fit();
            this.redrawNow();
          })
        : null;
    this.observer?.observe(canvas);
  }

  /** The tray's size, in tray pixels. */
  get size(): { width: number; height: number } {
    return { width: this.W, height: this.H };
  }

  private get stage(): Stage {
    return {
      ctx: this.ctx,
      theme: this.painter.theme,
      reduced: this.reduced,
      width: this.W,
      height: this.H,
    };
  }

  private applySize(): void {
    this.canvas.style.maxWidth = `${this.W}px`;
    this.canvas.style.aspectRatio = `${this.W} / ${this.H}`;
  }

  /** Sizes the backing store to the canvas as it is on screen, and draws the felt and the rim again. */
  private fit(): void {
    const cssW = this.canvas.getBoundingClientRect().width || this.W;
    const dpr = Math.min(MAX_DPR, this.win.devicePixelRatio || 1);
    const pw = Math.max(1, Math.round(cssW * dpr));
    const ph = Math.max(1, Math.round((pw * this.H) / this.W));
    if (this.canvas.width !== pw) {
      this.canvas.width = pw;
    }
    if (this.canvas.height !== ph) {
      this.canvas.height = ph;
    }
    this.dev = pw / this.W;
    this.layers = buildLayers(this.doc, this.win, this.painter.theme, {
      pw,
      ph,
      dev: this.dev,
      W: this.W,
      H: this.H,
    });
  }

  /** One frame: tSim is the sim time; oT the seconds since the dice settled (-1 while rolling). */
  private paint(tSim: number, oT: number): void {
    const { ctx, canvas, layers, scene } = this;
    this.shown = { tSim, oT };
    ctx.setTransform(1, 0, 0, 1, 0, 0);
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    ctx.setTransform(this.dev, 0, 0, this.dev, 0, 0);
    if (layers) {
      ctx.drawImage(layers.floor as CanvasImageSource, 0, 0, this.W, this.H);
    }
    if (!scene) {
      if (layers) {
        ctx.drawImage(layers.rim as CanvasImageSource, 0, 0, this.W, this.H);
      }
      return;
    }
    this.paintScene(scene, tSim, oT);
  }

  private paintScene(scene: Scene, tSim: number, oT: number): void {
    const { ctx, painter, layers } = this;
    const stage = this.stage;
    const views = sample(scene, tSim);
    scene.bodies.forEach((bd, i) => {
      views[i].shape = shapeOf(bd, views[i]);
    });
    const order = scene.order;
    order.sort((i, j) => views[i].z - views[j].z);
    const fx = scene.bodies.map((bd) => effectsOf(bd, oT, this.reduced));
    for (const i of order) {
      painter.drawShadow(scene.bodies[i], views[i], fx[i]);
    }
    if (oT >= 0) {
      for (const i of order.filter((k) => scene.bodies[k].critical)) {
        drawGlow(stage, scene.bodies[i], views[i], oT);
      }
    }
    const lift = scene.R * 0.35; // higher than this, a die is drawn over the rim
    for (const i of order.filter((k) => views[k].z <= lift)) {
      painter.drawDie(scene.bodies[i], views[i], fx[i]);
    }
    if (layers) {
      ctx.drawImage(layers.rim as CanvasImageSource, 0, 0, this.W, this.H);
    }
    for (const i of order.filter((k) => views[k].z > lift)) {
      painter.drawDie(scene.bodies[i], views[i], fx[i]);
    }
    if (oT >= 0) {
      for (const i of order) {
        drawSparks(stage, scene.bodies[i], views[i], oT);
      }
      drawLabels(stage, scene, oT);
    }
  }

  private summary(
    dice: readonly DieSpec[],
    seed: number,
    durationMs: number,
    cancelled: boolean,
  ): RollSummary {
    return {
      cancelled,
      seed,
      durationMs,
      dice: dice.map((d) => ({ sides: d.sides, result: d.result, discarded: !!d.discarded })),
    };
  }

  private stop(cancelled: boolean): void {
    const r = this.run;
    if (!r) {
      return;
    }
    this.run = null;
    this.win.cancelAnimationFrame(r.raf);
    r.sound?.stop();
    if (!r.resolved) {
      r.resolved = true;
      const ms = cancelled ? 0 : Math.round((r.sim.duration * 1000) / r.speed);
      r.resolve(this.summary(r.dice, r.seed, ms, cancelled));
    }
  }

  private tick = (ts: number): void => {
    const r = this.run;
    if (!r) {
      return;
    }
    r.start ??= ts;
    const t = ((ts - r.start) / 1000) * r.speed;
    const end = r.sim.duration;
    if (t < end) {
      r.sound?.play(t);
      this.paint(t, -1);
      r.raf = this.win.requestAnimationFrame(this.tick);
      return;
    }
    if (r.settledAt === null) {
      r.settledAt = ts;
      r.sound?.play(end);
      r.sound?.end();
      r.resolved = true;
      r.resolve(this.summary(r.dice, r.seed, Math.round((end * 1000) / r.speed), false));
    }
    if (this.run !== r) {
      return; // a new roll started from the resolve handler
    }
    const oT = (ts - r.settledAt) / 1000;
    if (oT < AFTERGLOW) {
      this.paint(end, oT);
      r.raf = this.win.requestAnimationFrame(this.tick);
    } else {
      this.paint(end, Infinity);
      this.run = null;
    }
  };

  /**
   * Rolls the dice to the given results. Options: { seed (uint32), speed (playback, 1 = normal) }. Resolves when
   * every die has settled, with { cancelled, seed, durationMs, dice }. A new roll cancels the one in flight (that
   * one resolves with cancelled: true). Under reduced motion the dice simply appear at rest, with their results.
   */
  roll(dice: readonly DieSpec[], opts: RollOptions = {}): Promise<RollSummary> {
    let bodies;
    try {
      bodies = expand(dice);
    } catch (err) {
      return Promise.reject(err instanceof Error ? err : new Error(String(err)));
    }
    const seed = (opts.seed ?? randomSeed()) >>> 0;
    const speed = clamp(Number(opts.speed) || 1, 0.25, 3);
    this.stop(true);
    this.fit(); // picks up a zoom or a new screen since the last roll
    if (this.soundOn) {
      this.audio.enable();
    }
    if (this.reduced) {
      this.scene = staticScene(bodies, seed, this.W, this.H);
      this.paint(0, Infinity);
      return Promise.resolve(this.summary(dice, seed, 0, false));
    }
    const sim = simulateBodies(bodies, { width: this.W, height: this.H, seed });
    this.scene = animatedScene(bodies, sim, seed);
    return new Promise((resolve) => {
      const run: Run = {
        dice,
        seed,
        speed,
        sim,
        resolve,
        resolved: false,
        start: null,
        settledAt: null,
        raf: 0,
        sound: this.soundOn ? this.audio.take(sim, speed) : null,
      };
      this.run = run;
      this.paint(0, -1);
      run.raf = this.win.requestAnimationFrame(this.tick);
    });
  }

  /** Jumps to the end of the roll in flight (its promise resolves now). */
  finish(): void {
    if (!this.run) {
      return;
    }
    const end = this.run.sim.duration;
    this.stop(false);
    this.paint(end, Infinity);
  }

  /** Changes the tray's size (tray pixels). What was on the tray is laid out again, at rest. */
  resize(width: number, height: number): void {
    this.finish();
    this.W = width;
    this.H = height;
    this.applySize();
    this.fit();
    if (this.scene) {
      this.scene = staticScene(bodiesOf(this.scene), 0, this.W, this.H);
    }
    this.paint(0, Infinity);
  }

  /** Reduced motion: no motion at all, the dice simply appear with their results. */
  setReducedMotion(on: boolean): void {
    this.reduced = on;
    if (on) {
      this.finish();
    }
  }

  /** Call from a user gesture (a tap or a click): browsers only start audio after one. */
  setSound(on: boolean): void {
    this.soundOn = on ? this.audio.enable() : false;
    if (!this.soundOn && this.run?.sound) {
      this.run.sound.stop();
      this.run.sound = null;
    }
  }

  /** Re-reads the colours and fonts from the canvas's custom properties (after a theme change). */
  refreshTheme(): void {
    this.painter.reset(readTheme(this.canvas));
    if (this.scene) {
      this.scene.labels = null;
    }
    this.fit();
    this.redrawNow();
  }

  /** Redraws (after the web fonts load, say). */
  redraw(): void {
    this.painter.reset();
    if (this.scene) {
      this.scene.labels = null;
    }
    this.redrawNow();
  }

  private redrawNow(): void {
    if (!this.run) {
      this.paint(this.shown.tSim, this.shown.oT); // the next frame paints anyway while a roll runs
    }
  }

  destroy(): void {
    this.stop(true);
    this.observer?.disconnect();
    this.audio.close();
  }
}

export const createTray = (canvas: HTMLCanvasElement, options?: TrayOptions): Tray =>
  new Tray(canvas, options);

/** The module's entry points. */
export const DiceTray = Object.freeze({ create: createTray, simulate, PHYS });
