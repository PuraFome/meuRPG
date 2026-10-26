/* eslint-disable @typescript-eslint/no-magic-numbers -- the geometry of the throw and the tray, tuned by feel */
import { between, clamp, mulberry32 } from './dice-math';
import { restRadius } from './dice-polyhedra';
import { KIND_PHYS, PHYS, advance, aim, layDown, rollingNow, type World } from './dice-physics';
import type { Body, Box, DieSpec, DieState, DieStyle, Kind, Sim, SimOptions } from './dice-types';

/** Floats recorded per die per step. */
export const FRAME = 10;
/** px: the tray's wall, drawn and collided. */
export const RIM = 9;
/** A die stops within ±10° of upright (0 = all aligned). */
export const REST_TILT = (10 * Math.PI) / 180;
/** The most dice a tray holds. */
export const MAX_ON_TRAY = 12;

export const SIDES: readonly number[] = [4, 6, 8, 10, 12, 20, 100];
const STYLES: readonly string[] = ['paper', 'garnet'];
const TENS = ['00', '10', '20', '30', '40', '50', '60', '70', '80', '90'];
const UNITS = ['0', '1', '2', '3', '4', '5', '6', '7', '8', '9'];
const FACES: Readonly<Record<Kind, readonly string[]>> = {
  4: ['1', '2', '3', '4'],
  6: ['1', '2', '3', '4', '5', '6'],
  8: Array.from({ length: 8 }, (_, i) => String(i + 1)),
  10: Array.from({ length: 10 }, (_, i) => String(i + 1)),
  12: Array.from({ length: 12 }, (_, i) => String(i + 1)),
  20: Array.from({ length: 20 }, (_, i) => String(i + 1)),
};

const styleOr = (given: string | undefined, fallback: DieStyle): DieStyle =>
  given && STYLES.includes(given) ? (given as DieStyle) : fallback;

/** The dice the caller asks for, as bodies (a d100 is two d10: tens and units). Throws on a die that does not exist. */
export function expand(dice: readonly DieSpec[]): Body[] {
  if (!Array.isArray(dice) || dice.length === 0) {
    throw new TypeError('DiceTray.roll: pass at least one die');
  }
  const out: Body[] = [];
  dice.forEach((d, source) => {
    const { sides, result } = d;
    if (!SIDES.includes(sides)) {
      throw new RangeError(`DiceTray.roll: d${String(sides)} is not a die`);
    }
    if (!Number.isInteger(result) || result < 1 || result > sides) {
      throw new RangeError(`DiceTray.roll: ${String(result)} does not fit a d${sides}`);
    }
    const base = {
      source,
      discarded: !!d.discarded,
      tag: d.tag ? String(d.tag) : '',
      critical: !!d.critical,
      fumble: !!d.fumble,
    };
    if (sides === 100) {
      out.push(
        {
          ...base,
          part: 'dezenas',
          kind: 10,
          labels: TENS,
          target: TENS[Math.floor((result % 100) / 10)],
          style: styleOr(d.style, 'garnet'),
        },
        {
          ...base,
          part: 'unidades',
          kind: 10,
          labels: UNITS,
          target: UNITS[result % 10],
          style: styleOr(d.unitsStyle, 'paper'),
        },
      );
    } else {
      out.push({
        ...base,
        part: '',
        kind: sides as Kind,
        labels: FACES[sides as Kind],
        target: String(result),
        style: styleOr(d.style, sides === 20 ? 'garnet' : 'paper'),
      });
    }
  });
  if (out.length > MAX_ON_TRAY) {
    throw new RangeError(`DiceTray.roll: at most ${MAX_ON_TRAY} dice fit the tray`);
  }
  return out;
}

/** The die radius for a tray and a number of dice: smaller when the tray gets crowded. */
export function dieRadius(width: number, height: number, count: number): number {
  const inner = Math.min(width, height) - 2 * RIM;
  return inner * 0.14 * (count > 6 ? 0.72 : count > 4 ? 0.8 : count > 2 ? 0.9 : 1);
}

type Entry = DieState['entry'];

function pickEdge(u: number): Entry {
  return u < 0.4 ? 'left' : u < 0.8 ? 'right' : 'bottom';
}

/** The order the dice take along the entry edge. */
function shuffledSlots(n: number, rng: () => number): number[] {
  const slots = Array.from({ length: n }, (_, i) => i);
  for (let i = n - 1; i > 0; i -= 1) {
    const j = Math.floor(rng() * (i + 1));
    [slots[i], slots[j]] = [slots[j], slots[i]];
  }
  return slots;
}

