import { DOCUMENT, DestroyRef, Injectable, InjectionToken, inject, signal } from '@angular/core';

import { PageTitle } from '../title/page-title';

/**
 * The turn alert (the master's request after the first session, "something like the old MSN nudge"): when the turn
 * PASSES to a combatant the viewer plays, the phone vibrates, a short chime sounds, the turn panel shakes and, in a hidden
 * tab, the title says "● Sua vez". Only on that transition: not on load, not on every stream update, not on an NPC's turn.
 * It reads nothing the player does not already receive (RN-10): the input is the player's own acting tab.
 */

/** The browser APIs the alert touches, behind one seam so a spec can fake each. */
export interface TurnAlertEnv {
  /** `navigator.vibrate`, when the browser has it (Android Chrome; iOS has none). */
  vibrate(pattern: readonly number[]): void;
  /** The audio context, made on first use; `null` when the browser has none. */
  audio(): AudioContext | null;
  /** The tab is in the background. */
  hidden(): boolean;
  /** The person asked the system for less motion. */
  reducedMotion(): boolean;
  /** Runs `cb` when the tab becomes visible; returns the way to stop. */
  onVisible(cb: () => void): () => void;
  /** Runs `cb` once, on the first tap or key (browsers only let audio start after one); returns the way to stop. */
  onGesture(cb: () => void): () => void;
}

export const TURN_ALERT_ENV = new InjectionToken<TurnAlertEnv>('TURN_ALERT_ENV', {
  providedIn: 'root',
  factory: browserEnv,
});

