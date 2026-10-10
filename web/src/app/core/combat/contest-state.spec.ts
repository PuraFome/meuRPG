import { ContestStatus, HideAttemptStatus } from '../../../gen/meurpg/play/v1/contest_types_pb';
import type { ContestClient } from './contest-client';
import { ContestState } from './contest-state';
import { contestState, contestView, hideAttempt } from './contest-testing';

function api(read: () => Promise<ReturnType<typeof contestState>>): ContestClient {
  return { state: read } as unknown as ContestClient;
}

describe('ContestState', () => {
  it('reads the contests of the combat and finds one by id', async () => {
    const state = new ContestState();
    await state.load(
      api(() => Promise.resolve(contestState({ contests: [contestView({ id: 'a' })] }))),
      'c',
      'e',
    );
    expect(state.contests().map((c) => c.id)).toEqual(['a']);
    expect(state.contest('a')?.status).toBe(ContestStatus.AWAITING_DEFENDER);
    expect(state.contest('zzz')).toBeUndefined();
  });

  it('drops an answer that arrives after a newer question', async () => {
    const state = new ContestState();
    let release: (v: ReturnType<typeof contestState>) => void = () => undefined;
    const slow = new Promise<ReturnType<typeof contestState>>((r) => (release = r));
    const first = state.load(
      api(() => slow),
      'c',
      'e',
    );
    await state.load(
      api(() => Promise.resolve(contestState({ contests: [contestView({ id: 'new' })] }))),
      'c',
      'e',
    );
    release(contestState({ contests: [contestView({ id: 'old' })] }));
    await first;
    expect(state.contests().map((c) => c.id)).toEqual(['new']);
  });

  it('never shows the contests of another combat while the answer comes', async () => {
    const state = new ContestState();
    await state.load(
      api(() => Promise.resolve(contestState({ contests: [contestView()] }))),
      'c',
      'e1',
    );
    const slow = new Promise<ReturnType<typeof contestState>>(() => undefined);
    void state.load(
      api(() => slow),
      'c',
      'e2',
    );
    expect(state.contests()).toEqual([]);
  });

  it('keeps what it had when a read fails', async () => {
    const state = new ContestState();
    await state.load(
      api(() => Promise.resolve(contestState({ contests: [contestView()] }))),
      'c',
      'e',
    );
    await state.load(
      api(() => Promise.reject(new Error('lost'))),
      'c',
      'e',
    );
    expect(state.contests()).toHaveLength(1);
  });

  it('puts the contest a call answered in place of the old one, before any read', () => {
    const state = new ContestState();
    state.applyContest(contestView({ id: 'a', status: ContestStatus.AWAITING_DEFENDER }));
    state.applyContest(contestView({ id: 'a', status: ContestStatus.RESOLVED }));
    state.applyContest(contestView({ id: 'b' }));
    expect(state.contests().map((c) => `${c.id}:${c.status}`)).toEqual([
      `b:${ContestStatus.AWAITING_DEFENDER}`,
      `a:${ContestStatus.RESOLVED}`,
    ]);
  });

  it('does the same for a Hide attempt', () => {
    const state = new ContestState();
    state.applyAttempt(hideAttempt({ status: HideAttemptStatus.PENDING }));
    state.applyAttempt(hideAttempt({ status: HideAttemptStatus.APPLIED }));
    expect(state.attempts()).toHaveLength(1);
    expect(state.attempt('hd1')?.status).toBe(HideAttemptStatus.APPLIED);
  });

  it('is empty after it is cleared', () => {
    const state = new ContestState();
    state.applyContest(contestView());
    state.clear();
    expect(state.contests()).toEqual([]);
  });
});
