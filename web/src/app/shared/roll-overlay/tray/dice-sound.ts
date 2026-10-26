/* eslint-disable @typescript-eslint/no-magic-numbers -- the mixing levels and timings of the sound, tuned by ear */
/*
 * Sound (Web Audio, only after a user gesture). The simulation already knows every impact, so each one is scheduled
 * on the audio clock a little ahead of the frame, not fired late. Everything goes through one compressor, with a
 * touch of a small room: many dice blend instead of piling up.
 */
import { PHYS } from './dice-physics';
import { synthSounds, type Sounds } from './dice-sound-synth';
import type { Sim, SoundEvent } from './dice-types';

const SOUND = Object.freeze({
  /** Gain of the hardest hit of each kind. */
  level: { dice: 0.5, wall: 0.55, floor: 0.45, roll: 0.3, rock: 0.2 },
  /** s of sound scheduled ahead of the frame. */
  lookahead: 0.12,
  /** s: drift past this re-syncs to the frame; an impact this late (a stalled tab) is skipped. */
  late: 0.05,
  /** s: one die sounds at most once in this time (unless the later hit is much harder). */
  sameDie: 0.018,
  /** Voices starting within `crowdWindow`: past this the soft ones (roll, rock) drop out. */
  crowd: 6,
  crowdWindow: 0.03,
  /** Gain of the rumble with the dice rolling at full speed (it sits under the taps). */
  rumble: 0.03,
  /** Hz: how often the rumble's level follows the simulation. */
  rumbleRate: 30,
  /** Gain of the small room. */
  wet: 0.14,
  /** How far left or right a die at the tray's edge sounds (0 = centre, 1 = one ear). */
  pan: 0.5,
  /** After the compressor (which adds its own make-up gain); quiet on purpose, half of what was first measured. */
  master: 0.55,
});

interface Mixer {
  readonly ac: AudioContext;
  readonly sounds: Sounds;
  readonly bus: GainNode;
}

/** The output chain: every roll → one compressor (and the small room into it) → the speakers. */
function mixer(ac: AudioContext): Mixer {
  const sounds = synthSounds(ac);
  const bus = ac.createGain();
  const comp = ac.createDynamicsCompressor();
  comp.threshold.value = -16;
  comp.knee.value = 12;
  comp.ratio.value = 3;
  comp.attack.value = 0.003;
  comp.release.value = 0.15;
  const room = ac.createConvolver();
  room.buffer = sounds.room;
  const wet = ac.createGain();
  wet.gain.value = SOUND.wet;
  const out = ac.createGain();
  out.gain.value = SOUND.master;
  bus.connect(comp);
  bus.connect(room).connect(wet).connect(comp);
  comp.connect(out).connect(ac.destination);
  return { ac, sounds, bus };
}

/** What a roll's sound does: schedule up to a frame, stop at once, or let the last taps ring out. */
export interface RollSound {
  play(t: number): void;
  stop(): void;
  end(): void;
}

/** One voice of a roll: a buffer, its pitch, tone, level and place across the tray. */
function voice(m: Mixer, gate: GainNode, e: SoundEvent, when: number, duck: number): void {
  const { ac, sounds } = m;
  const soft = e.kind === 'roll' || e.kind === 'rock';
  const set = sounds[soft ? 'roll' : e.kind];
  const src = ac.createBufferSource();
  src.buffer = set[(e.i * 3 + Math.round(e.t * 480)) % set.length];
  // a harder hit rings a little higher and brighter; a rock is the roll's tap, lower
  src.playbackRate.value = e.pitch * (0.95 + 0.1 * e.s) * (e.kind === 'rock' ? 0.85 : 1);
  const tone = ac.createBiquadFilter();
  tone.type = 'lowpass';
  tone.frequency.value = 1000 + 11000 * e.s ** 1.3;
  const g = ac.createGain();
  g.gain.value = SOUND.level[e.kind] * (0.1 + 0.9 * e.s) ** 1.6 * duck;
  let tail: AudioNode = src.connect(tone).connect(g);
  if (typeof ac.createStereoPanner === 'function') {
    const pan = ac.createStereoPanner();
    pan.pan.value = (e.x * 2 - 1) * SOUND.pan;
    tail = tail.connect(pan);
  }
  tail.connect(gate);
  src.start(when);
}

