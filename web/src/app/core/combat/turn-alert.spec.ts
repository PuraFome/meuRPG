import { TestBed } from '@angular/core/testing';
import { afterEach, beforeEach, vi } from 'vitest';

import { PageTitle } from '../title/page-title';
import {
  CHIME_NOTES,
  SHAKE_MS,
  TURN_ALERT_ENV,
  TurnAlert,
  type TurnAlertEnv,
  VIBRATION_PATTERN,
} from './turn-alert';

class FakeParam {
  readonly calls: [string, number, number?][] = [];
  value = 0;
  setValueAtTime(v: number, t: number): void {
    this.calls.push(['set', v, t]);
  }
  exponentialRampToValueAtTime(v: number, t: number): void {
    this.calls.push(['ramp', v, t]);
  }
}

class FakeAudio {
  state = 'running';
  currentTime = 1;
  destination = {};
  resumed = 0;
  readonly oscillators: { frequency: FakeParam; started: number; stopped: number }[] = [];
  resume(): Promise<void> {
    this.resumed++;
    return Promise.resolve();
  }
  createOscillator() {
    const osc = {
      type: '',
      frequency: new FakeParam(),
      started: -1,
      stopped: -1,
      connect: () => undefined,
      start(t: number) {
        osc.started = t;
      },
      stop(t: number) {
        osc.stopped = t;
      },
    };
    this.oscillators.push(osc);
    return osc;
  }
  createGain() {
    return { gain: new FakeParam(), connect: () => undefined };
  }
}

interface Setup {
  readonly alert: TurnAlert;
  readonly vibrate: ReturnType<typeof vi.fn>;
  readonly audio: FakeAudio;
  readonly env: { hidden: boolean; reduced: boolean };
  readonly show: () => void;
  readonly tap: () => void;
}

function setup(over: { audio?: FakeAudio | null; vibrate?: boolean } = {}): Setup {
  const audio = over.audio === undefined ? new FakeAudio() : over.audio;
  const vibrate = vi.fn();
  const state = {
    hidden: false,
    reduced: false,
  };
  let visible: () => void = () => undefined;
  let gesture: () => void = () => undefined;
  const env: TurnAlertEnv = {
    vibrate: over.vibrate === false ? () => undefined : vibrate,
    audio: () => audio as unknown as AudioContext | null,
    hidden: () => state.hidden,
    reducedMotion: () => state.reduced,
    onVisible: (cb) => {
      visible = cb;
      return () => undefined;
    },
    onGesture: (cb) => {
      gesture = cb;
      return () => undefined;
    },
  };
  TestBed.configureTestingModule({ providers: [{ provide: TURN_ALERT_ENV, useValue: env }] });
  const alert = TestBed.inject(TurnAlert);
  return {
    alert,
    vibrate,
    audio: audio ?? new FakeAudio(),
    env: state,
    show: () => {
      state.hidden = false;
      visible();
    },
    tap: () => gesture(),
  };
}

const pen = { key: 'character', label: 'Pensantus' };
const wolves = { key: 'grp-1', label: 'Lobos atrozes (2)' };

