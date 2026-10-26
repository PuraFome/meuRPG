/* eslint-disable @typescript-eslint/no-magic-numbers -- the synthesis of each sound: frequencies, decays and mixing gains */
/*
 * Every sound is synthesized when the sound is first switched on; nothing is loaded. Resin on resin (dice), resin
 * on the wooden rim (wall), a landing on the felt (floor), a face tipping onto the felt as a die rolls on (roll),
 * four takes of each, plus a low rumble and the answer of a small room.
 */
import { mulberry32 } from './dice-math';

/** Synthesized variants of each sound, so a repeat never sounds the same. */
export const TAKES = 4;
/** Hz: the rumble's band (a phone's speaker plays next to nothing below about 300 Hz). */
export const RUMBLE_BAND = 600;

export interface Sounds {
  readonly dice: readonly AudioBuffer[];
  readonly wall: readonly AudioBuffer[];
  readonly floor: readonly AudioBuffer[];
  readonly roll: readonly AudioBuffer[];
  readonly rumble: AudioBuffer;
  readonly room: AudioBuffer;
}

type Partial3 = readonly [number, number, number];

/** The helpers a sound is built from, bound to a sample rate and a generator. */
function toolbox(ac: BaseAudioContext, rng: () => number) {
  const sr = ac.sampleRate;
  const len = (seconds: number): number => Math.ceil(seconds * sr);
  const peak1 = (a: Float32Array): Float32Array => {
    let m = 0;
    for (const v of a) {
      m = Math.max(m, Math.abs(v));
    }
    if (m > 0) {
      for (let n = 0; n < a.length; n += 1) {
        a[n] /= m;
      }
    }
    return a;
  };
  return {
    // damped partials [amplitude, Hz, decay s]: the ring of a small hard body
    modes(seconds: number, list: readonly Partial3[]): Float32Array {
      const out = new Float32Array(len(seconds));
      for (const [amp, f, tau] of list) {
        const ph = rng() * 2 * Math.PI;
        for (let n = 0; n < out.length; n += 1) {
          const env = Math.exp(-n / sr / tau);
          if (env < 1e-4) {
            break;
          }
          out[n] += amp * env * Math.sin((2 * Math.PI * f * n) / sr + ph);
        }
      }
      return peak1(out);
    },
    // a noise burst, low-passed (cut > 0) or high-passed (cut < 0) with one pole, fading with tau
    burst(seconds: number, tau: number, cut: number): Float32Array {
      const out = new Float32Array(len(seconds));
      const a = Math.exp((-2 * Math.PI * Math.abs(cut)) / sr);
      let lp = 0;
      for (let n = 0; n < out.length; n += 1) {
        const w = rng() * 2 - 1;
        lp = (1 - a) * w + a * lp;
        out[n] = Math.exp(-n / sr / tau) * (cut < 0 ? w - lp : lp);
      }
      return peak1(out);
    },
    // a body landing on cloth: a short sine whose pitch falls from f0 to f1 Hz
    thump(seconds: number, f0: number, f1: number, tau: number): Float32Array {
      const out = new Float32Array(len(seconds));
      let ph = 0;
      for (let n = 0; n < out.length; n += 1) {
        const tt = n / sr;
        ph += (2 * Math.PI * (f1 + (f0 - f1) * Math.exp(-tt / 0.01))) / sr;
        out[n] = Math.exp(-tt / tau) * Math.sin(ph);
      }
      return peak1(out);
    },
    // the layers of one sound, mixed and normalized, with no click at the end
    buffer(parts: readonly (readonly [number, Float32Array])[]): AudioBuffer {
      const n = Math.max(...parts.map(([, a]) => a.length));
      const mixed = new Float32Array(n);
      for (const [g, a] of parts) {
        for (let i = 0; i < a.length; i += 1) {
          mixed[i] += g * a[i];
        }
      }
      peak1(mixed);
      const fade = Math.min(n, len(0.004));
      for (let i = 0; i < fade; i += 1) {
        mixed[n - 1 - i] *= i / fade;
      }
      const buf = ac.createBuffer(1, n, sr);
      buf.getChannelData(0).set(mixed);
      return buf;
    },
    // each take a little apart from the others in pitch (±5 %)
    takes(make: (v: number, j: number) => AudioBuffer): AudioBuffer[] {
      return Array.from({ length: TAKES }, (_, v) => make(v, 1 + (rng() - 0.5) * 0.1));
    },
  };
}

/**
 * 1.5 s of noise in the rumble's band, looping without a seam (its end fades into its start), at a known
 * loudness (RMS 0.25), so the rumble's gain means the same whatever the band.
 */
