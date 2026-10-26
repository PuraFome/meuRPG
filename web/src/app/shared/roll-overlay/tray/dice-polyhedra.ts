/* eslint-disable @typescript-eslint/no-magic-numbers -- the coordinates and proportions of the solids */
import { TAU, clamp, m3, mulberry32, v3 } from './dice-math';
import type { Corner, Edge, Face, Kind, Layout, Mat3, Poly, Vec3 } from './dice-types';

/*
 * The dice are real polyhedra. Seen from straight above (orthographic) with flat, banded shading they read as
 * drawings, but they really turn over their edges as they roll.
 * Coordinates: x right, y up (the screen's up), z toward the viewer.
 */

const PHI = (1 + Math.sqrt(5)) / 2;
export const KINDS: readonly Kind[] = [4, 6, 8, 10, 12, 20];

/** The radius of a die's silhouette at rest, in die radii: how far it reaches from its centre. */
export const restRadius = (kind: Kind): number => LOOK[kind].rest;

/*
 * Silhouette radius at rest, in die radii (R): sets each solid's size, so the dice look alike in size at rest;
 * and how big a face's number is, against the face's inradius.
 */
const LOOK: Readonly<Record<Kind, { rest: number; digits: number }>> = {
  4: { rest: 1.5, digits: 0.62 },
  6: { rest: 1.14, digits: 1.25 },
  8: { rest: 1.26, digits: 1.35 },
  10: { rest: 1.3, digits: 1.35 },
  12: { rest: 1.22, digits: 1.35 },
  20: { rest: 1.36, digits: 1.5 },
};

/** The faces of a convex solid from its face normals: the vertices furthest along each normal, in turn. */
function facesFromNormals(verts: readonly Vec3[], normals: readonly Vec3[]): number[][] {
  return normals.map((n0) => {
    const n = v3.norm(n0);
    let best = -Infinity;
    for (const v of verts) {
      best = Math.max(best, v3.dot(v, n));
    }
    return verts
      .map((v, i) => [i, v3.dot(v, n)] as const)
      .filter(([, d]) => d > best - 1e-6)
      .map(([i]) => i);
  });
}

/** Every sign combination of each point (zeros stay). */
function signs(list: readonly Vec3[]): Vec3[] {
  const out: Vec3[] = [];
  for (const p of list) {
    const [ox, oy, oz] = p.map((x) => (x === 0 ? [0] : [x, -x]));
    for (const a of ox) {
      for (const b of oy) {
        for (const c of oz) {
          out.push([a, b, c]);
        }
      }
    }
  }
  return out;
}

const icosa = (): Vec3[] =>
  signs([
    [0, 1, PHI],
    [1, PHI, 0],
    [PHI, 0, 1],
  ]);
// the dual of the icosahedron: its vertices sit over the icosahedron's face centres
const dodeca = (): Vec3[] =>
  signs([
    [1, 1, 1],
    [1 / PHI, 0, PHI],
    [0, PHI, 1 / PHI],
    [PHI, 1 / PHI, 0],
  ]);

interface Solid {
  readonly verts: Vec3[];
  readonly faceIdx: number[][];
}

/** A pentagonal trapezohedron: two apexes, two rings of five, ten kites. */
function trapezohedron(): Solid {
  // The ring height z0 and the apex height h keep each kite flat: h = z0 (1 + cos 36°) / (1 − cos 36°).
  const z0 = 0.105;
  const c36 = Math.cos(Math.PI / 5);
  const h = (z0 * (1 + c36)) / (1 - c36);
  const verts: Vec3[] = [
    [0, 0, h],
    [0, 0, -h],
  ];
  for (let k = 0; k < 5; k += 1) {
    verts.push([Math.cos((k * TAU) / 5), Math.sin((k * TAU) / 5), z0]);
  }
  for (let k = 0; k < 5; k += 1) {
    verts.push([Math.cos((k * TAU) / 5 + Math.PI / 5), Math.sin((k * TAU) / 5 + Math.PI / 5), -z0]);
  }
  const faceIdx: number[][] = [];
  for (let k = 0; k < 5; k += 1) {
    const k1 = (k + 1) % 5;
    faceIdx.push([0, 2 + k, 7 + k, 2 + k1], [1, 7 + k, 2 + k1, 7 + k1]);
  }
  return { verts, faceIdx };
}

