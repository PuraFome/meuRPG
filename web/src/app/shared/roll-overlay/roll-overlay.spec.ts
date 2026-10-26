import { TestBed } from '@angular/core/testing';
import { vi } from 'vitest';

import { nextTurn } from '../../../test-settle';
import { DiceSoundSetting } from './dice-sound-setting';
import { MAX_DICE, RollAnimator, type RollShow, trayDice } from './roll-animator';
import { RollOverlay } from './roll-overlay';
import { TRAY_LOADER, traySize } from './roll-tray';
import { MAX_ON_TRAY } from './tray/dice-sim';
import type { DieSpec, RollSummary } from './tray/dice-types';

/** The tray, as the overlay sees it: every call is recorded, and a roll lands when the test says so. */
interface FakeTray {
  options: Record<string, unknown>;
  rolls: unknown[][];
  reduced: boolean[];
  sounds: boolean[];
  finished: number;
  destroyed: boolean;
  land: () => void;
  size: { width: number; height: number };
}

const made: FakeTray[] = [];

function createFakeTray(_canvas: unknown, options: Record<string, unknown> = {}) {
  let resolve: (s: RollSummary) => void = () => undefined;
  const state: FakeTray = {
    options,
    rolls: [],
    reduced: [],
    sounds: [],
    finished: 0,
    destroyed: false,
    land: () => resolve({ cancelled: false, seed: 1, durationMs: 1200, dice: [] }),
    size: { width: options['width'] as number, height: options['height'] as number },
  };
  made.push(state);
  return {
    get size() {
      return state.size;
    },
    setReducedMotion: (on: boolean) => state.reduced.push(on),
    roll: (dice: unknown[]) => {
      state.rolls.push(dice);
      return new Promise<RollSummary>((r) => {
        resolve = r;
        if (state.reduced.at(-1)) {
          state.land(); // under reduced motion the dice simply appear
        }
      });
    },
    finish: () => {
      state.finished += 1;
      state.land();
    },
    resize: (width: number, height: number) => {
      state.size = { width, height };
    },
    setSound: (on: boolean) => state.sounds.push(on),
    refreshTheme: () => undefined,
    redraw: () => undefined,
    destroy: () => {
      state.destroyed = true;
    },
  };
}

// With --coverage test-setup.ts only hooks the first file of a worker, so the stubbed globals this file leaves on would
// reach the next file (a real setTimeout never fires on a fake clock).
afterEach(() => {
  vi.unstubAllGlobals();
  made.length = 0;
});

const attack: RollShow = {
  label: 'Ataque com Machado grande',
  dice: [{ sides: 20, face: 17 }],
  line: '17 + 5 = 22',
  outcome: { word: 'Acertou', good: true },
};

function setup(reduce = false) {
  vi.stubGlobal('matchMedia', (q: string) => ({ matches: reduce && q.includes('reduce') }));
  TestBed.configureTestingModule({
    providers: [
      {
        provide: TRAY_LOADER,
        useValue: () => Promise.resolve({ createTray: createFakeTray as never }),
      },
    ],
  });
  const fixture = TestBed.createComponent(RollOverlay);
  fixture.detectChanges();
  const animator = TestBed.inject(RollAnimator);
  const el = fixture.nativeElement as HTMLElement;
  /** Lets the module load, the tray start and the promises settle. */
  const settle = async () => {
    for (let i = 0; i < 4; i += 1) {
      await nextTurn();
      fixture.detectChanges();
    }
  };
  const step = (ms: number) => {
    vi.advanceTimersByTime(ms);
    fixture.detectChanges();
  };
  const tray = () => made.at(-1);
  return { fixture, animator, el, step, settle, tray };
}

