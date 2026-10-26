/* eslint-disable @typescript-eslint/no-magic-numbers -- the physics of a die on felt, tuned by feel (see PHYS and KIND) */
import { clamp } from './dice-math';
import type { Box, DieState, SoundEvent, SoundKind } from './dice-types';

/*
 * Physics, in tray pixels and seconds, for a die 24 px in radius. Lengths, speeds and accelerations are multiplied by
 * (die radius / 24), so a phone tray rolls in the same time as a laptop one.
 */
export const PHYS = Object.freeze({
  /** Fixed step (s): the same seed gives the same roll at any frame rate. */
  dt: 1 / 120,
  /** px/s²: pulls a die in the air back to the felt. */
  gravity: 2400,
  /** px above the felt where a die enters. */
  throwHeight: [34, 52] as const,
  /** px/s: its vertical speed as it enters. */
  throwLift: [-20, 70] as const,
  /** px/s across the tray (a wide range: some rolls stop mid-tray, some hit the far wall). */
  throwSpeed: [480, 820] as const,
  /** rad/s: turning flat on the table. */
  throwSpin: [3, 11] as const,
  /** faces/s while in the air. */
  airTumble: [7, 13] as const,
  /** px/s: a landing slower than this does not bounce. */
  landQuiet: 120,
  /** Vertical restitution when a die lands. */
  floorBounce: 0.36,
  /** Share of the horizontal speed kept on a bouncing landing. */
  landKeep: 0.88,
  /** Restitution against the walls. */
  wallBounce: 0.5,
  /** Coulomb friction at the walls: a glancing hit turns into spin. */
  wallFriction: 0.3,
  /** Restitution die against die. */
  diceBounce: 0.45,
  /** Friction die against die (turns into spin on both). */
  diceFriction: 0.2,
  /** px/s²: rolling resistance of the felt. */
  rollDecel: 220,
  /** 1/s: a small viscous part on top of it. */
  rollDrag: 0.6,
  /** rad/s²: friction on the flat spin. */
  spinDecel: 9,
  spinDrag: 1.2,
  /** 1/s: how firmly the tumble follows the rolling speed. */
  tipCouple: 22,
  /** faces/s: slower than this, a die pivots on its edge instead of rolling. */
  gripSpeed: 2.5,
  /** faces/s²: balanced on an edge, it falls back before half-way, over after it. */
  tipGravity: 22,
  tipDrag: 3,
  /** 1/s: extra drag on the tumble of a die that has (nearly) stopped rolling. */
  tipSettle: 4,
  /** Share of the tumble speed kept when a rocking die slaps back onto its face. */
  faceSlap: 0.4,
  /** s: after this the felt brakes harder, so a roll ends by about 2 s. */
  softEnd: 1.6,
  /** 1/s²: how fast that extra drag grows. */
  brake: 10,
  /** s: nothing is simulated past this; a die still moving is laid down. */
  hardEnd: 3,
  /** px/s: slower than this (and flat, and still) a die is at rest. */
  restSpeed: 3,
  restSpin: 0.3,
  /** px/s: an impact harder than this is an event (sound). */
  clickSpeed: 90,
  /** px/s: a landing too soft to bounce still taps the felt above this. */
  landTick: 40,
  /** The softest face-onto-felt tap, as a share of the hardest hit. */
  rollTap: 0.2,
  /** faces/s: a die rocking back onto its face faster than this taps the felt. */
  rockTick: 0.8,
  /** px/s: a die rolling this fast rumbles at full. */
  rumbleSpeed: 500,
});

/*
 * Per die: step = distance rolled per face, in die radii (a round d20 tips over far more often than a cube);
 * keep = share of the speed kept each time it lands on a new face on the felt (a d4 stops dead, a d20 runs on);
 * wall / hit = collision radius against the walls and against other dice, in die radii;
 * pitch = how its size moves the pitch of its sounds (a d4 is small and light, a d20 big and round).
 */
export const KIND_PHYS = Object.freeze({
  4: { step: 1.7, keep: 0.66, wall: 1.1, hit: 1.04, pitch: 1.14 },
  6: { step: 1.5, keep: 0.78, wall: 1.05, hit: 1.0, pitch: 1.0 },
  8: { step: 1.3, keep: 0.83, wall: 1.1, hit: 1.04, pitch: 1.06 },
  10: { step: 1.2, keep: 0.85, wall: 1.1, hit: 1.04, pitch: 1.03 },
  12: { step: 1.05, keep: 0.88, wall: 1.1, hit: 1.05, pitch: 0.96 },
  20: { step: 0.9, keep: 0.91, wall: 1.2, hit: 1.14, pitch: 0.9 },
});