function solidOf(kind: Kind): Solid {
  switch (kind) {
    case 4: {
      const verts: Vec3[] = [
        [1, 1, 1],
        [1, -1, -1],
        [-1, 1, -1],
        [-1, -1, 1],
      ];
      return {
        verts,
        faceIdx: facesFromNormals(
          verts,
          verts.map((v) => v3.scale(v, -1)),
        ),
      };
    }
    case 6: {
      const verts = signs([[1, 1, 1]]);
      const normals: Vec3[] = [
        [1, 0, 0],
        [-1, 0, 0],
        [0, 1, 0],
        [0, -1, 0],
        [0, 0, 1],
        [0, 0, -1],
      ];
      return { verts, faceIdx: facesFromNormals(verts, normals) };
    }
    case 8: {
      const verts: Vec3[] = [
        [1, 0, 0],
        [-1, 0, 0],
        [0, 1, 0],
        [0, -1, 0],
        [0, 0, 1],
        [0, 0, -1],
      ];
      return { verts, faceIdx: facesFromNormals(verts, signs([[1, 1, 1]])) };
    }
    case 12: {
      const verts = dodeca();
      return { verts, faceIdx: facesFromNormals(verts, icosa()) };
    }
    case 20: {
      const verts = icosa();
      return { verts, faceIdx: facesFromNormals(verts, dodeca()) };
    }
    default:
      return trapezohedron();
  }
}

/** Where a number's top points: the apex of a kite, a corner of a triangle or pentagon, an edge of a square. */
function towardOf(kind: Kind, verts: readonly Vec3[], idx: readonly number[]): Vec3 {
  if (kind === 10) {
    return verts[idx.find((i) => i < 2) ?? idx[0]];
  }
  if (kind === 6) {
    return v3.mean([verts[idx[0]], verts[idx[1]]]);
  }
  return verts[idx[0]];
}

/** The radius of the circle inside a face: its nearest edge to the centre. */
function inradius(verts: readonly Vec3[], idx: readonly number[], c: Vec3): number {
  let inr = Infinity;
  idx.forEach((a, i) => {
    const b = idx[(i + 1) % idx.length];
    const ab = v3.sub(verts[b], verts[a]);
    const ac = v3.sub(c, verts[a]);
    const t = v3.dot(ac, ab) / v3.dot(ab, ab);
    const foot: Vec3 = [verts[a][0] + ab[0] * t, verts[a][1] + ab[1] * t, verts[a][2] + ab[2] * t];
    inr = Math.min(inr, Math.hypot(...v3.sub(c, foot)));
  });
  return inr;
}

function faceOf(kind: Kind, verts: readonly Vec3[], idx0: readonly number[]): Face {
  const c = v3.mean(idx0.map((i) => verts[i]));
  let n = v3.norm(
    v3.cross(v3.sub(verts[idx0[1]], verts[idx0[0]]), v3.sub(verts[idx0[2]], verts[idx0[0]])),
  );
  if (v3.dot(n, c) < 0) {
    n = v3.scale(n, -1);
  }
  // counter-clockwise seen from outside
  const e1 = v3.norm(v3.sub(verts[idx0[0]], c));
  const e2 = v3.cross(n, e1);
  const angle = (i: number): number => {
    const d = v3.sub(verts[i], c);
    return Math.atan2(v3.dot(d, e2), v3.dot(d, e1));
  };
  const idx = idx0.slice().sort((a, b) => angle(a) - angle(b));
  const up = v3.norm(v3.sub(towardOf(kind, verts, idx), c));
  return { idx, c, n, up, right: v3.cross(up, n), inr: inradius(verts, idx, c), edges: [] };
}