describe('RollOverlay', () => {
  beforeEach(() => vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] }));
  afterEach(() => vi.useRealTimers());

  it('throws the dice on the tray and shows the words only once they have landed', async () => {
    const { animator, el, settle, tray } = setup();
    animator.play(attack);
    await settle();
    expect(el.querySelector('.scrim')).toBeTruthy();
    expect(el.querySelector('app-roll-tray canvas')).toBeTruthy();
    expect(tray()?.rolls).toEqual([
      [{ sides: 20, result: 17, discarded: false, critical: false, fumble: false }],
    ]);
    expect(el.querySelector('.result')).toBeNull();
    expect(el.querySelector('[role=status]')?.textContent?.trim()).toBe('');

    tray()?.land();
    await settle();
    expect(el.querySelector('.result')?.textContent).toContain('Ataque com Machado grande');
    expect(el.querySelector('.result')?.textContent).toContain('17 + 5 = 22');
    expect(el.querySelector('.result')?.textContent).toContain('Acertou');
    expect(el.querySelector('[role=status]')?.textContent).toContain(
      'Ataque com Machado grande: 17. 17 + 5 = 22. Acertou.',
    );
  });

  it('hides the picture from assistive technology and keeps the sentence in the live region', async () => {
    const { animator, el, settle, tray } = setup();
    animator.play(attack);
    await settle();
    tray()?.land();
    await settle();
    expect(el.querySelector('app-roll-tray canvas')?.getAttribute('aria-hidden')).toBe('true');
    expect(el.querySelector('.veil')?.getAttribute('aria-hidden')).toBe('true');
    expect(el.querySelector('.result')?.getAttribute('aria-hidden')).toBe('true');
    expect(el.querySelector('p.sr')?.getAttribute('aria-live')).toBe('polite');
  });

  it('lands the dice at once on a tap, and the second tap closes', async () => {
    const { animator, el, settle, tray } = setup();
    animator.play(attack);
    await settle();
    el.querySelector<HTMLElement>('.veil')!.click();
    await settle();
    expect(tray()?.finished).toBe(1);
    expect(animator.current()).not.toBeNull();
    expect(el.querySelector('.result')).toBeTruthy();
    el.querySelector<HTMLElement>('.veil')!.click();
    await settle();
    expect(animator.current()).toBeNull();
    expect(el.querySelector('.scrim')).toBeNull();
    expect(tray()?.destroyed).toBe(true);
  });

  it('lands on Esc, and the second Esc closes', async () => {
    const { animator, settle, tray } = setup();
    animator.play(attack);
    await settle();
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    await settle();
    expect(tray()?.finished).toBe(1);
    expect(animator.current()).not.toBeNull();
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    expect(animator.current()).toBeNull();
  });

  it('lands at once a roll whose tap came before the tray was there', async () => {
    const { animator, el, fixture, settle } = setup();
    animator.play(attack);
    fixture.detectChanges();
    el.querySelector<HTMLElement>('.veil')!.click();
    await settle();
    expect(animator.done()).toBe(true);
  });

  it('stays open for the table to read and closes by itself at 10 s', async () => {
    const { animator, el, settle, step, tray } = setup();
    animator.play(attack);
    await settle();
    tray()?.land();
    step(9_900);
    expect(el.querySelector('.scrim')).toBeTruthy();
    step(100);
    expect(el.querySelector('.scrim')).toBeNull();
  });

  it('replaces a roll that is playing', async () => {
    const { animator, el, settle, tray } = setup();
    animator.play(attack);
    await settle();
    animator.play({ label: 'Teste de Sabedoria', dice: [{ sides: 20, face: 4 }] });
    await settle();
    expect(tray()?.rolls.map((r) => (r[0] as DieSpec).result)).toEqual([17, 4]);
    tray()?.land();
    await settle();
    expect(el.querySelector('.result')?.textContent).toContain('Teste de Sabedoria');
    expect(el.querySelectorAll('app-roll-tray').length).toBe(1);
  });

  it('plays the same show twice', async () => {
    const { animator, settle, tray } = setup();
    animator.play(attack);
    await settle();
    animator.play(attack);
    await settle();
    expect(tray()?.rolls.length).toBe(2);
  });

  it('draws the dice at rest and shows the words at once under reduced motion', async () => {
    const { animator, el, settle, tray } = setup(true);
    animator.play(attack);
    await settle();
    expect(animator.reduced()).toBe(true);
    expect(tray()?.reduced.at(-1)).toBe(true);
    expect(el.querySelector('.result')?.textContent).toContain('17 + 5 = 22');
    expect(el.querySelector('[role=status]')?.textContent).toContain(
      'Ataque com Machado grande: 17.',
    );
  });

  it('treats a browser that cannot say as reduced motion, and does nothing without a mounted overlay', async () => {
    vi.stubGlobal('matchMedia', undefined);
    TestBed.configureTestingModule({
      providers: [
        {
          provide: TRAY_LOADER,
          useValue: () => Promise.resolve({ createTray: createFakeTray as never }),
        },
      ],
    });
    const fixture = TestBed.createComponent(RollOverlay);
    fixture.detectChanges();
    const animator = TestBed.inject(RollAnimator);
    animator.play(attack);
    expect(animator.current()).not.toBeNull();
    expect(animator.reduced()).toBe(true);
    fixture.destroy();
    animator.play(attack);
    expect(animator.current()).toBeNull();
  });

  it('refuses a face the die does not have', () => {
    const { animator } = setup();
    animator.play({ label: 'x', dice: [{ sides: 6, face: 9 }] });
    expect(animator.current()).toBeNull();
  });

  it('shows at most eight dice and a +N chip for the rest', async () => {
    const { animator, el, settle, tray } = setup();
    animator.play({
      label: 'Bola de Fogo',
      dice: Array.from({ length: 10 }, () => ({ sides: 6 as const, face: 3 })),
    });
    await settle();
    expect(tray()?.rolls[0].length).toBe(MAX_DICE);
    expect(el.querySelector('.more')?.textContent).toContain('+2');
  });

  it('keeps a d100 as one die for the tray, which draws it as two d10', async () => {
    const { animator, settle, tray } = setup();
    animator.play({ label: 'Dado', dice: [{ sides: 100, face: 47 }] });
    await settle();
    expect(tray()?.rolls[0]).toEqual([
      { sides: 100, result: 47, discarded: false, critical: false, fumble: false },
    ]);
  });

  it('sizes the tray to the overlay: 0.55 of the width on a phone, at most 520 × 280', async () => {
    const { animator, settle, tray } = setup();
    animator.play(attack);
    await settle();
    const { width, height } = tray()!.size;
    expect(width).toBeLessThanOrEqual(520);
    expect(height).toBeLessThanOrEqual(280);
    expect(Math.abs(height - Math.round(width * 0.55)) <= 1 || height === 280).toBe(true);
  });
});