/** Moment of inertia / (mass · r²): between a disc (0.5) and a ring of corners. */
const INERTIA = 0.4;
/** The impact squash fades in about 0.15 s. */
const SQUASH_KEEP = Math.exp(-16 * PHYS.dt);

/** Everything a step needs besides the dice. */
export interface World {
  readonly box: Box;
  /** Scale of lengths: die radius / 24. */
  readonly S: number;
  readonly events: SoundEvent[];
}

function emit(
  w: World,
  t: number,
  s: number,
  kind: SoundKind,
  a: DieState,
  b: DieState | null = null,
): void {
  const { box } = w;
  const x = b ? (a.x + b.x) / 2 : a.x;
  w.events.push({
    t,
    s: Math.min(1, s),
    kind,
    i: a.i,
    j: b ? b.i : -1,
    pitch: b ? (a.pitch + b.pitch) / 2 : a.pitch,
    x: clamp((x - box.l) / (box.r - box.l), 0, 1),
  });
}

/** Sets the direction the die tips toward from its velocity; false if it has none. */
export function aim(b: DieState): boolean {
  const sp = Math.hypot(b.vx, b.vy);
  if (sp > 1e-6) {
    b.tdx = b.vx / sp;
    b.tdy = b.vy / sp;
    return true;
  }
  return false;
}

function squash(b: DieState, amount: number, angle: number): void {
  if (amount > b.sq) {
    b.sq = amount;
    b.sqAng = angle;
  }
}

/** Height: a thrown die hops two or three times before it rolls. */
function hop(b: DieState, t: number, w: World): void {
  if (b.z <= 0 && b.vz <= 0) {
    return;
  }
  const { S } = w;
  b.vz -= PHYS.gravity * S * PHYS.dt;
  b.z += b.vz * PHYS.dt;
  if (b.z > 0) {
    return;
  }
  b.z = 0;
  const impact = -b.vz;
  if (impact > PHYS.landQuiet * S) {
    b.vz = impact * PHYS.floorBounce;
    b.vx *= PHYS.landKeep;
    b.vy *= PHYS.landKeep;
    squash(b, Math.min(0.14, impact / (3200 * S)), NaN);
    emit(w, t, impact / (900 * S), 'floor', b);
  } else {
    b.vz = 0;
    if (impact > PHYS.landTick * S) {
      emit(w, t, impact / (900 * S), 'floor', b);
    }
  }
}

/** The felt slows the die and its flat spin; past `softEnd` it brakes harder. */
function feltFriction(b: DieState, brake: number, S: number): void {
  const sp = Math.hypot(b.vx, b.vy);
  if (sp > 0) {
    const ns = Math.max(0, sp - (PHYS.rollDecel * S + (PHYS.rollDrag + brake) * sp) * PHYS.dt);
    b.vx *= ns / sp;
    b.vy *= ns / sp;
  }
  const as = Math.abs(b.spin);
  if (as > 0) {
    b.spin *= Math.max(0, as - (PHYS.spinDecel + (PHYS.spinDrag + brake) * as) * PHYS.dt) / as;
  }
}

const gripOf = (b: DieState): number =>
  Math.min(1, Math.hypot(b.vx, b.vy) / (b.stepLen * PHYS.gripSpeed));

/** On the felt, the tumble follows rolling without slipping; slowed down, the edge balance decides. */
function tumbleOnFelt(b: DieState): void {
  const roll = (b.vx * b.tdx + b.vy * b.tdy) / b.stepLen;
  const grip = gripOf(b);
  b.w += (roll - b.w) * Math.min(1, PHYS.tipCouple * grip * PHYS.dt);
  // never balanced on the edge for long: the push does not fade to nothing at half-way
  const lean = Math.cos(Math.PI * b.p);
  b.w -= PHYS.tipGravity * (Math.abs(lean) < 0.35 ? (lean < 0 ? -0.35 : 0.35) : lean) * PHYS.dt;
  b.w *= Math.max(0, 1 - (PHYS.tipDrag + PHYS.tipSettle * (1 - grip)) * PHYS.dt);
  b.p += b.w * PHYS.dt;
}

/** The die lands on a new face. */
function tipOver(b: DieState, grounded: boolean, t: number, w: World): void {
  b.p -= 1;
  b.k += 1;
  if (grounded) {
    // the new face taps the felt: the rolling sound is these taps, closer together the faster it rolls
    emit(w, t, PHYS.rollTap + Math.hypot(b.vx, b.vy) / (700 * w.S), 'roll', b);
    b.vx *= b.keep;
    b.vy *= b.keep;
    squash(b, 0.035, NaN);
    // a slow die falling onto its new face lands flat instead of tipping on toward the next one
    b.w *= PHYS.faceSlap + (1 - PHYS.faceSlap) * gripOf(b);
  }
  aim(b);
}