/** The edges, the two faces on each, and the angle a die turns when it tips over that edge. */
function edgesOf(verts: readonly Vec3[], faces: readonly Face[]): Edge[] {
  const byKey = new Map<string, Edge>();
  faces.forEach((f, fi) => {
    f.idx.forEach((a, i) => {
      const b = f.idx[(i + 1) % f.idx.length];
      const key = a < b ? `${a}-${b}` : `${b}-${a}`;
      const known = byKey.get(key);
      if (known) {
        known.f1 = fi;
      } else {
        byKey.set(key, { a, b, f0: fi, f1: -1, beta: 0, mid: [0, 0, 0] });
      }
    });
  });
  const edges = [...byKey.values()];
  for (const e of edges) {
    e.beta = Math.acos(clamp(v3.dot(faces[e.f0].n, faces[e.f1].n), -1, 1));
    e.mid = v3.mean([verts[e.a], verts[e.b]]);
    faces[e.f0].edges.push(e);
    faces[e.f1].edges.push(e);
  }
  return edges;
}

/** A pose with face f flat on the table (its normal pointing down). */
function restingOn(f: Face): Mat3 {
  const n = f.n;
  const r = v3.norm(Math.abs(n[2]) < 0.9 ? v3.cross([0, 0, 1], n) : v3.cross([1, 0, 0], n));
  const u = v3.cross(n, r);
  return m3.basis(r, u, n, [1, 0, 0], [0, -1, 0], [0, 0, -1]);
}

/** A d4 shows its result on the top corner: the number goes on every face, near each corner. */
function cornersOf(verts: readonly Vec3[], faces: readonly Face[]): Corner[][] {
  return faces.map((f) =>
    f.idx.map((vi) => {
      const up = v3.norm(v3.sub(verts[vi], f.c));
      return {
        vi,
        at: v3.sub(f.c, v3.scale(v3.sub(f.c, verts[vi]), 0.3)),
        up,
        right: v3.cross(up, f.n),
      };
    }),
  );
}

function buildPoly(kind: Kind): Poly {
  const solid = solidOf(kind);
  const radius = Math.max(...solid.verts.map((v) => Math.hypot(v[0], v[1], v[2])));
  const verts = solid.verts.map((v) => v3.scale(v, 1 / radius));
  const faces = solid.faceIdx.map((idx) => faceOf(kind, verts, idx));
  const edges = edgesOf(verts, faces);
  // Sized so the silhouette, flat on a face, has the radius LOOK[kind].rest (in die radii).
  const flat = restingOn(faces[0]);
  const sil = Math.max(
    ...verts.map((v) => {
      const w = m3.apply(flat, v);
      return Math.hypot(w[0], w[1]);
    }),
  );
  return {
    kind,
    verts,
    faces,
    edges,
    size: LOOK[kind].rest / sil,
    digits: LOOK[kind].digits,
    corners: kind === 4 ? cornersOf(verts, faces) : null,
  };
}

const POLY = new Map<Kind, Poly>();

export function polyOf(kind: Kind): Poly {
  let poly = POLY.get(kind);
  if (!poly) {
    poly = buildPoly(kind);
    POLY.set(kind, poly);
  }
  return poly;
}

/*
 * Where the numbers go: a real layout, opposite faces adding up to the same total (1 and 20, 2 and 19...).
 * A d4 carries its numbers on the corners. The layout is the same on every roll, like a real die.
 */
const layouts = new Map<string, Layout>();

export function layoutOf(kind: Kind, values: readonly string[]): Layout {
  const key = `${kind}:${values.join(',')}`;
  const known = layouts.get(key);
  if (known) {
    return known;
  }
  const poly = polyOf(kind);
  let out: Layout;
  if (kind === 4) {
    out = { byCorner: true, labels: values.slice() };
  } else {
    const rng = mulberry32(kind * 7919);
    const order = poly.faces.map((_, i) => i);
    for (let i = order.length - 1; i > 0; i -= 1) {
      const j = Math.floor(rng() * (i + 1));
      [order[i], order[j]] = [order[j], order[i]];
    }
    const labels: string[] = new Array<string>(poly.faces.length);
    let next = 0;
    for (const fi of order) {
      if (labels[fi] !== undefined) {
        continue;
      }
      const n = poly.faces[fi].n;
      const opposite = poly.faces.findIndex((f) => v3.dot(f.n, n) < -0.999);
      labels[fi] = values[next];
      labels[opposite] = values[values.length - 1 - next];
      next += 1;
    }
    out = { byCorner: false, labels };
  }
  layouts.set(key, out);
  return out;
}

