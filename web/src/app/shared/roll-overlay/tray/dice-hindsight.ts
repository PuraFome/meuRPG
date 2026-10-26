/* eslint-disable @typescript-eslint/no-magic-numbers -- the hash constants of a seed and a table of sound thresholds */
import { m3, mulberry32 } from './dice-math';
import { PHYS } from './dice-physics';
import { edgeToward, finalPose, layoutOf, polyOf, tiltOf, topLabel } from './dice-polyhedra';
import { FRAME, REST_TILT, expand, simulateBodies } from './dice-sim';
import type {
  Body,
  DieSpec,
  Mat3,
  Orientation,
  Segment,
  Sim,
  SimulationReport,
} from './dice-types';

/*
 * Hindsight. The simulation recorded, per step, the tumble (k faces done, progress p on the next, the direction it
 * tips) and the flat spin. Walking it backwards from the server's result at rest, each stretch of one tumble becomes a
 * real tip over a real edge: the pose where it started is the pose where it ended, turned back over the trailing edge
 * by that edge's angle. So the die lands flat on every face on the way, ends on the result with its number upright,
 * and the motion is exactly what was simulated.
 */

/** Cuts the recording of one die into stretches of one tumble in one direction. */
function segmentsOf(sim: Sim, j: number): { segs: Segment[]; segOf: Int32Array } {
  const { frames, steps, n } = sim;
  const at = (i: number, field: number): number => frames[(i * n + j) * FRAME + field];
  const segs: Segment[] = [];
  const segOf = new Int32Array(steps + 1);
  for (let i = 0; i <= steps; i += 1) {
    const k = at(i, 5);
    const tdx = at(i, 6);
    const tdy = at(i, 7);
    const s = segs[segs.length - 1] as Segment | undefined;
    if (s && s.k === k && s.tdx === tdx && s.tdy === tdy) {
      s.last = i;
    } else {
      segs.push({ first: i, last: i, k, tdx, tdy, u: [0, 0, 0], beta: 0, P: [] });
    }
    segOf[i] = segs.length - 1;
  }
  return { segs, segOf };
}

/** The tumble direction of a stretch, in the frame without the flat spin (the spin is turned back at the end). */
function directionIn(seg: Segment, angleAtEnd: number, angleEnd: number): [number, number] {
  const psi = -(angleAtEnd - angleEnd);
  const c = Math.cos(-psi);
  const sn = Math.sin(-psi);
  const wx = seg.tdx;
  const wy = -seg.tdy;
  return [wx * c - wy * sn, wx * sn + wy * c];
}

/** Gives every stretch, last to first, the pose it starts from and the edge it tips over. */
function walkBack(
  body: Body,
  sim: Sim,
  j: number,
  segs: readonly Segment[],
  rest: Mat3,
  angleEnd: number,
): void {
  const poly = polyOf(body.kind);
  for (let s = segs.length - 1; s >= 0; s -= 1) {
    const seg = segs[s];
    const next = s + 1 < segs.length ? segs[s + 1] : undefined;
    const lastAngle = sim.frames[(seg.last * sim.n + j) * FRAME + 2];
    const [dx, dy] = directionIn(seg, lastAngle, angleEnd);
    if (next && next.k === seg.k + 1) {
      // it tipped over onto the next stretch's face: undo that over the next pose's trailing edge
      const e = edgeToward(poly, next.P, [-dx, -dy]);
      seg.u = [e.oy, -e.ox, 0]; // z × (−o): tipping forward, away from the trailing edge
      seg.beta = e.beta;
      seg.P = m3.mul(m3.rot(seg.u, -e.beta), next.P);
    } else {
      // it rocked on its leading edge and fell back (or this is the last stretch, at rest)
      const base = next ? next.P : rest;
      const e = edgeToward(poly, base, [dx, dy]);
      seg.u = [-e.oy, e.ox, 0]; // z × o
      seg.beta = e.beta;
      seg.P = base;
    }
  }
}

/** How each body is oriented over the whole roll. */
export function orient(bodies: readonly Body[], sim: Sim, seed: number): Orientation[] {
  const rng = mulberry32((seed ^ 0x9e3779b9) >>> 0);
  const { frames, steps, n } = sim;
  return bodies.map((b, j) => {
    const poly = polyOf(b.kind);
    const layout = layoutOf(b.kind, b.labels);
    const { segs, segOf } = segmentsOf(sim, j);
    const angleEnd = frames[(steps * n + j) * FRAME + 2];
    const rest = finalPose(poly, layout, b.target, (rng() * 2 - 1) * REST_TILT);
    walkBack(b, sim, j, segs, rest, angleEnd);
    return {
      poly,
      layout,
      segs,
      segOf,
      angleEnd,
      rest,
      tilt: tiltOf(poly, layout, b.target, rest),
    };
  });
}

/** For tuning the sound: the events by kind, and the longest stretch with a die rolling and no event. */
function soundStats(sim: Sim): { byKind: Record<string, number>; quietMs: number } {
  const byKind: Record<string, number> = {};
  for (const e of sim.events) {
    byKind[e.kind] = (byKind[e.kind] ?? 0) + 1;
  }
  let quiet = 0;
  let last = 0;
  let next = 0;
  for (let i = 0; i < sim.rolling.length; i += 1) {
    const t = i * PHYS.dt;
    while (next < sim.events.length && sim.events[next].t <= t) {
      last = sim.events[next].t;
      next += 1;
    }
    if (sim.rolling[i] > 0.06) {
      quiet = Math.max(quiet, t - last); // some die rolls faster than 30 px/s
    }
  }
  return { byKind, quietMs: Math.round(quiet * 1000) };
}

/**
 * The roll a tray of this size would show, without drawing it: pure, so it runs in a test. The faces are read back
 * from the final pose of each die.
 */
export function simulate(
  dice: readonly DieSpec[],
  { width = 480, height = 260, seed = 1 }: { width?: number; height?: number; seed?: number } = {},
): SimulationReport {
  const bodies = expand(dice);
  const sim = simulateBodies(bodies, { width, height, seed: seed >>> 0 });
  const plan = orient(bodies, sim, seed >>> 0);
  return {
    durationMs: sim.duration * 1000,
    forced: sim.forced,
    flips: sim.flips.slice(),
    finalFaces: plan.map((o) => {
      const last = o.segs[o.segs.length - 1];
      return topLabel(o.poly, o.layout, last.P);
    }),
    events: sim.events.length,
    sound: soundStats(sim),
    dieRadius: sim.R,
    steps: sim.steps,
    restTilts: plan.map((o) => o.tilt),
  };
}
