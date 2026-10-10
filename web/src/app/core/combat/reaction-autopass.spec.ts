import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ReactionKind, ReactionWindowStatus } from '../../../gen/meurpg/play/v1/combat_pb';
import { combatant, encounter, reactionWindow } from './combat-testing';
import {
  AUTO_PASS_SECONDS,
  AutoPassClock,
  autoPassKey,
  autoPassText,
  autoPasses,
  playerAutoPassText,
  playerSecondsLeft,
} from './reaction-autopass';
import { windowsBarText } from './reaction-master';

const MS = 1000;

function win(id: string, kind: ReactionKind, over: object = {}) {
  return reactionWindow({
    id,
    kind,
    reactorId: 'pen',
    reactorLabel: 'Pensantus',
    reactorIsPlayer: true,
    answerNow: true,
    ...over,
  } as never);
}

describe("the automatic pass of a player's optional reaction", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-10-10T12:00:00Z'));
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it('passes an optional player window once, at 30 s, with the auto-pass key', () => {
    const clock = new AutoPassClock();
    const w = win('w1', ReactionKind.CUTTING_WORDS);
    clock.sync([w], Date.now());
    expect(clock.due(Date.now())).toEqual([]);
    vi.advanceTimersByTime((AUTO_PASS_SECONDS - 1) * MS);
    clock.sync([w], Date.now());
    expect(clock.due(Date.now())).toEqual([]);
    vi.advanceTimersByTime(MS);
    clock.sync([w], Date.now());
    expect(clock.due(Date.now())).toEqual(['w1']);
    // Once: a second look does not send it again.
    expect(clock.due(Date.now())).toEqual([]);
    expect(autoPassKey('w1')).toBe('auto-pass:w1');
  });

  it('may send a pass again after a failure that was not a refusal', () => {
    const clock = new AutoPassClock();
    const w = win('w1', ReactionKind.SHIELD);
    clock.sync([w], Date.now());
    vi.advanceTimersByTime(AUTO_PASS_SECONDS * MS);
    expect(clock.due(Date.now())).toEqual(['w1']);
    clock.release('w1');
    expect(clock.due(Date.now())).toEqual(['w1']);
  });

  it('never passes a saving throw, a contest, the master check or an NPC window', () => {
    const never = [
      win('c', ReactionKind.CONCENTRATION_SAVE),
      win('e', ReactionKind.EFFECT_SAVE),
      win('k', ReactionKind.CONTEST),
      win('m', ReactionKind.MASTER_CHECK),
      win('r', ReactionKind.HELLISH_REBUKE, { secondStep: true }),
      win('n', ReactionKind.SHIELD, { reactorIsPlayer: false }),
      win('q', ReactionKind.SHIELD, { answerNow: false }),
    ];
    for (const w of never) {
      expect(autoPasses(w)).toBe(false);
    }
    const clock = new AutoPassClock();
    clock.sync(never, Date.now());
    vi.advanceTimersByTime(AUTO_PASS_SECONDS * 2 * MS);
    expect(clock.due(Date.now())).toEqual([]);
  });

  it('does not pass a window answered before 30 s', () => {
    const clock = new AutoPassClock();
    const w = win('w1', ReactionKind.UNCANNY_DODGE);
    clock.sync([w], Date.now());
    vi.advanceTimersByTime(10 * MS);
    const answered = win('w1', ReactionKind.UNCANNY_DODGE, {
      status: ReactionWindowStatus.ANSWERED,
    });
    clock.sync([answered], Date.now());
    vi.advanceTimersByTime(AUTO_PASS_SECONDS * MS);
    expect(clock.due(Date.now())).toEqual([]);
    expect(clock.soonest(Date.now())).toBeNull();
  });

  it('starts the 30 s when the window becomes the one to answer, not before', () => {
    const clock = new AutoPassClock();
    clock.sync([win('w1', ReactionKind.COUNTERSPELL, { answerNow: false })], Date.now());
    vi.advanceTimersByTime(20 * MS);
    clock.sync([win('w1', ReactionKind.COUNTERSPELL)], Date.now());
    expect(clock.secondsLeft('w1', Date.now())).toBe(AUTO_PASS_SECONDS);
  });

  it('counts the seconds down, and writes them for the master and for the player', () => {
    const clock = new AutoPassClock();
    clock.sync([win('w1', ReactionKind.CUTTING_WORDS)], Date.now());
    vi.advanceTimersByTime(6 * MS);
    expect(clock.secondsLeft('w1', Date.now())).toBe(24);
    expect(autoPassText(24)).toBe('Passa sozinho em 24 s');
    expect(playerAutoPassText(24)).toBe('Se você não responder, passa sozinho em 24 s.');
    expect(playerSecondsLeft(Date.now() - 6 * MS, Date.now())).toBe(24);
    expect(playerSecondsLeft(Date.now() - 99 * MS, Date.now())).toBe(0);
  });

  it("goes with the master's bar text", () => {
    const e = encounter({
      combatants: [combatant({ id: 'pen', label: 'Pensantus' })],
      reactionWindows: [win('w1', ReactionKind.CUTTING_WORDS)],
    } as never);
    expect(windowsBarText(e)).toBe('Esperando a reação de Pensantus');
  });
});