/** In the die's own frame: the direction the target number's top points (a d4: from the face's centre to the top corner). */
export function numberUp(
  poly: Poly,
  layout: Layout,
  target: string,
): { up: Vec3; right: Vec3; n: Vec3 } {
  if (layout.byCorner) {
    const vi = layout.labels.indexOf(target);
    const top = v3.norm(poly.verts[vi]);
    // one of the three faces around the top corner sits at the bottom, so its copy of the number reads upright
    const f = poly.faces.find((x) => x.idx.includes(vi)) ?? poly.faces[0];
    const d = v3.sub(poly.verts[vi], f.c);
    const h = v3.norm(v3.sub(d, v3.scale(top, v3.dot(d, top))));
    return { up: h, right: v3.cross(h, top), n: top };
  }
  const f = poly.faces[layout.labels.indexOf(target)];
  return { up: f.up, right: f.right, n: f.n };
}

/** The pose at rest showing `target` on top, its number upright, turned by `yaw` (radians). */
export function finalPose(poly: Poly, layout: Layout, target: string, yaw: number): Mat3 {
  const R: Vec3 = [Math.cos(yaw), Math.sin(yaw), 0];
  const U: Vec3 = [-Math.sin(yaw), Math.cos(yaw), 0];
  const f = numberUp(poly, layout, target);
  return m3.basis(f.right, f.up, f.n, R, U, [0, 0, 1]);
}

/** How far the target's number is turned from upright in a pose at rest (radians). */
export function tiltOf(poly: Poly, layout: Layout, target: string, pose: Mat3): number {
  const w = m3.apply(pose, numberUp(poly, layout, target).up);
  return Math.atan2(-w[0], w[1]);
}

/** The face (or corner, for a d4) that points up in a pose. */
export function topLabel(poly: Poly, layout: Layout, pose: Mat3): string {
  const points: readonly Vec3[] = layout.byCorner ? poly.verts : poly.faces.map((f) => f.n);
  let best = 0;
  let high = -Infinity;
  points.forEach((v, i) => {
    const z = m3.apply(pose, v)[2];
    if (z > high) {
      high = z;
      best = i;
    }
  });
  return layout.labels[best];
}

/** In a pose at rest, the edge of the bottom face whose outward direction (on the table) is nearest to d. */
export function edgeToward(
  poly: Poly,
  pose: Mat3,
  d: readonly [number, number],
): { ox: number; oy: number; beta: number } {
  let bottom = poly.faces[0];
  let low = Infinity;
  for (const f of poly.faces) {
    const z = m3.apply(pose, f.n)[2];
    if (z < low) {
      low = z;
      bottom = f;
    }
  }
  const c = m3.apply(pose, bottom.c);
  let best = { ox: 1, oy: 0, beta: 0 };
  let score = -Infinity;
  for (const e of bottom.edges) {
    // the outward direction square to the edge (a kite's centre is not square to its edges' midpoints)
    const m = m3.apply(pose, e.mid);
    const pa = m3.apply(pose, poly.verts[e.a]);
    const pb = m3.apply(pose, poly.verts[e.b]);
    const el = Math.hypot(pb[0] - pa[0], pb[1] - pa[1]) || 1;
    const ex = (pb[0] - pa[0]) / el;
    const ey = (pb[1] - pa[1]) / el;
    let ox = m[0] - c[0];
    let oy = m[1] - c[1];
    const along = ox * ex + oy * ey;
    ox -= along * ex;
    oy -= along * ey;
    const l = Math.hypot(ox, oy) || 1;
    ox /= l;
    oy /= l;
    const s = ox * d[0] + oy * d[1];
    if (s > score) {
      score = s;
      best = { ox, oy, beta: e.beta };
    }
  }
  return best;
}