function browserEnv(): TurnAlertEnv {
  const doc = inject(DOCUMENT);
  const win = doc.defaultView;
  let context: AudioContext | null = null;
  return {
    vibrate: (pattern) => win?.navigator?.vibrate?.([...pattern]),
    audio: () => {
      if (!context) {
        const ctor =
          (win as unknown as { AudioContext?: typeof AudioContext }).AudioContext ??
          (win as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
        context = ctor ? new ctor() : null;
      }
      return context;
    },
    hidden: () => doc.hidden,
    reducedMotion: () => win?.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false,
    onVisible: (cb) => {
      const listener = () => {
        if (!doc.hidden) {
          cb();
        }
      };
      doc.addEventListener('visibilitychange', listener);
      return () => doc.removeEventListener('visibilitychange', listener);
    },
    onGesture: (cb) => {
      const events = ['pointerdown', 'keydown'];
      const stop = () => events.forEach((e) => doc.removeEventListener(e, run, true));
      const run = () => {
        stop();
        cb();
      };
      events.forEach((e) => doc.addEventListener(e, run, true));
      return stop;
    },
  };
}

/** The vibration: buzz, pause, buzz, in milliseconds. */
const BUZZ_MS = 200;
const BUZZ_PAUSE_MS = 100;
export const VIBRATION_PATTERN: readonly number[] = [BUZZ_MS, BUZZ_PAUSE_MS, BUZZ_MS];

/** The chime: two rising notes (E5, then A5) so it reads as "your turn", not as an error. Hz. */
const NOTE_E5_HZ = 659.25;
const NOTE_A5_HZ = 880;
export const CHIME_NOTES: readonly number[] = [NOTE_E5_HZ, NOTE_A5_HZ];
/** Seconds between the start of one note and the next. */
const NOTE_STEP_S = 0.14;
/** Seconds a note rings; the last note ends the chime at 0.14 + 0.2 = 0.34 s, well under the 0.6 s ceiling. */
const NOTE_LENGTH_S = 0.2;
/** The loudest a note gets: a soft tick, far below full scale. */
const NOTE_PEAK_GAIN = 0.07;
/** The gain the fade-out goes to (an exponential ramp cannot reach 0). */
const NOTE_FLOOR_GAIN = 0.0001;
/** Seconds of the attack, so the note does not click. */
const NOTE_ATTACK_S = 0.01;

/** How long the shake (or the static highlight under reduced motion) lasts, in ms; the CSS animation is as long. */
export const SHAKE_MS = 500;

/** Who is on turn among those the viewer plays, or nothing. */
export interface TurnAlertTurn {
  /** What identifies the acting group (the tab id), so a turn passing from the character to its creature alerts too. */
  readonly key: string;
  /** "Pensantus", "Lobos atrozes (2)": for the log; the announcement is the turn panel's own live region. */
  readonly label: string;
}

@Injectable({ providedIn: 'root' })
export class TurnAlert {
  private readonly env = inject(TURN_ALERT_ENV);
  private readonly pageTitle = inject(PageTitle);

  /**
   * Sound and vibration are on (the switch). The shake and the title are not switched off by it. It lives in this root
   * service, so it holds until the page reloads: nothing is kept on the device, because the app stores nothing there but
   * the session cookie (docs/privacy.md, "No Web Storage").
   */
  readonly enabled = signal(true);
  /** The turn panel shakes now. */
  readonly shaking = signal(false);
  /** The static highlight that replaces the shake under reduced motion. */
  readonly highlighted = signal(false);

  /** `undefined`: nothing seen yet since the page began, so the first look is only the baseline. */
  private previous: string | null | undefined;
  private timer: ReturnType<typeof setTimeout> | undefined;

  constructor() {
    const destroy = inject(DestroyRef);
    // A tab that comes back to the front has seen the alert: the title goes back.
    destroy.onDestroy(this.env.onVisible(() => this.pageTitle.setTurnAlert(false)));
    // The first tap or key lets the browser start audio: make the context now, so the chime can play later.
    destroy.onDestroy(this.env.onGesture(() => this.prime()));
  }

  /** The combat view calls this with the player's acting group on every change; it alerts only on the pass. */
  observe(turn: TurnAlertTurn | null): void {
    const key = turn?.key ?? null;
    const before = this.previous;
    this.previous = key;
    if (before === undefined) {
      return;
    }
    if (key === null) {
      this.pageTitle.setTurnAlert(false);
      return;
    }
    if (key !== before) {
      this.alert();
    }
  }

  /** The combat view went away: forget the baseline, so the next one starts from what is on the screen. */
  reset(): void {
    this.previous = undefined;
    clearTimeout(this.timer);
    this.shaking.set(false);
    this.highlighted.set(false);
    this.pageTitle.setTurnAlert(false);
  }

  setEnabled(on: boolean): void {
    this.enabled.set(on);
    if (on) {
      this.prime();
    }
  }

  private alert(): void {
    if (this.enabled()) {
      this.vibrate();
      this.chime();
    }
    this.nudge();
    if (this.env.hidden()) {
      this.pageTitle.setTurnAlert(true);
    }
  }

  private nudge(): void {
    clearTimeout(this.timer);
    const still = this.env.reducedMotion();
    this.shaking.set(!still);
    this.highlighted.set(still);
    this.timer = setTimeout(() => {
      this.shaking.set(false);
      this.highlighted.set(false);
    }, SHAKE_MS);
  }

  private vibrate(): void {
    try {
      this.env.vibrate(VIBRATION_PATTERN);
    } catch {
      // A browser that refuses (no gesture yet): the alert goes on without it.
    }
  }

  /** Creates the audio context and wakes it, called from a tap (a switch, the first touch). */
  private prime(): void {
    try {
      const ctx = this.env.audio();
      if (ctx?.state === 'suspended') {
        void ctx.resume().catch(() => undefined);
      }
    } catch {
      // No audio here: the chime is skipped silently.
    }
  }

  /** Plays the two notes, but only when the context is already running: a late chime after a delayed resume would be stale. */
  private chime(): void {
    try {
      const ctx = this.env.audio();
      if (!ctx || ctx.state !== 'running') {
        return;
      }
      const start = ctx.currentTime;
      CHIME_NOTES.forEach((hz, i) => {
        const at = start + i * NOTE_STEP_S;
        const osc = ctx.createOscillator();
        const gain = ctx.createGain();
        osc.type = 'sine';
        osc.frequency.value = hz;
        gain.gain.setValueAtTime(NOTE_FLOOR_GAIN, at);
        gain.gain.exponentialRampToValueAtTime(NOTE_PEAK_GAIN, at + NOTE_ATTACK_S);
        gain.gain.exponentialRampToValueAtTime(NOTE_FLOOR_GAIN, at + NOTE_LENGTH_S);
        osc.connect(gain);
        gain.connect(ctx.destination);
        osc.start(at);
        osc.stop(at + NOTE_LENGTH_S);
      });
    } catch {
      // Silent: the vibration, the shake and the title still say it.
    }
  }
}
