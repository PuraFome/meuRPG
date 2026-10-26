/* eslint-disable @typescript-eslint/no-magic-numbers -- the layout of resting dice, tuned by eye */
import { clamp } from './dice-math';
import { PHYS } from './dice-physics';
import { finalPose, layoutOf, polyOf } from './dice-polyhedra';
import { orient } from './dice-hindsight';
import { makeSparks, sparkRng } from './dice-effects';
import { FRAME, RIM, dieRadius } from './dice-sim';
import type { Body, Orientation, Scene, SceneBody, Sim, View } from './dice-types';

export function blankView(): View {
  return { x: 0, y: 0, z: 0, psi: 0, p: 0, seg: 0, sq: 0, sqAng: NaN, shape: null };
}

/** The pose of a die that never rolled: its result on top, upright. */
function restingOrientation(b: Body): Orientation {
  const poly = polyOf(b.kind);
  const layout = layoutOf(b.kind, b.labels);
  const rest = finalPose(poly, layout, b.target, 0);
  return {
    poly,
    layout,
    segs: [{ first: 0, last: 0, k: 0, tdx: 0, tdy: 0, u: [1, 0, 0], beta: 0, P: rest }],
    segOf: null,
    angleEnd: 0,
    rest,
    tilt: 0,
  };
}

function decorate(
  bodies: readonly Body[],
  R: number,
  plan: readonly Orientation[] | null,
  seed: number,
): SceneBody[] {
  const rng = sparkRng(seed);
  return bodies.map((b, i) => ({
    ...b,
    R,
    ...(plan ? plan[i] : restingOrientation(b)),
    sparks: b.critical ? makeSparks(rng, R) : null,
  }));
}

/** Dice laid out in rows, centred, at rest: the reduced-motion roll, and what a resize shows. */
export function staticScene(bodies: readonly Body[], seed: number, W: number, H: number): Scene {
  const n = bodies.length;
  const R = dieRadius(W, H, n);
  const gap = 2.8 * R;
  const rowGap = 3.1 * R;
  const cols = Math.max(1, Math.min(n, Math.floor((W - 2 * RIM) / gap)));
  const rows = Math.ceil(n / cols);
  const views = bodies.map((_, i) => {
    const row = Math.floor(i / cols);
    const inRow = row < rows - 1 ? cols : n - cols * (rows - 1);
    const v = blankView();
    v.x = W / 2 + ((i % cols) - (inRow - 1) / 2) * gap;
    v.y = H / 2 + (row - (rows - 1) / 2) * rowGap - 0.3 * R;
    return v;
  });
  return {
    bodies: decorate(bodies, R, null, seed),
    views,
    sim: null,
    order: bodies.map((_, i) => i),
    R,
    labels: null,
  };
}

export function animatedScene(bodies: readonly Body[], sim: Sim, seed: number): Scene {
  return {
    bodies: decorate(bodies, sim.R, orient(bodies, sim, seed), seed),
    views: bodies.map(blankView),
    sim,
    order: bodies.map((_, i) => i),
    R: sim.R,
    labels: null,
  };
}

/** The recorded state at sim time t, interpolated between the two steps around it. */
export function sample(sc: Scene, t: number): View[] {
  const { views, sim } = sc;
  if (!sim) {
    return views;
  }
  const { frames, steps, n } = sim;
  const f = clamp(t / PHYS.dt, 0, steps);
  const i0 = Math.floor(f);
  const i1 = Math.min(steps, i0 + 1);
  const a = f - i0;
  const lerp = (o0: number, o1: number, field: number): number =>
    frames[o0 + field] + (frames[o1 + field] - frames[o0 + field]) * a;
  for (let j = 0; j < n; j += 1) {
    const bd = sc.bodies[j];
    const segOf = bd.segOf;
    const o0 = (i0 * n + j) * FRAME;
    const o1 = (i1 * n + j) * FRAME;
    const v = views[j];
    v.x = lerp(o0, o1, 0);
    v.y = lerp(o0, o1, 1);
    v.psi = -(lerp(o0, o1, 2) - bd.angleEnd);
    v.z = lerp(o0, o1, 3);
    v.sq = lerp(o0, o1, 8);
    v.sqAng = frames[o1 + 9];
    const same = !segOf || segOf[i0] === segOf[i1];
    const src = same || a < 0.5 ? i0 : i1;
    v.seg = segOf ? segOf[src] : 0;
    v.p = same ? lerp(o0, o1, 4) : frames[(src * n + j) * FRAME + 4];
  }
  return views;
}

/** The same bodies, without what the recording gave them (for a resize). */
export function bodiesOf(sc: Scene): Body[] {
  return sc.bodies.map((sb): Body => ({
    source: sb.source,
    discarded: sb.discarded,
    tag: sb.tag,
    critical: sb.critical,
    fumble: sb.fumble,
    part: sb.part,
    kind: sb.kind,
    labels: sb.labels,
    target: sb.target,
    style: sb.style,
  }));
}
