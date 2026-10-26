import { vi } from 'vitest';

import { createTray } from './dice-tray';
import { parseColor, readTheme } from './dice-theme';

/** A 2d context that records what is drawn and answers the few questions the tray asks. */
function fakeContext() {
  const texts: string[] = [];
  const calls = new Map<string, number>();
  const target: Record<string, unknown> = {
    measureText: (t: string) => ({
      width: t.length * 8,
      actualBoundingBoxAscent: 8,
      actualBoundingBoxDescent: 0,
    }),
    createLinearGradient: () => ({ addColorStop: vi.fn() }),
    createRadialGradient: () => ({ addColorStop: vi.fn() }),
    fillText: (t: string) => {
      texts.push(t);
      calls.set('fillText', (calls.get('fillText') ?? 0) + 1);
    },
  };
  const ctx = new Proxy(target, {
    get(t, key: string) {
      if (key in t) {
        return t[key];
      }
      return (t[key] = (..._args: unknown[]) => {
        calls.set(key, (calls.get(key) ?? 0) + 1);
      });
    },
    set(t, key: string, value) {
      t[key] = value;
      return true;
    },
  }) as unknown as CanvasRenderingContext2D;
  return { ctx, texts, calls };
}

/** Drives requestAnimationFrame by hand. */
function fakeFrames() {
  let queue: FrameRequestCallback[] = [];
  vi.spyOn(window, 'requestAnimationFrame').mockImplementation((cb) => queue.push(cb));
  vi.spyOn(window, 'cancelAnimationFrame').mockImplementation(() => {
    queue = [];
  });
  return {
    pending: () => queue.length,
    /** Runs frames at 16 ms steps until `ms` of tray time have passed or the loop stops. */
    run(ms: number) {
      for (let t = 0; t <= ms && queue.length; t += 16) {
        const now = queue;
        queue = [];
        now.forEach((cb) => cb(t));
      }
    },
  };
}

function setup(options = {}) {
  const { ctx, texts, calls } = fakeContext();
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(ctx as never);
  const canvas = document.createElement('canvas');
  document.body.append(canvas);
  const frames = fakeFrames();
  const tray = createTray(canvas, { width: 358, height: 197, ...options });
  return { tray, canvas, texts, calls, frames };
}

/** A Web Audio context that makes no sound and counts what was asked of it. */
function fakeAudio() {
  const log = { contexts: 0, started: 0, stopped: 0, closed: 0 };
  const param = () => ({
    value: 0,
    setValueAtTime: vi.fn(),
    setTargetAtTime: vi.fn(),
    setValueCurveAtTime: vi.fn(),
  });
  const node = (extra: object = {}) => ({
    connect: (n: unknown) => n,
    disconnect: vi.fn(),
    ...extra,
  });
  class FakeContext {
    sampleRate = 8000;
    state = 'running';
    currentTime = 0;
    destination = node();
    constructor() {
      log.contexts += 1;
    }
    createBuffer(_channels: number, length: number) {
      const data = new Float32Array(length);
      return { length, getChannelData: () => data };
    }
    createGain() {
      return node({ gain: param() });
    }
    createDynamicsCompressor() {
      return node({
        threshold: param(),
        knee: param(),
        ratio: param(),
        attack: param(),
        release: param(),
      });
    }
    createConvolver() {
      return node({ buffer: null });
    }
    createBufferSource() {
      return node({
        buffer: null,
        loop: false,
        playbackRate: param(),
        start: () => (log.started += 1),
        stop: () => (log.stopped += 1),
      });
    }
    createBiquadFilter() {
      return node({ frequency: param(), type: '' });
    }
    createStereoPanner() {
      return node({ pan: param() });
    }
    resume() {
      return Promise.resolve();
    }
    close() {
      log.closed += 1;
      return Promise.resolve();
    }
  }
  Object.defineProperty(window, 'AudioContext', { value: FakeContext, configurable: true });
  return log;
}

afterEach(() => {
  vi.restoreAllMocks();
  Reflect.deleteProperty(window, 'AudioContext');
  document.body.innerHTML = '';
});

