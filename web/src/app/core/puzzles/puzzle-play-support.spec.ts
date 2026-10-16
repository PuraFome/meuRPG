import { Code, ConnectError } from '@connectrpc/connect';

import { hintLineStale, movedOn, RETRY_WAITS_MS, withRetry } from './puzzle-play-support';
import { playerRun, riddlePuzzle, sequencePuzzle } from './puzzles-testing';

describe('withRetry', () => {
  it('returns the first answer without waiting', async () => {
    const waits: number[] = [];
    expect(
      await withRetry(
        () => Promise.resolve(7),
        async (ms) => void waits.push(ms),
      ),
    ).toBe(7);
    expect(waits).toEqual([]);
  });

  it('waits the listed time before each new try of a call that never got an answer', async () => {
    const waits: number[] = [];
    let calls = 0;
    const result = await withRetry(
      () =>
        ++calls < 3
          ? Promise.reject(new ConnectError('x', Code.Unavailable))
          : Promise.resolve('ok'),
      async (ms) => void waits.push(ms),
    );
    expect(result).toBe('ok');
    expect(waits).toEqual([RETRY_WAITS_MS[0], RETRY_WAITS_MS[1]]);
  });

  it('gives up after the last wait, and never tries a refusal again', async () => {
    let calls = 0;
    await expect(
      withRetry(
        () => {
          calls++;
          return Promise.reject(new ConnectError('x', Code.Unavailable));
        },
        async () => undefined,
      ),
    ).rejects.toThrow();
    expect(calls).toBe(1 + RETRY_WAITS_MS.length);
    let refused = 0;
    await expect(
      withRetry(
        () => {
          refused++;
          return Promise.reject(new ConnectError('x', Code.PermissionDenied));
        },
        async () => undefined,
      ),
    ).rejects.toThrow();
    expect(refused).toBe(1);
  });
});

describe('movedOn and hintLineStale', () => {
  const run = (partial: Record<string, unknown> = {}) => playerRun(riddlePuzzle('p', 'x'), partial);
  const seq = (sequence: Record<string, unknown>, revision = 1) =>
    playerRun(sequencePuzzle('p', 'x'), { revision, sequence });

  it('takes a read that stopped, or found the key of the cipher, at the same revision', () => {
    expect(movedOn(run(), run({ stopped: true }))).toBe(true);
    expect(movedOn(run({ stopped: true }), run({ stopped: true }))).toBe(false);
    expect(movedOn(run(), run({ keyClueId: 'c' }))).toBe(true);
    expect(movedOn(run({ keyClueId: 'c' }), run({ keyClueId: 'c' }))).toBe(false);
    expect(movedOn(run({ revision: 3 }), run({ revision: 2, stopped: true }))).toBe(false);
  });

  it('takes a play only when it goes forward', () => {
    const base = { totalSteps: 6, plays: 1, playing: true, shown: [0], stepMs: 1, nextInMs: 1 };
    expect(movedOn(seq(base), seq({ ...base, plays: 2 }))).toBe(true);
    expect(movedOn(seq({ ...base, plays: 2 }), seq({ ...base, plays: 1 }))).toBe(false);
    expect(movedOn(seq(base), seq({ ...base, shown: [0, 1] }))).toBe(true);
    expect(movedOn(seq(base), seq({ ...base, playing: false }))).toBe(true);
    expect(movedOn(seq(base), seq(base))).toBe(false);
    expect(movedOn(seq({ ...base, playing: false }), seq({ ...base, shown: [0, 1] }))).toBe(false);
    expect(movedOn(run(), seq(base))).toBe(false);
  });

  it('says the line about a try is stale when a hint came, the player may try again, or it is over', () => {
    const base = run({ canTryHint: false, hints: [] });
    expect(hintLineStale(base, run({ canTryHint: false, hints: [] }))).toBe(false);
    expect(hintLineStale(base, run({ canTryHint: false, hints: ['dica'] }))).toBe(true);
    expect(hintLineStale(base, run({ canTryHint: true, hints: [] }))).toBe(true);
    expect(hintLineStale(base, run({ solved: true }))).toBe(true);
    expect(hintLineStale(base, run({ stopped: true }))).toBe(true);
  });
});