function rumbleLoop(ac: BaseAudioContext, rng: () => number): AudioBuffer {
  const sr = ac.sampleRate;
  const n = Math.ceil(1.5 * sr);
  const fade = Math.ceil(0.1 * sr);
  const warm = Math.ceil(0.05 * sr); // the filter settles before anything is kept
  // a band-pass biquad (Robert Bristow-Johnson's cookbook, 0 dB at the centre)
  const w0 = (2 * Math.PI * RUMBLE_BAND) / sr;
  const alpha = Math.sin(w0) / (2 * 0.8);
  const a0 = 1 + alpha;
  const a1 = -2 * Math.cos(w0);
  const a2 = 1 - alpha;
  const raw = new Float32Array(n + fade);
  let x1 = 0;
  let x2 = 0;
  let y1 = 0;
  let y2 = 0;
  for (let i = -warm; i < raw.length; i += 1) {
    const x = rng() * 2 - 1;
    const y = (alpha * x - alpha * x2 - a1 * y1 - a2 * y2) / a0;
    x2 = x1;
    x1 = x;
    y2 = y1;
    y1 = y;
    if (i >= 0) {
      raw[i] = y;
    }
  }
  const out = raw.slice(0, n);
  for (let i = 0; i < fade; i += 1) {
    out[i] = raw[i] * (i / fade) + raw[n + i] * (1 - i / fade);
  }
  let sum = 0;
  let peak = 0;
  for (const v of out) {
    sum += v * v;
    peak = Math.max(peak, Math.abs(v));
  }
  const k = Math.min(0.25 / Math.sqrt(sum / n), 1 / peak);
  const buf = ac.createBuffer(1, n, sr);
  buf.getChannelData(0).set(out.map((v) => v * k));
  return buf;
}

/** A small room's answer (0.3 s, stereo): what makes many taps sound like one table. */
function roomResponse(ac: BaseAudioContext, rng: () => number): AudioBuffer {
  const sr = ac.sampleRate;
  const n = Math.ceil(0.3 * sr);
  const buf = ac.createBuffer(2, n, sr);
  for (let c = 0; c < 2; c += 1) {
    const d = buf.getChannelData(c);
    let lp = 0;
    for (let i = 0; i < n; i += 1) {
      lp = 0.6 * lp + 0.4 * (rng() * 2 - 1); // a little darker than white noise
      d[i] = lp * Math.exp(-i / sr / 0.06);
    }
  }
  return buf;
}

/** The sounds, synthesized once per audio context. */
export function synthSounds(ac: BaseAudioContext): Sounds {
  const rng = mulberry32(0x5eed);
  const { modes, burst, thump, buffer, takes } = toolbox(ac, rng);
  return {
    // two dice: a bright, short clack
    dice: takes((v, j) => {
      const f = (2500 + 260 * v) * j;
      return buffer([
        [
          0.75,
          modes(0.07, [
            [1, f, 0.02],
            [0.6, f * 1.59, 0.014],
            [0.42, f * 2.33, 0.01],
            [0.25, f * 3.1, 0.007],
          ]),
        ],
        [0.45, burst(0.01, 0.0012, -2000)],
      ]);
    }),
    // a die against the wooden rim: a knock, with the die's own click on top
    wall: takes((v, j) => {
      const w = (330 + 45 * v) * j;
      const f = (2300 + 200 * v) * j;
      return buffer([
        [
          0.8,
          modes(0.1, [
            [1, w, 0.03],
            [0.55, w * 2.63, 0.018],
            [0.3, w * 4.9, 0.009],
          ]),
        ],
        [
          0.35,
          modes(0.04, [
            [1, f, 0.012],
            [0.5, f * 1.62, 0.008],
            [0.3, f * 2.4, 0.006],
          ]),
        ],
        [0.35, burst(0.015, 0.002, 3000)],
      ]);
    }),
    // a die landing on the felt: a muffled thud and a little click
    floor: takes((v, j) =>
      buffer([
        [0.8, thump(0.08, 210 * j, 110 * j, 0.016)],
        [0.7, burst(0.05, 0.01, 1100)],
        [0.12, modes(0.02, [[1, (2600 + 150 * v) * j, 0.005]])],
      ]),
    ),
    // a face tipping onto the felt: softer and shorter (a rock is the same tap, played lower)
    roll: takes((v, j) =>
      buffer([
        [0.8, burst(0.03, 0.005, 800)],
        [0.5, thump(0.04, 170 * j, 120 * j, 0.008)],
        [0.1, modes(0.01, [[1, (3000 + 200 * v) * j, 0.003]])],
      ]),
    ),
    rumble: rumbleLoop(ac, rng),
    room: roomResponse(ac, rng),
  };
}
