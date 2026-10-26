/* eslint-disable @typescript-eslint/no-magic-numbers -- vector maths and a generator's published constants */
import type { Mat3, Vec3 } from './dice-types';

export const TAU = Math.PI * 2;

export const clamp = (v: number, lo: number, hi: number): number => (v < lo ? lo : v > hi ? hi : v);
export const between = (range: readonly [number, number], u: number): number =>
  range[0] + (range[1] - range[0]) * u;

/** A small, fast, seedable generator (mulberry32). */
export function mulberry32(seed: number): () => number {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

/** A seed for a screen that has none from the server (the result is always the server's). */
export function randomSeed(): number {
  try {
    return globalThis.crypto.getRandomValues(new Uint32Array(1))[0];
  } catch {
    return Math.floor(Math.random() * 4294967296);
  }
}

export const v3 = {
  sub: (a: Vec3, b: Vec3): Vec3 => [a[0] - b[0], a[1] - b[1], a[2] - b[2]],
  scale: (a: Vec3, s: number): Vec3 => [a[0] * s, a[1] * s, a[2] * s],
  dot: (a: Vec3, b: Vec3): number => a[0] * b[0] + a[1] * b[1] + a[2] * b[2],
  cross: (a: Vec3, b: Vec3): Vec3 => [
    a[1] * b[2] - a[2] * b[1],
    a[2] * b[0] - a[0] * b[2],
    a[0] * b[1] - a[1] * b[0],
  ],
  norm: (a: Vec3): Vec3 => {
    const l = Math.hypot(a[0], a[1], a[2]) || 1;
    return [a[0] / l, a[1] / l, a[2] / l];
  },
  mean: (pts: readonly Vec3[]): Vec3 => {
    const c = [0, 0, 0];
    for (const p of pts) {
      c[0] += p[0];
      c[1] += p[1];
      c[2] += p[2];
    }
    return [c[0] / pts.length, c[1] / pts.length, c[2] / pts.length];
  },
};

/** 3×3 rotation matrices, row-major arrays of 9. */
export const m3 = {
  mul(a: Mat3, b: Mat3): Mat3 {
    const r: number[] = [];
    for (let i = 0; i < 3; i += 1) {
      for (let j = 0; j < 3; j += 1) {
        r[i * 3 + j] = a[i * 3] * b[j] + a[i * 3 + 1] * b[3 + j] + a[i * 3 + 2] * b[6 + j];
      }
    }
    return r;
  },
  apply: (m: Mat3, v: Vec3): Vec3 => [
    m[0] * v[0] + m[1] * v[1] + m[2] * v[2],
    m[3] * v[0] + m[4] * v[1] + m[5] * v[2],
    m[6] * v[0] + m[7] * v[1] + m[8] * v[2],
  ],
  /** Rotation by angle a about the unit axis u (Rodrigues). */
  rot(u: Vec3, a: number): Mat3 {
    const c = Math.cos(a);
    const s = Math.sin(a);
    const t = 1 - c;
    const [x, y, z] = u;
    return [
      t * x * x + c,
      t * x * y - s * z,
      t * x * z + s * y,
      t * x * y + s * z,
      t * y * y + c,
      t * y * z - s * x,
      t * x * z - s * y,
      t * y * z + s * x,
      t * z * z + c,
    ];
  },
  rotZ(a: number): Mat3 {
    const c = Math.cos(a);
    const s = Math.sin(a);
    return [c, -s, 0, s, c, 0, 0, 0, 1];
  },
  /** The rotation that takes the body basis (r, u, n) onto the world basis (R, U, N). */
  basis(r: Vec3, u: Vec3, n: Vec3, R: Vec3, U: Vec3, N: Vec3): Mat3 {
    const out: number[] = [];
    for (let i = 0; i < 3; i += 1) {
      for (let j = 0; j < 3; j += 1) {
        out[i * 3 + j] = R[i] * r[j] + U[i] * u[j] + N[i] * n[j];
      }
    }
    return out;
  },
};