/** The rumble: looped band noise, its level following the simulation's rolling. */
function startRumble(m: Mixer, gate: GainNode, sim: Sim, speed: number, anchor: number): void {
  const { ac } = m;
  const step = Math.max(1, Math.round(1 / (SOUND.rumbleRate * PHYS.dt)));
  const count = Math.floor((sim.rolling.length - 1) / step) + 1;
  if (count < 2) {
    return;
  }
  const curve = new Float32Array(count + 1); // the last point is silence
  for (let i = 0; i < count; i += 1) {
    curve[i] = SOUND.rumble * (1 - Math.exp(-sim.rolling[i * step]));
  }
  const span = (count * step * PHYS.dt) / speed;
  const at = Math.max(ac.currentTime, anchor);
  const src = ac.createBufferSource();
  src.buffer = m.sounds.rumble;
  src.loop = true;
  const g = ac.createGain();
  g.gain.value = 0;
  src.connect(g).connect(gate);
  g.gain.setValueCurveAtTime(curve, at, span);
  src.start(at);
  src.stop(at + span + 0.05);
}

/**
 * The sound of one roll. play(t) schedules what happens up to a little past simulation time t (call it every frame);
 * stop() fades it out now (a new roll, finish(), the sound switched off); end() lets the last taps ring out.
 */
function soundTake(
  m: Mixer,
  sim: Sim,
  speed: number,
  later: (fn: () => void, ms: number) => void,
): RollSound {
  const { ac } = m;
  const gate = ac.createGain();
  gate.connect(m.bus);
  const lastAt = new Float64Array(sim.n).fill(-1);
  const lastS = new Float64Array(sim.n);
  const recent: number[] = [];
  let anchor = NaN; // the audio time of simulation time 0
  let next = 0;
  let rumbling = false;
  let done = false;

  /** Whether the voice sounds, and how much the crowd ducks it; null if it drops out. */
  const admit = (e: SoundEvent, when: number): number | null => {
    if (when - lastAt[e.i] < SOUND.sameDie && e.s < lastS[e.i] * 1.5) {
      return null; // the same die, a hair later
    }
    while (recent.length && when - recent[0] > SOUND.crowdWindow) {
      recent.shift();
    }
    const soft = e.kind === 'roll' || e.kind === 'rock';
    if (soft && recent.length >= SOUND.crowd) {
      return null;
    }
    recent.push(when);
    for (const die of [e.i, e.j]) {
      if (die >= 0) {
        lastAt[die] = when;
        lastS[die] = e.s;
      }
    }
    return recent.length > SOUND.crowd ? SOUND.crowd / recent.length : 1;
  };

  const drop = (ms: number): void => later(() => gate.disconnect(), ms);

  return {
    play(t) {
      if (done) {
        return;
      }
      const ev = sim.events;
      const horizon = t + SOUND.lookahead * speed;
      if (ac.state !== 'running') {
        // no clock to schedule on (the context is still starting): drop these, never pile them up
        while (next < ev.length && ev[next].t <= horizon) {
          next += 1;
        }
        return;
      }
      const now = ac.currentTime;
      if (Number.isNaN(anchor) || Math.abs(now - (anchor + t / speed)) > SOUND.late) {
        anchor = now - t / speed; // the first frame, or the page stalled (a hidden tab): follow the frame
      }
      if (!rumbling) {
        rumbling = true;
        startRumble(m, gate, sim, speed, anchor);
      }
      while (next < ev.length && ev[next].t <= horizon) {
        const e = ev[next];
        next += 1;
        if (e.t < t - SOUND.late * speed) {
          continue; // its moment has passed
        }
        const when = Math.max(now, anchor + e.t / speed);
        const duck = admit(e, when);
        if (duck !== null) {
          voice(m, gate, e, when, duck);
        }
      }
    },
    stop() {
      if (done) {
        return;
      }
      done = true;
      const now = ac.currentTime;
      gate.gain.setValueAtTime(1, now);
      gate.gain.setTargetAtTime(0, now, 0.015);
      drop(300);
    },
    end() {
      if (done) {
        return;
      }
      done = true;
      drop(1500); // the last taps and the room ring out first
    },
  };
}

/** The audio of a tray: created on the first tap that switches the sound on, closed with the tray. */
export class TrayAudio {
  private mix: Mixer | null = null;

  constructor(private readonly win: Window & typeof globalThis) {}

  /** Starts (or resumes) the audio context; call it from a tap. False if the browser has no Web Audio. */
  enable(): boolean {
    const AC = this.win.AudioContext as typeof AudioContext | undefined; // absent in an old browser
    if (!AC) {
      return false;
    }
    this.mix ??= mixer(new AC());
    if (this.mix.ac.state === 'suspended') {
      this.mix.ac.resume().catch(() => undefined);
    }
    return true;
  }

  /** The sound of a roll about to play, or null (no audio context yet). */
  take(sim: Sim, speed: number): RollSound | null {
    return this.mix
      ? soundTake(this.mix, sim, speed, (fn, ms) => this.win.setTimeout(fn, ms))
      : null;
  }

  close(): void {
    this.mix?.ac.close().catch(() => undefined);
    this.mix = null;
  }
}