describe('the dice sent to the tray', () => {
  it('maps sides, result, a discarded d20 and the critical onto the d20 that counts', () => {
    const { dice, hidden } = trayDice({
      label: 'Ataque',
      dice: [
        { sides: 20, face: 20, counts: false },
        { sides: 20, face: 20, counts: true },
      ],
      critical: true,
    });
    expect(hidden).toBe(0);
    expect(dice).toEqual([
      { sides: 20, result: 20, discarded: true, critical: false, fumble: false },
      { sides: 20, result: 20, discarded: false, critical: true, fumble: false },
    ]);
  });

  it('puts the fumble on the d20 that counts, and neither on a damage die', () => {
    const fumble = trayDice({
      label: 'Ataque',
      dice: [
        { sides: 20, face: 1, counts: true },
        { sides: 20, face: 12, counts: false },
      ],
      fumble: true,
    });
    expect(fumble.dice.map((d) => d.fumble)).toEqual([true, false]);
    const damage = trayDice({
      label: 'Dano',
      dice: [{ sides: 6, face: 6 }],
      critical: true,
    });
    expect(damage.dice[0].critical).toBe(false);
  });

  it('keeps a d100 as one die and stops where the tray is full', () => {
    expect(trayDice({ label: 'x', dice: [{ sides: 100, face: 47 }] }).dice).toHaveLength(1);
    const many = trayDice({
      label: 'x',
      dice: Array.from({ length: 8 }, () => ({ sides: 100 as const, face: 5 })),
    });
    expect(many.dice.length * 2).toBeLessThanOrEqual(MAX_ON_TRAY);
    expect(many.hidden).toBe(8 - many.dice.length);
  });

  it('draws nothing for no show', () => {
    expect(trayDice(null)).toEqual({ dice: [], hidden: 0 });
  });
});

describe('Som dos dados', () => {
  beforeEach(() => vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] }));
  afterEach(() => vi.useRealTimers());

  const toggle = (el: HTMLElement) => el.querySelector<HTMLButtonElement>('button.action')!;

  it('is off by default: the tray is made silent and the button is not pressed', async () => {
    const { animator, el, settle, tray } = setup();
    animator.play(attack);
    await settle();
    expect(tray()?.options['sound']).toBe(false);
    expect(tray()?.sounds).toEqual([]);
    expect(toggle(el).getAttribute('aria-pressed')).toBe('false');
    expect(toggle(el).textContent).toContain('Som dos dados');
  });

  it('is a 44 px toggle that switches the sound on in the tap itself, without landing or closing the roll', async () => {
    const { animator, el, fixture, settle, tray } = setup();
    animator.play(attack);
    await settle();
    toggle(el).click();
    fixture.detectChanges();
    expect(tray()?.sounds).toEqual([true]);
    expect(toggle(el).getAttribute('aria-pressed')).toBe('true');
    expect(TestBed.inject(DiceSoundSetting).on()).toBe(true);
    expect(tray()?.finished).toBe(0);
    expect(animator.current()).not.toBeNull();
    toggle(el).click();
    fixture.detectChanges();
    expect(tray()?.sounds).toEqual([true, false]);
    expect(toggle(el).getAttribute('aria-pressed')).toBe('false');
  });

  it('is on for the next roll of the same tab', async () => {
    const { animator, el, settle } = setup();
    animator.play(attack);
    await settle();
    toggle(el).click();
    animator.dismiss();
    await settle();
    animator.play(attack);
    await settle();
    expect(made.at(-1)?.options['sound']).toBe(true);
  });

  it('writes nothing to web storage or a cookie', async () => {
    const setItem = vi.spyOn(Storage.prototype, 'setItem');
    const cookie = vi.spyOn(document, 'cookie', 'set');
    const { animator, el, settle } = setup();
    animator.play(attack);
    await settle();
    toggle(el).click();
    toggle(el).click();
    expect(setItem).not.toHaveBeenCalled();
    expect(cookie).not.toHaveBeenCalled();
    expect(localStorage.length).toBe(0);
    expect(sessionStorage.length).toBe(0);
  });

  it('starts silent again in a new tab (a new service instance)', () => {
    expect(TestBed.inject(DiceSoundSetting).on()).toBe(false);
  });
});

describe('traySize', () => {
  it('spans the width on a phone at 0.55 of it, and stops at 520 × 280 from a tablet up', () => {
    expect(traySize(358)).toEqual({ width: 358, height: 197 });
    expect(traySize(520)).toEqual({ width: 520, height: 280 });
    expect(traySize(900)).toEqual({ width: 520, height: 280 });
  });

  it('keeps the words and the buttons on screen when the window is low', () => {
    expect(traySize(520, 390)).toEqual({ width: 520, height: 195 });
    expect(traySize(0)).toEqual({ width: 320, height: 176 });
  });
});
