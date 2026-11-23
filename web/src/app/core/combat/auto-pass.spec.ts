import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { Code, ConnectError } from '@connectrpc/connect';
import { timestampFromMs } from '@bufbuild/protobuf/wkt';
import { vi } from 'vitest';

import {
  CombatantKind,
  type Encounter,
  ReactionKind,
  ReadiedKind,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { MasterAutoPass, autoPassOffers, masterWaitNote } from './auto-pass';
import { CombatState } from './combat-state';
import { combatant, encounter, reactionWindow } from './combat-testing';
import { AUTO_PASS_SECONDS, autoPassKey } from './reaction-autopass';
import { beginReleaseWait } from './release-mark';
import { masterRowWait } from './readied-wait';
import { fakeCombatClient } from './combat-client-testing';

const toren = combatant({ id: 't', label: 'Toren', kind: CombatantKind.PLAYER, mine: true });
const goblin = combatant({ id: 'g', label: 'Goblin' });
const shield = (id = 'w1') =>
  reactionWindow({
    id,
    kind: ReactionKind.SHIELD,
    reactorId: 't',
    reactorIsPlayer: true,
    answerNow: true,
  });
const offer = (id: string, reactorId: string) =>
  ({ id, reactorId, moverId: 'g', forYou: true }) as never;
const withWindows = (over: Partial<Encounter> = {}) =>
  encounter({ revision: 2, combatants: [toren, goblin], reactionWindows: [shield()], ...over });

function setup(opts: { master?: boolean; enc?: Encounter } = {}) {
  vi.useFakeTimers({ now: new Date('2026-10-16T12:00:00Z') });
  const state = new CombatState();
  state.apply(opts.enc ?? withWindows());
  const master = signal(opts.master ?? true);
  const api = {
    answerReaction: vi.fn(async () => ({ encounter: encounter({ revision: 9, round: 7 }) })),
    skipOpportunity: vi.fn(async () => encounter({ revision: 9, round: 8 })),
  };
  const pass = TestBed.runInInjectionContext(
    () =>
      new MasterAutoPass({
        api: fakeCombatClient(api),
        campaignId: () => 'c',
        encounter: () => state.shown(),
        isMaster: () => master(),
        state: () => state,
      }),
  );
  TestBed.tick();
  return { pass, state, api, master };
}

describe('MasterAutoPass', () => {
  afterEach(() => vi.useRealTimers());

  it('counts down the optional reaction of a player and passes it with its own key when the time is up', async () => {
    const { pass, state, api } = setup();
    expect(pass.windowsLeft()).toEqual({ w1: AUTO_PASS_SECONDS });
    await vi.advanceTimersByTimeAsync(10_000);
    expect(pass.windowsLeft()).toEqual({ w1: AUTO_PASS_SECONDS - 10 });
    expect(api.answerReaction).not.toHaveBeenCalled();

    await vi.advanceTimersByTimeAsync(20_000);
    expect(api.answerReaction).toHaveBeenCalledTimes(1);
    expect(api.answerReaction).toHaveBeenCalledWith(
      'c',
      'enc',
      'w1',
      { use: false },
      autoPassKey('w1'),
    );
    expect(state.shown()?.round).toBe(7);
  });

  it('runs no clock on a player’s screen', async () => {
    const { pass, api } = setup({ master: false });
    expect(pass.windowsLeft()).toEqual({});
    await vi.advanceTimersByTimeAsync(40_000);
    expect(api.answerReaction).not.toHaveBeenCalled();
    expect(vi.getTimerCount()).toBe(0);
  });

  it('stops the timer when nothing waits any more', async () => {
    const { state } = setup();
    expect(vi.getTimerCount()).toBe(1);
    state.apply(withWindows({ revision: 3, reactionWindows: [] }));
    TestBed.tick();
    expect(vi.getTimerCount()).toBe(0);
  });

  it('counts down the opportunity attacks of a player’s character, never the master’s own', async () => {
    const enc = withWindows({
      reactionWindows: [],
      opportunityOffers: [offer('o1', 't'), offer('o2', 'g')],
    });
    expect(autoPassOffers(enc).map((o) => o.id)).toEqual(['o1']);
    const { pass, api } = setup({ enc });
    expect(pass.offersLeft()).toEqual({ o1: AUTO_PASS_SECONDS });
    await vi.advanceTimersByTimeAsync(30_000);
    expect(api.skipOpportunity).toHaveBeenCalledWith('c', 'enc', 'o1', autoPassKey('o1'));
    expect(api.skipOpportunity).toHaveBeenCalledTimes(1);
  });

  it('sends the pass again after a failure that is the network’s, never after a refusal', async () => {
    const { api } = setup();
    api.answerReaction.mockRejectedValueOnce(new ConnectError('lost', Code.Unavailable));
    await vi.advanceTimersByTimeAsync(31_000);
    expect(api.answerReaction).toHaveBeenCalledTimes(2);

    const refused = setup();
    refused.api.answerReaction.mockRejectedValue(
      new ConnectError('answered', Code.FailedPrecondition),
    );
    await vi.advanceTimersByTimeAsync(40_000);
    expect(refused.api.answerReaction).toHaveBeenCalledTimes(1);
  });

  it('sends the pass again after a failure that is not a Connect error', async () => {
    const { api } = setup();
    api.answerReaction.mockRejectedValueOnce(new TypeError('network'));
    await vi.advanceTimersByTimeAsync(31_000);
    expect(api.answerReaction).toHaveBeenCalledTimes(2);
  });

  it('counts the readied 120 s on a monotonic clock: a wall-clock jump of +1 h mid-wait does not pass the window', async () => {
    const t0 = Date.parse('2026-10-16T12:00:00Z');
    const held = reactionWindow({
      id: 'r1',
      kind: ReactionKind.READIED,
      reactorId: 't',
      reactorIsPlayer: true,
      answerNow: true,
      rolling: true,
      waitBeganAt: timestampFromMs(t0),
      waitedMs: 10_000n,
    } as never);
    const { pass, api } = setup({ enc: withWindows({ reactionWindows: [held] }) });
    expect(pass.windowsLeft()).toEqual({ r1: 110 });
    await vi.advanceTimersByTimeAsync(5000);
    vi.setSystemTime(t0 + 3_600_000);
    await vi.advanceTimersByTimeAsync(5000);
    expect(pass.windowsLeft()).toEqual({ r1: 100 });
    expect(api.answerReaction).not.toHaveBeenCalled();
  });

  describe("the master's release beginning a readied window's wait (#857)", () => {
    const T0 = Date.parse('2026-10-16T12:00:00Z');
    const readied = (over: object = {}) =>
      reactionWindow({
        id: 'r1',
        kind: ReactionKind.READIED,
        reactorId: 't',
        reactorLabel: 'Toren',
        reactorIsPlayer: true,
        answerNow: true,
        ...over,
      } as never);
    const began = (ms: number) => ({
      waitBeganAt: timestampFromMs(T0 + 29_000 - ms),
      waitedMs: BigInt(ms),
    });
    const ids = { campaignId: 'c', encounterId: 'enc', windowId: 'r1' };

    it('passes nothing at 30 s while his call is on its way, then follows the wait the server began: 120 s, not 30 s', async () => {
      const { pass, api, state } = setup({ enc: withWindows({ reactionWindows: [readied()] }) });
      await vi.advanceTimersByTimeAsync(29_000);
      let answer!: (e: Encounter) => void;
      const call = beginReleaseWait(
        {
          markReactionRolling: () => new Promise<Encounter>((resolve) => (answer = resolve)),
        } as never,
        ids,
        (e) => state.apply(e),
      );
      await vi.advanceTimersByTimeAsync(3000);
      expect(api.answerReaction).not.toHaveBeenCalled();
      answer(
        withWindows({
          revision: 9,
          reactionWindows: [readied({ rolling: false, ...began(29_000) })],
        }),
      );
      await call;
      TestBed.tick();
      await vi.advanceTimersByTimeAsync(5000);
      expect(api.answerReaction).not.toHaveBeenCalled();
      expect(pass.windowsLeft()['r1']).toBeGreaterThan(80);
      await vi.advanceTimersByTimeAsync(90_000);
      expect(api.answerReaction).toHaveBeenCalledOnce();
    });

    it('decides from the window the server sent when the tick runs between the answer’s apply and the effect', async () => {
      const { pass, api, state } = setup({ enc: withWindows({ reactionWindows: [readied()] }) });
      await vi.advanceTimersByTimeAsync(29_500);
      state.apply(
        withWindows({
          revision: 9,
          reactionWindows: [readied({ rolling: false, ...began(29_500) })],
        }),
      );
      // The tick runs right after the apply, before the effect that follows the combat: the clock still holds the old window.
      vi.setSystemTime(Date.now() + 1000);
      (pass as unknown as { tick(): void }).tick();
      expect(api.answerReaction).not.toHaveBeenCalled();
      TestBed.tick();
      await vi.advanceTimersByTimeAsync(5000);
      expect(api.answerReaction).not.toHaveBeenCalled();
    });

    it('passes a window the server sent without a wait (the call failed) at once when the guard ends', async () => {
      const { api, state } = setup({ enc: withWindows({ reactionWindows: [readied()] }) });
      await vi.advanceTimersByTimeAsync(29_000);
      let refuse!: (e: Error) => void;
      const call = beginReleaseWait(
        { markReactionRolling: () => new Promise((_, reject) => (refuse = reject)) } as never,
        ids,
        (e) => state.apply(e),
      );
      await vi.advanceTimersByTimeAsync(3000);
      expect(api.answerReaction).not.toHaveBeenCalled();
      refuse(new Error('gone'));
      await call;
      await vi.advanceTimersByTimeAsync(1000);
      expect(api.answerReaction).toHaveBeenCalledOnce();
    });

    it('a second master screen reading the wait the first began passes at 120 s, not 30 s, and its row is "Esperando", never "No mapa"', async () => {
      const w = readied({ rolling: false, waitBeganAt: timestampFromMs(T0), waitedMs: 0n });
      const { pass, api } = setup({ enc: withWindows({ reactionWindows: [w] }) });
      await vi.advanceTimersByTimeAsync(31_000);
      expect(api.answerReaction).not.toHaveBeenCalled();
      expect(masterRowWait('Toren', false, pass.windowsLeft()['r1']).text).toBe(
        'Esperando Toren · passa sozinho em 1 min 29 s',
      );
      await vi.advanceTimersByTimeAsync(90_000);
      expect(api.answerReaction).toHaveBeenCalledOnce();
    });
  });

  it('stops the timer with the screen', () => {
    setup();
    expect(vi.getTimerCount()).toBe(1);
    TestBed.resetTestingModule();
    expect(vi.getTimerCount()).toBe(0);
  });
});

describe('masterWaitNote', () => {
  const name = (id: string) => (id === 'char-t' ? 'Vinicius' : '');

  it('says what the master waits for on a reaction of a player, with the time left of its automatic pass', () => {
    const e = withWindows();
    const plain = masterWaitNote(e, {}, name);
    expect(plain).not.toBe('');
    expect(plain).not.toContain('Passa sozinho');
    expect(masterWaitNote(e, { w1: 12 }, name)).toBe(`${plain} \u00b7 passa sozinho em 12 s`);
  });

  it('says nothing when nothing waits', () => {
    expect(masterWaitNote(encounter({ combatants: [toren, goblin] }), {}, name)).toBe('');
  });

  it('names the player an opportunity attack waits for, before any reaction', () => {
    const e = withWindows({ opportunityOffers: [offer('o1', 't')] });
    expect(masterWaitNote(e, { w1: 5 }, name)).toContain('Vinicius');
  });
});

describe("masterWaitNote of a readied window's wait (#857, 857-08 card 7)", () => {
  const name = () => '';
  const readied = (over: object = {}) =>
    reactionWindow({
      id: 'r1',
      kind: ReactionKind.READIED,
      reactorId: 'p',
      reactorLabel: 'Pensantus',
      reactorIsPlayer: true,
      answerNow: true,
      ...over,
    } as never);
  const pen = combatant({ id: 'p', label: 'Pensantus', kind: CombatantKind.PLAYER });

  it('says "Esperando a reação de Pensantus" and the time left before the player is on the map', () => {
    const e = encounter({ combatants: [pen, goblin], reactionWindows: [readied()] });
    expect(masterWaitNote(e, { r1: 24 }, name)).toBe(
      'Esperando a reação de Pensantus · passa sozinho em 24 s',
    );
  });

  const holding = (kind: ReadiedKind) =>
    combatant({
      id: 'p',
      label: 'Pensantus',
      kind: CombatantKind.PLAYER,
      readied: { id: 'h1', kind } as never,
    });

  it('says the player is on the map, with the 2 minutes counted as minutes, for a held move', () => {
    const e = encounter({
      combatants: [holding(ReadiedKind.MOVE), goblin],
      reactionWindows: [readied({ rolling: true })],
    });
    expect(masterWaitNote(e, { r1: 108 }, name)).toBe(
      'Pensantus está no mapa · passa sozinho em 1 min 48 s',
    );
  });

  it('does not say "no mapa" for the mark of a player who only chooses targets in the window', () => {
    const e = encounter({
      combatants: [holding(ReadiedKind.ATTACK), goblin],
      reactionWindows: [readied({ rolling: true })],
    });
    expect(masterWaitNote(e, { r1: 108 }, name)).toBe(
      'Esperando a reação de Pensantus · passa sozinho em 1 min 48 s',
    );
  });

  it('says it for a held spell the caller knows is placed on the map', () => {
    const e = encounter({
      combatants: [holding(ReadiedKind.SPELL), goblin],
      reactionWindows: [readied({ rolling: true })],
    });
    expect(masterWaitNote(e, { r1: 108 }, name, () => true)).toBe(
      'Pensantus está no mapa · passa sozinho em 1 min 48 s',
    );
  });

  it('says it again as the same wait once the player is back from the map', () => {
    const e = encounter({
      combatants: [pen, goblin],
      reactionWindows: [readied({ rolling: false })],
    });
    expect(masterWaitNote(e, { r1: 72 }, name)).toBe(
      'Esperando a reação de Pensantus · passa sozinho em 1 min 12 s',
    );
  });
});