/**
 * Rocked back past flat: the face slaps the felt (losing most of the rock when slow), and it tips over the opposite
 * edge, or the way it is sliding.
 */
function rockBack(b: DieState, grounded: boolean, t: number, w: World): void {
  const sp = Math.hypot(b.vx, b.vy);
  const grip = grounded ? gripOf(b) : 1;
  if (grounded && Math.abs(b.w) > PHYS.rockTick) {
    emit(w, t, Math.abs(b.w) / 6, 'rock', b);
  }
  b.p = -b.p;
  b.w = -b.w * (PHYS.faceSlap + (1 - PHYS.faceSlap) * grip);
  if (!(grounded && sp > b.stepLen * 1.5 && aim(b))) {
    b.tdx = -b.tdx;
    b.tdy = -b.tdy;
  }
}

/*
 * The tumble: p goes from 0 (flat on face k) to 1 (flat on face k + 1), tipping over the edge that faces the
 * direction (tdx, tdy). On the felt it follows rolling without slipping; slowed down, the edge balance decides:
 * before half-way it falls back (and may rock onto the opposite edge), after it falls over.
 */
function tumble(b: DieState, grounded: boolean, t: number, w: World): void {
  if (grounded) {
    tumbleOnFelt(b);
  } else {
    b.p += b.w * PHYS.dt;
  }
  if (b.p >= 1) {
    tipOver(b, grounded, t, w);
  } else if (b.p < 0) {
    rockBack(b, grounded, t, w);
  }
}

/** Two dice: circles with restitution and friction (the friction spins both). */
function collide(a: DieState, b: DieState, t: number, w: World, report: boolean): void {
  if (a.entering || b.entering || (a.rest && b.rest) || Math.abs(a.z - b.z) > a.rh) {
    return;
  }
  const dx = b.x - a.x;
  const dy = b.y - a.y;
  const rr = a.rh + b.rh;
  const d2 = dx * dx + dy * dy;
  if (d2 >= rr * rr) {
    return;
  }
  const d = Math.sqrt(d2);
  const nx = d > 1e-6 ? dx / d : 1;
  const ny = d > 1e-6 ? dy / d : 0;
  const pen = (rr - d) / 2;
  a.x -= nx * pen;
  a.y -= ny * pen;
  b.x += nx * pen;
  b.y += ny * pen;
  // velocity of each contact point: v + spin × r
  const rvx = b.vx + b.spin * ny * b.rh - (a.vx - a.spin * ny * a.rh);
  const rvy = b.vy - b.spin * nx * b.rh - (a.vy + a.spin * nx * a.rh);
  const vn = rvx * nx + rvy * ny;
  if (vn >= 0) {
    return;
  }
  a.rest = false;
  b.rest = false;
  const jn = (-(1 + PHYS.diceBounce) * vn) / 2;
  a.vx -= jn * nx;
  a.vy -= jn * ny;
  b.vx += jn * nx;
  b.vy += jn * ny;
  const tx = -ny;
  const ty = nx;
  const vt = rvx * tx + rvy * ty;
  const lim = PHYS.diceFriction * jn;
  const jt = clamp(-vt / (2 + 2 / INERTIA), -lim, lim);
  a.vx -= jt * tx;
  a.vy -= jt * ty;
  b.vx += jt * tx;
  b.vy += jt * ty;
  a.spin -= jt / (INERTIA * a.rh);
  b.spin -= jt / (INERTIA * b.rh);
  const ang = Math.atan2(ny, nx);
  squash(a, Math.min(0.12, -vn / (3000 * w.S)), ang);
  squash(b, Math.min(0.12, -vn / (3000 * w.S)), ang);
  if (report && -vn > PHYS.clickSpeed * w.S) {
    emit(w, t, -vn / (700 * w.S), 'dice', a, b);
  }
}

/** A wall with inward normal (nx, ny): restitution, and friction at the contact point turned into spin. */
function wallHit(b: DieState, nx: number, ny: number, pen: number): number {
  b.x += nx * pen;
  b.y += ny * pen;
  const r = b.rw;
  const cvx = b.vx + b.spin * ny * r;
  const cvy = b.vy - b.spin * nx * r;
  const vn = cvx * nx + cvy * ny;
  if (vn >= 0) {
    return 0;
  }
  const jn = -(1 + PHYS.wallBounce) * vn;
  b.vx += jn * nx;
  b.vy += jn * ny;
  const tx = -ny;
  const ty = nx;
  const vt = cvx * tx + cvy * ty;
  const lim = PHYS.wallFriction * jn;
  const jt = clamp(-vt / (1 + 1 / INERTIA), -lim, lim);
  b.vx += jt * tx;
  b.vy += jt * ty;
  b.spin -= jt / (INERTIA * r);
  squash(b, Math.min(0.14, -vn / (2600 * (r / 24))), Math.atan2(ny, nx));
  return -vn;
}