describe('TurnAlert', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    TestBed.resetTestingModule();
  });
  afterEach(() => vi.useRealTimers());

  it('fires once on the pass to the viewer, and not on the first look', () => {
    const s = setup();
    s.alert.observe(pen); // the page loaded on their turn: the baseline
    expect(s.vibrate).not.toHaveBeenCalled();
    s.alert.observe(null);
    s.alert.observe(pen);
    expect(s.vibrate).toHaveBeenCalledTimes(1);
    expect(s.vibrate).toHaveBeenCalledWith(VIBRATION_PATTERN);
    expect(s.alert.shaking()).toBe(true);
  });

  it('does not fire again on repeated stream updates of the same turn', () => {
    const s = setup();
    s.alert.observe(null);
    s.alert.observe(pen);
    s.alert.observe({ ...pen });
    s.alert.observe(pen);
    expect(s.vibrate).toHaveBeenCalledTimes(1);
  });

  it('fires for a creature, and when the turn goes from the character to its creature', () => {
    const s = setup();
    s.alert.observe(null);
    s.alert.observe(wolves);
    s.alert.observe(pen);
    expect(s.vibrate).toHaveBeenCalledTimes(2);
  });

  it('does not fire on a turn that is not the viewer', () => {
    const s = setup();
    s.alert.observe(null);
    s.alert.observe(null);
    expect(s.vibrate).not.toHaveBeenCalled();
    expect(s.alert.shaking()).toBe(false);
  });

  it('forgets the baseline on reset, so a new page start is silent again', () => {
    const s = setup();
    s.alert.observe(null);
    s.alert.reset();
    s.alert.observe(pen);
    expect(s.vibrate).not.toHaveBeenCalled();
  });

  it('plays two rising notes, short, and only with the context running', () => {
    const s = setup();
    s.alert.observe(null);
    s.alert.observe(pen);
    expect(s.audio.oscillators.map((o) => o.frequency.value)).toEqual([...CHIME_NOTES]);
    const end = Math.max(...s.audio.oscillators.map((o) => o.stopped)) - s.audio.currentTime;
    expect(end).toBeLessThan(0.6);
    TestBed.resetTestingModule();
    const quiet = setup({ audio: Object.assign(new FakeAudio(), { state: 'suspended' }) });
    quiet.alert.observe(null);
    quiet.alert.observe(pen);
    expect(quiet.audio.oscillators).toHaveLength(0);
    expect(quiet.alert.shaking()).toBe(true);
  });

  it('wakes the audio context on the first tap', () => {
    const audio = Object.assign(new FakeAudio(), { state: 'suspended' });
    const s = setup({ audio });
    s.tap();
    expect(audio.resumed).toBe(1);
  });

  it('skips sound and vibration when the switch is off, but still shakes', () => {
    const s = setup();
    s.alert.setEnabled(false);
    expect(s.alert.enabled()).toBe(false);
    s.alert.observe(null);
    s.alert.observe(pen);
    expect(s.vibrate).not.toHaveBeenCalled();
    expect(s.audio.oscillators).toHaveLength(0);
    expect(s.alert.shaking()).toBe(true);
  });

  it('starts with the switch on and keeps it only in memory (no Web Storage: docs/privacy.md)', () => {
    const s = setup();
    expect(s.alert.enabled()).toBe(true);
    s.alert.setEnabled(false);
    expect(s.alert.enabled()).toBe(false);
    s.alert.setEnabled(true);
    expect(s.alert.enabled()).toBe(true);
  });

  it('survives a browser without vibrate or audio', () => {
    const s = setup({ audio: null });
    s.alert.observe(null);
    expect(() => s.alert.observe(pen)).not.toThrow();
    expect(s.alert.shaking()).toBe(true);
  });

  it('shakes for half a second, then stops', () => {
    const s = setup();
    s.alert.observe(null);
    s.alert.observe(pen);
    expect(s.alert.shaking()).toBe(true);
    vi.advanceTimersByTime(SHAKE_MS);
    expect(s.alert.shaking()).toBe(false);
  });

  it('does not shake under reduced motion: a static highlight instead', () => {
    const s = setup();
    s.env.reduced = true;
    s.alert.observe(null);
    s.alert.observe(pen);
    expect(s.alert.shaking()).toBe(false);
    expect(s.alert.highlighted()).toBe(true);
    vi.advanceTimersByTime(SHAKE_MS);
    expect(s.alert.highlighted()).toBe(false);
  });

  describe('the tab title', () => {
    it('says "● Sua vez" while the tab is hidden, and restores it when it shows', () => {
      const s = setup();
      TestBed.inject(PageTitle).begin('Sessão ao vivo');
      expect(document.title).toBe('Sessão ao vivo · MeuRPG');
      s.env.hidden = true;
      s.alert.observe(null);
      s.alert.observe(pen);
      expect(document.title).toBe('● Sua vez · MeuRPG');
      s.show();
      expect(document.title).toBe('Sessão ao vivo · MeuRPG');
    });

    it('is restored when the turn moves on', () => {
      const s = setup();
      TestBed.inject(PageTitle).begin('Sessão ao vivo');
      s.env.hidden = true;
      s.alert.observe(null);
      s.alert.observe(pen);
      s.alert.observe(null);
      expect(document.title).toBe('Sessão ao vivo · MeuRPG');
    });

    it('is left alone while the tab is visible', () => {
      const s = setup();
      TestBed.inject(PageTitle).begin('Sessão ao vivo');
      s.alert.observe(null);
      s.alert.observe(pen);
      expect(document.title).toBe('Sessão ao vivo · MeuRPG');
    });
  });
});
