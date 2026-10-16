import type { PuzzleRun } from '../../../gen/meurpg/play/v1/puzzles_pb';
import { isTransient } from './puzzle-errors';

/** Waits before the 1st, 2nd and 3rd retry of a move whose answer never came. */
const FIRST_RETRY_MS = 400;
const SECOND_RETRY_MS = 1200;
const THIRD_RETRY_MS = 3000;
export const RETRY_WAITS_MS: readonly number[] = [FIRST_RETRY_MS, SECOND_RETRY_MS, THIRD_RETRY_MS];

/**
 * Calls the server and, when the answer never came (`isTransient`), calls it again after each wait of `RETRY_WAITS_MS`. The
 * caller keeps the idempotency key the same across the tries, so the server plays it once.
 */
export async function withRetry<T>(
  call: () => Promise<T>,
  wait: (ms: number) => Promise<void>,
): Promise<T> {
  for (let attempt = 0; ; attempt++) {
    try {
      return await call();
    } catch (err) {
      const waitMs = RETRY_WAITS_MS[attempt];
      if (waitMs === undefined || !isTransient(err)) {
        throw err;
      }
      await wait(waitMs);
    }
  }
}

/** Whether the line about the last try for a hint is about a hint that has moved on: one was released, the player may try again, or the puzzle is over. */
export function hintLineStale(current: PuzzleRun, next: PuzzleRun): boolean {
  return (
    next.hints.length !== current.hints.length ||
    (next.canTryHint && !current.canTryHint) ||
    next.solved ||
    next.stopped
  );
}

/**
 * Whether a read of the same revision is further along than the one on screen. A play of the sequence reveals a step, a time limit
 * stops the puzzle, a clue reaches the notes: none changes the revision, so the page takes the newer of two reads of one revision by what only moves forward.
 */
export function movedOn(current: PuzzleRun, next: PuzzleRun): boolean {
  if (next.revision < current.revision) {
    return false;
  }
  if (next.stopped && !current.stopped) {
    return true;
  }
  // The player found the key of the cipher in the adventure: a clue that reached their notes, which moves no revision either.
  if (next.keyClueId !== '' && current.keyClueId === '') {
    return true;
  }
  const a = current.sequence;
  const b = next.sequence;
  if (!a || !b) {
    return false;
  }
  if (b.plays !== a.plays) {
    return b.plays > a.plays;
  }
  // The same play: it only goes forward, and once it ended (not playing, nothing shown) a late read from the middle of it never brings it back.
  if (!a.playing) {
    return false;
  }
  return !b.playing || b.shown.length > a.shown.length;
}