/** A die coming in through an edge is inside once it has crossed it; until then it is kept moving in. */
function keepEntering(b: DieState, box: Box, S: number): void {
  const inside =
    b.entry === 'left'
      ? b.x - b.rw >= box.l
      : b.entry === 'right'
        ? b.x + b.rw <= box.r
        : b.y + b.rw <= box.b;
  if (inside) {
    b.entering = false;
    return;
  }
  // whatever happened, keep it coming in
  const min = 60 * S;
  if (b.entry === 'left' && b.vx < min) {
    b.vx = min;
  }
  if (b.entry === 'right' && b.vx > -min) {
    b.vx = -min;
  }
  if (b.entry === 'bottom' && b.vy > -min) {
    b.vy = -min;
  }
}

function walls(b: DieState, t: number, w: World): void {
  if (b.rest) {
    return;
  }
  const { box } = w;
  if (b.entering) {
    keepEntering(b, box, w.S);
  }
  const skip = b.entering ? b.entry : '';
  let hit = 0;
  if (skip !== 'left' && b.x - b.rw < box.l) {
    hit = Math.max(hit, wallHit(b, 1, 0, box.l - (b.x - b.rw)));
  }
  if (skip !== 'right' && b.x + b.rw > box.r) {
    hit = Math.max(hit, wallHit(b, -1, 0, b.x + b.rw - box.r));
  }
  if (b.y - b.rw < box.t) {
    hit = Math.max(hit, wallHit(b, 0, 1, box.t - (b.y - b.rw)));
  }
  if (skip !== 'bottom' && b.y + b.rw > box.b) {
    hit = Math.max(hit, wallHit(b, 0, -1, b.y + b.rw - box.b));
  }
  if (hit > PHYS.clickSpeed * w.S) {
    emit(w, t, hit / (700 * w.S), 'wall', b);
  }
}

function checkRest(b: DieState, S: number): void {
  if (b.rest || b.entering || b.z > 0 || b.vz !== 0) {
    return;
  }
  const slow = Math.hypot(b.vx, b.vy) < PHYS.restSpeed * S && Math.abs(b.spin) < PHYS.restSpin;
  if (slow && b.p < 0.02 && Math.abs(b.w) < 0.6) {
    b.rest = true;
    b.vx = 0;
    b.vy = 0;
    b.spin = 0;
    b.w = 0;
    b.p = 0;
  }
}

/** The hard cap: whatever still moves is laid down on the nearer face. */
export function layDown(b: DieState): void {
  if (b.p > 0.5) {
    b.k += 1;
  }
  b.p = 0;
  b.w = 0;
  b.vx = 0;
  b.vy = 0;
  b.spin = 0;
  b.z = 0;
  b.vz = 0;
  b.rest = true;
  b.entering = false;
}

/** One fixed step of every die at simulation time t. */
export function advance(st: readonly DieState[], t: number, w: World): void {
  const brake = t > PHYS.softEnd ? (t - PHYS.softEnd) * PHYS.brake : 0;
  for (const b of st) {
    if (b.rest) {
      continue;
    }
    hop(b, t, w);
    const grounded = b.z <= 0;
    if (grounded) {
      feltFriction(b, brake, w.S);
    }
    b.x += b.vx * PHYS.dt;
    b.y += b.vy * PHYS.dt;
    b.angle += b.spin * PHYS.dt;
    tumble(b, grounded, t, w);
    b.sq *= SQUASH_KEEP;
  }
  for (let pass = 0; pass < 2; pass += 1) {
    for (let i = 0; i < st.length; i += 1) {
      for (let j = i + 1; j < st.length; j += 1) {
        collide(st[i], st[j], t, w, pass === 0);
      }
    }
  }
  for (const b of st) {
    walls(b, t, w);
  }
  for (const b of st) {
    checkRest(b, w.S);
  }
}

/** How much rolling goes on, for the rumble: each die on the felt counts up to 1, at full speed. */
export function rollingNow(st: readonly DieState[], S: number): number {
  let sum = 0;
  for (const b of st) {
    if (!b.rest && b.z <= 0) {
      sum += Math.min(1, Math.hypot(b.vx, b.vy) / (PHYS.rumbleSpeed * S));
    }
  }
  return sum;
}