/** Puts one die in the air outside the edge it enters through, thrown toward the middle. */
function enterAt(
  b: DieState,
  edge: Entry,
  slot: number,
  box: Box,
  rng: () => number,
  S: number,
): void {
  const back = b.rw * (1.2 + rng() * 1.1);
  // from the player's side the tray is short: a softer throw, or every die ends against the far wall
  const speed = between(PHYS.throwSpeed, rng()) * S * (edge === 'bottom' ? 0.62 : 1);
  const spread = (rng() - 0.5) * 0.6;
  if (edge === 'bottom') {
    b.x = box.l + (box.r - box.l) * (0.15 + 0.7 * slot);
    b.y = box.b + back;
    const ang = spread + (((box.l + box.r) / 2 - b.x) / (box.r - box.l)) * 0.5;
    b.vx = speed * Math.sin(ang);
    b.vy = -speed * Math.cos(ang);
    return;
  }
  const dir = edge === 'left' ? 1 : -1;
  b.y = box.t + (box.b - box.t) * (0.18 + 0.64 * slot);
  b.x = edge === 'left' ? box.l - back : box.r + back;
  // aim a little toward the middle, so the dice do not hug a side wall
  const ang = spread + (((box.t + box.b) / 2 - b.y) / (box.b - box.t)) * 0.5;
  b.vx = dir * speed * Math.cos(ang);
  b.vy = speed * Math.sin(ang);
}

/** The dice come in through one edge, in the air, already tumbling, spread along it. */
function throwIn(st: readonly DieState[], rng: () => number, box: Box, S: number): void {
  const edge = pickEdge(rng());
  const n = st.length;
  const slots = shuffledSlots(n, rng);
  st.forEach((b, i) => {
    const slot = clamp((slots[i] + 0.5) / n + (rng() - 0.5) * 0.12, 0, 1);
    enterAt(b, edge, slot, box, rng, S);
    b.z = between(PHYS.throwHeight, rng()) * S;
    b.vz = between(PHYS.throwLift, rng()) * S;
    b.spin = between(PHYS.throwSpin, rng()) * (rng() < 0.5 ? -1 : 1);
    b.w = between(PHYS.airTumble, rng());
    b.p = rng(); // already part-way through a tumble when it comes in
    b.entry = edge;
    aim(b);
  });
}

function newState(i: number, kind: Kind, R: number): DieState {
  const k = KIND_PHYS[kind];
  return {
    i,
    // the walls stop the die's whole silhouette, not only its collision circle: it never rests over the rim
    rw: R * Math.max(k.wall, restRadius(kind)),
    rh: R * k.hit,
    stepLen: R * k.step,
    keep: k.keep,
    pitch: k.pitch,
    x: 0,
    y: 0,
    vx: 0,
    vy: 0,
    z: 0,
    vz: 0,
    angle: 0,
    spin: 0,
    p: 0,
    w: 0,
    k: 0,
    tdx: 1,
    tdy: 0,
    sq: 0,
    sqAng: NaN,
    entry: 'left',
    entering: true,
    rest: false,
  };
}

function record(frames: Float64Array, st: readonly DieState[], i: number): void {
  let o = i * st.length * FRAME;
  for (const b of st) {
    frames[o] = b.x;
    frames[o + 1] = b.y;
    frames[o + 2] = b.angle;
    frames[o + 3] = b.z;
    frames[o + 4] = b.p;
    frames[o + 5] = b.k;
    frames[o + 6] = b.tdx;
    frames[o + 7] = b.tdy;
    frames[o + 8] = b.sq;
    frames[o + 9] = b.sqAng;
    o += FRAME;
  }
}

/** The simulation: pure and deterministic (same dice, tray and seed: same frames). */
export function simulateBodies(bodies: readonly Body[], { width, height, seed }: SimOptions): Sim {
  const rng = mulberry32(seed);
  const n = bodies.length;
  const R = dieRadius(width, height, n);
  const S = R / 24;
  const box: Box = { l: RIM, t: RIM, r: width - RIM, b: height - RIM };
  const st = bodies.map((b, i) => newState(i, b.kind, R));
  const world: World = { box, S, events: [] };
  throwIn(st, rng, box, S);
  const maxSteps = Math.ceil(PHYS.hardEnd / PHYS.dt);
  const frames = new Float64Array((maxSteps + 1) * n * FRAME);
  const rolling = new Float32Array(maxSteps + 1);
  record(frames, st, 0);
  let steps = 0;
  while (steps < maxSteps && !st.every((b) => b.rest)) {
    advance(st, steps * PHYS.dt, world);
    steps += 1;
    record(frames, st, steps);
    rolling[steps] = rollingNow(st, S);
  }
  const forced = !st.every((b) => b.rest);
  if (forced) {
    st.forEach(layDown);
    record(frames, st, steps);
  }
  return {
    R,
    S,
    n,
    steps,
    frames,
    events: world.events,
    forced,
    rolling: rolling.subarray(0, steps + 1),
    duration: steps * PHYS.dt,
    flips: st.map((b) => b.k),
    angles: st.map((b) => b.angle),
  };
}