describe('the tray on a canvas', () => {
  it('draws the felt at once, and the dice with their result once they land', async () => {
    const { tray, texts, frames } = setup();
    const done = tray.roll([{ sides: 20, result: 17 }], { seed: 5 });
    expect(frames.pending()).toBe(1);
    frames.run(5000);
    const out = await done;
    expect(out).toMatchObject({
      cancelled: false,
      seed: 5,
      dice: [{ sides: 20, result: 17, discarded: false }],
    });
    expect(out.durationMs).toBeGreaterThan(300);
    expect(out.durationMs).toBeLessThanOrEqual(3000);
    expect(texts).toContain('17');
    tray.destroy();
  });

  it('keeps animating after the dice land, until the afterglow is over', async () => {
    const { tray, frames } = setup();
    const done = tray.roll([{ sides: 20, result: 20, critical: true }], { seed: 9 });
    frames.run(8000);
    await done;
    expect(frames.pending()).toBe(0);
    tray.destroy();
  });

  it('lands the roll at once on finish(), and a new roll cancels the one in flight', async () => {
    const { tray, texts, frames } = setup();
    const first = tray.roll([{ sides: 6, result: 4 }], { seed: 1 });
    const second = tray.roll([{ sides: 8, result: 3 }], { seed: 2 });
    expect((await first).cancelled).toBe(true);
    tray.finish();
    const out = await second;
    expect(out.cancelled).toBe(false);
    expect(texts).toContain('3');
    expect(frames.pending()).toBe(0);
    tray.finish();
    tray.destroy();
  });

  it('draws the dice at rest with no frame at all under reduced motion', async () => {
    const { tray, texts, frames } = setup({ reducedMotion: true });
    const out = await tray.roll(
      [
        { sides: 20, result: 14 },
        { sides: 20, result: 3, discarded: true },
        { sides: 100, result: 47 },
      ],
      { seed: 3 },
    );
    expect(out).toMatchObject({ cancelled: false, durationMs: 0 });
    expect(frames.pending()).toBe(0);
    expect(texts).toEqual(
      expect.arrayContaining(['14', '3', '40', '7', 'descartado', 'dezenas', 'unidades']),
    );
    tray.destroy();
  });

  it('shows "Crítico!" and "Falha crítica" in words, never colour alone', async () => {
    const crit = setup({ reducedMotion: true });
    await crit.tray.roll([{ sides: 20, result: 20, critical: true }]);
    expect(crit.texts).toContain('Crítico!');
    const fumble = setup({ reducedMotion: true });
    await fumble.tray.roll([{ sides: 20, result: 1, fumble: true }]);
    expect(fumble.texts).toContain('Falha crítica');
  });

  it('turns to reduced motion in the middle of a roll, resizes and repaints', async () => {
    const { tray, frames } = setup();
    const done = tray.roll([{ sides: 12, result: 5 }], { seed: 4 });
    frames.run(200);
    tray.setReducedMotion(true);
    expect((await done).cancelled).toBe(false);
    tray.resize(300, 165);
    expect(tray.size).toEqual({ width: 300, height: 165 });
    tray.refreshTheme();
    tray.redraw();
    tray.destroy();
  });

  it('rejects a die that does not exist, with the roll untouched', async () => {
    const { tray } = setup();
    await expect(tray.roll([{ sides: 6, result: 9 }])).rejects.toThrow(RangeError);
    await expect(tray.roll([])).rejects.toThrow(TypeError);
  });

  it('refuses a canvas with no 2d context', () => {
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(null);
    expect(() => createTray(document.createElement('canvas'))).toThrow(TypeError);
  });
});

describe('the tray sound', () => {
  it('is off by default: no audio context is made, not even by a whole roll', async () => {
    const audio = fakeAudio();
    const { tray, frames } = setup();
    const done = tray.roll(
      [
        { sides: 6, result: 2 },
        { sides: 6, result: 5 },
      ],
      { seed: 8 },
    );
    frames.run(5000);
    await done;
    expect(audio.contexts).toBe(0);
    expect(audio.started).toBe(0);
    tray.destroy();
  });

  it('plays the impacts of the roll once switched on, and stops them when switched off', async () => {
    const audio = fakeAudio();
    const { tray, frames } = setup();
    tray.setSound(true);
    expect(audio.contexts).toBe(1);
    const done = tray.roll(
      [
        { sides: 6, result: 2 },
        { sides: 6, result: 5 },
        { sides: 20, result: 11 },
      ],
      { seed: 8 },
    );
    frames.run(5000);
    await done;
    expect(audio.started).toBeGreaterThan(3);
    const again = tray.roll([{ sides: 8, result: 3 }], { seed: 9 });
    frames.run(100);
    tray.setSound(false);
    frames.run(5000);
    await again;
    tray.destroy();
    expect(audio.closed).toBe(1);
  });

  it('starts with the sound on when the tray is made so, and keeps quiet without Web Audio', async () => {
    const audio = fakeAudio();
    const loud = setup({ sound: true });
    expect(audio.contexts).toBe(1);
    loud.tray.destroy();
    Reflect.deleteProperty(window, 'AudioContext');
    const none = setup({ sound: true });
    const done = none.tray.roll([{ sides: 4, result: 3 }], { seed: 2 });
    none.frames.run(5000);
    await done;
    none.tray.setSound(true);
    none.tray.destroy();
  });
});

describe('the tray colours', () => {
  it('reads the browser colour forms and the hex ones', () => {
    expect(parseColor('rgb(10, 20, 30)')).toEqual([10, 20, 30]);
    expect(parseColor('rgba(10 20 30 / 0.5)')).toEqual([10, 20, 30]);
    expect(parseColor('color(srgb 1 0 0.5)')).toEqual([255, 0, 127.5]);
    expect(parseColor('#fff')).toEqual([255, 255, 255]);
    expect(parseColor('#1b2230')).toEqual([27, 34, 48]);
    expect(parseColor('light-dark(#fff, #000)')).toBeNull();
    expect(parseColor(undefined)).toBeNull();
  });

  it('falls back to the felt and paper of the light theme when the page has no tokens', () => {
    const theme = readTheme(document.createElement('canvas'));
    expect(theme.floor).toEqual(expect.any(Array));
    expect(theme.display).toContain('Alegreya');
  });
});
