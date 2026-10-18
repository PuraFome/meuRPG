import { signal } from '@angular/core';
import type { MessageInitShape } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import type { PuzzleMoveSchema, PuzzleRun } from '../../../gen/meurpg/play/v1/puzzles_pb';
import { newKey } from '../connect/idempotency';
import type { HeldAnswer } from '../held-rolls/held-roll-offer';
import { puzzleBlocked, puzzleErrorMessage } from './puzzle-errors';
import { PuzzleHints } from './puzzle-hints';
import {
  type HintTry,
  type MoveVerdict,
  type PlayApi,
  type Schedule,
  notSent,
} from './puzzle-play-api';
import { RETRY_WAITS_MS, hintLineStale, movedOn, withRetry } from './puzzle-play-support';
import type { HintDie, MoveAnswer } from './puzzles-client';

// The API of the play lives in `puzzle-play-api`; the pages import it from here.
export type { HintTry, MoveVerdict, PlayApi, Schedule };
export { RETRY_WAITS_MS };

const timeoutSchedule: Schedule = (ms, fn) => {
  const id = setTimeout(fn, ms);
  return () => clearTimeout(id);
};

/** A little after the server's `next_in_ms`, so the read finds the step revealed, never the one before it. */
export const REVEAL_SLACK_MS = 60;

/**
 * The player's open puzzle (MR-038, RN-27): the run as the server last said it, and the moves.
 *
 * - **The board follows the server.** A move changes nothing on screen until the server answers; the answer (or a read
 *   after `puzzle_changed`) is applied only when its `revision` is larger than the one on screen, so a late answer never
 *   puts the board back.
 * - **Each move has its own idempotency key**, made when the player acts, and a failed try that never got an answer is
 *   sent again with the *same* key (up to three times): the server plays a key once, so a retry after a lost answer
 *   never touches the board twice. Moves are relative and commute, so two taps in a row are sent at once.
 * - **A refusal reads again.** A `failed_precondition` (solved, stopped, no attempts) or a `not_found` (the master closed
 *   it) says the screen was stale: the message is shown and the run is read again.
 *
 * Plain TypeScript with signals, so the timing is tested with a fake `wait`.
 */
export class PuzzlePlay {
  /** The run on screen, or `null` before the first read and after the master closed it. */
  readonly run = signal<PuzzleRun | null>(null);
  /** The master closed the puzzle, or it never was one the player may read. */
  readonly gone = signal(false);
  /** Something to say about the last move or read, in words, or `''`. */
  readonly message = signal('');
  /** Moves sent and not answered yet. */
  readonly pending = signal(0);
  /** The tries for a hint, apart from the moves; the signals below are theirs, kept here for the pages. */
  private readonly hints: PuzzleHints;
  /** A try for a hint is in flight. */
  readonly hintBusy: PuzzleHints['busy'];
  /** The last try for a hint, until the next one or the next change of the puzzle. */
  readonly hintTry: PuzzleHints['last'];
  /** The try's d20 was rolled and kept and the try waits for a question: the halfling's Lucky about a 1 first, then the
   * Bardic Inspiration die's. */
  readonly hintHeld: PuzzleHints['held']['offer'];
  /** The next try with a real die takes two d20: the server said so, or the last result had advantage or disadvantage. */
  readonly hintPair: PuzzleHints['pair'];
  /** The d4 an effect adds to a try with a real die (Orientação): asked when the server says the roll takes them. */
  readonly extraDice: PuzzleHints['extraDice'];
  /** The server refused a try because the table rolls its dice the other way: the page reads the campaign's dice mode again. */
  readonly diceModeStale: PuzzleHints['diceModeStale'];
  /** What to say when the first read of the run failed and nothing is on screen, or `''`; "Tentar de novo" reads again. */
  readonly loadError = signal('');

  constructor(
    private readonly api: PlayApi,
    private readonly campaignId: () => string,
    private readonly wait: (ms: number) => Promise<void> = (ms) =>
      new Promise((resolve) => setTimeout(resolve, ms)),
    private readonly makeKey: () => string = newKey,
    private readonly schedule: Schedule = timeoutSchedule,
  ) {
    this.hints = new PuzzleHints({
      api,
      campaignId,
      wait,
      makeKey,
      run: this.run,
      gone: this.gone,
      message: this.message,
      openSeq: () => this.openSeq,
      apply: (run) => this.apply(run),
      refresh: () => this.refresh(),
    });
    this.hintBusy = this.hints.busy;
    this.hintTry = this.hints.last;
    this.hintHeld = this.hints.held.offer;
    this.hintPair = this.hints.pair;
    this.extraDice = this.hints.extraDice;
    this.diceModeStale = this.hints.diceModeStale;
  }

  private cancelReveal: (() => void) | null = null;
  /** The page left: a read that is still on its way never starts a timer after it. */
  private disposed = false;
  /** Bell taps wait for the one before: the server may commit two moves in flight in either order, and a retry swaps them for sure. */
  private bellQueue: Promise<unknown> = Promise.resolve();
  /** Counts the wrong bells: taps made before the player knew a bell was wrong are dropped, not judged again. */
  private bellEpoch = 0;
  /** Counts the puzzles opened: work that began for an earlier one (a tap, a hint, a read) never touches the one on screen. */
  private openSeq = 0;

  /** Stops the timer that reads the sequence again (the page is leaving). */
  dispose(): void {
    this.disposed = true;
    this.stopReveal();
  }

  private stopReveal(): void {
    this.cancelReveal?.();
    this.cancelReveal = null;
  }

  /** Opens a puzzle: the first read. */
  async open(puzzleId: string): Promise<void> {
    this.disposed = false;
    this.stopReveal();
    this.openSeq++;
    this.bellQueue = Promise.resolve();
    this.bellEpoch++;
    this.pending.set(0);
    this.run.set(null);
    this.gone.set(false);
    this.loadError.set('');
    this.message.set('');
    this.hints.reset();
    this.pendingId = puzzleId;
    await this.refresh();
  }

  private pendingId = '';

  /** Reads the run again (after `puzzle_changed`, or when the page was away). */
  async refresh(): Promise<void> {
    const id = this.pendingId;
    if (id === '') {
      return;
    }
    try {
      const next = await this.api.run(this.campaignId(), id);
      if (id !== this.pendingId) {
        return;
      }
      this.apply(next);
      this.gone.set(false);
      this.loadError.set('');
    } catch (err) {
      if (id !== this.pendingId) {
        return;
      }
      if (ConnectError.from(err, Code.Unavailable).code === Code.NotFound) {
        this.gone.set(true);
        return;
      }
      // Any other failure keeps what is on screen: the next hint or "ready" reads again. With nothing on screen, the page says so.
      if (this.run() === null) {
        this.loadError.set(puzzleErrorMessage(err, 'abrir o quebra-cabeça', 'player'));
      }
    }
  }

  /** Applies a run if it is newer than the one on screen (the same puzzle) or the first of another. Returns whether it did. */
  apply(next: PuzzleRun): boolean {
    // A late answer for the puzzle that was open before is not this one's.
    if (this.pendingId !== '' && next.puzzleId !== this.pendingId) {
      return false;
    }
    const current = this.run();
    if (
      current &&
      current.puzzleId === next.puzzleId &&
      next.revision <= current.revision &&
      !movedOn(current, next)
    ) {
      return false;
    }
    // What a player tried for a hint is about the hint they were at: once it moves on (the master released one, the player may try again),
    // or the puzzle is over, the line goes.
    if (current && hintLineStale(current, next)) {
      this.hintTry.set(null);
    }
    this.run.set(next);
    this.watchReveal(next);
    return true;
  }

  /** While a play runs, read again when the server says the next step is shown (the stream's hint says it too; this is the safety net). */
  private watchReveal(run: PuzzleRun): void {
    this.stopReveal();
    const playback = run.sequence;
    if (!this.disposed && playback?.playing && playback.nextInMs > 0) {
      this.cancelReveal = this.schedule(
        playback.nextInMs + REVEAL_SLACK_MS,
        () => void this.refresh(),
      );
    }
  }

  /**
   * Makes one move. Never rejects: a failure becomes `message`. The verdict says whether a typed answer or a bell was judged wrong:
   * the server's answer carries the verdict of this very move (also on a replay), never read from the run's last move, which may be
   * another player's.
   *
   * Bell taps go one at a time, in the order they were made, each after the answer to the one before; `pending` counts the taps still
   * waiting too, so the screen can say so. A wrong bell drops the taps queued behind it (they were made for a sequence that started over).
   */
  move(move: MessageInitShape<typeof PuzzleMoveSchema>): Promise<MoveVerdict> {
    if (move.kind?.case !== 'sequence') {
      return this.send(move);
    }
    const epoch = this.bellEpoch;
    const seq = this.openSeq;
    this.pending.update((n) => n + 1);
    const turn = this.bellQueue.then(async () => {
      if (seq !== this.openSeq) {
        // Made for the puzzle that was open before: opening this one already cleared the count.
        return notSent();
      }
      if (epoch !== this.bellEpoch) {
        this.pending.update((n) => n - 1);
        return notSent();
      }
      const verdict = await this.send(move, true);
      if (verdict.wrong && seq === this.openSeq) {
        this.bellEpoch++;
      }
      return verdict;
    });
    this.bellQueue = turn;
    return turn;
  }

  private async send(
    move: MessageInitShape<typeof PuzzleMoveSchema>,
    counted = false,
  ): Promise<MoveVerdict> {
    const run = this.run();
    if (!run || run.solved || run.stopped) {
      if (counted) {
        this.pending.update((n) => n - 1);
      }
      return notSent();
    }
    // A typed answer spends an attempt: a second one made before the first is judged (Enter held, a double tap) is not sent.
    if (isTypedAnswer(move) && this.pending() > 0) {
      return notSent();
    }
    const seq = this.openSeq;
    const key = this.makeKey();
    if (!counted) {
      this.pending.update((n) => n + 1);
    }
    this.message.set('');
    try {
      const answer = await this.sendWithRetry(run.puzzleId, move, key);
      if (seq !== this.openSeq) {
        return notSent();
      }
      this.apply(answer.run);
      return {
        sent: true,
        wrong: answer.wrong && !answer.run.solved,
        solved: answer.solvedByThisMove || answer.run.solved,
      };
    } catch (err) {
      if (seq !== this.openSeq) {
        return notSent();
      }
      await this.moveFailed(err);
      return notSent();
    } finally {
      if (seq === this.openSeq) {
        this.pending.update((n) => n - 1);
      }
    }
  }

  /** Says why a move failed; a refusal or a closed puzzle says the screen was stale, so the run is read again. */
  private async moveFailed(err: unknown): Promise<void> {
    this.message.set(puzzleErrorMessage(err, 'fazer essa jogada', 'player'));
    const notFound = ConnectError.from(err, Code.Unavailable).code === Code.NotFound;
    if (puzzleBlocked(err) || notFound) {
      await this.refresh();
    }
  }

  /** "Tentar uma dica": see `PuzzleHints.try`. */
  tryHint(die: HintDie): Promise<void> {
    return this.hints.try(die);
  }

  /** The answer to the question the try waits on (the halfling's Lucky, the Bardic Inspiration die): see `PuzzleHints.answer`. */
  answerHint(answer: HeldAnswer): Promise<void> {
    return this.hints.answer(answer);
  }

  private sendWithRetry(
    puzzleId: string,
    move: MessageInitShape<typeof PuzzleMoveSchema>,
    key: string,
  ): Promise<MoveAnswer> {
    return withRetry(() => this.api.move(this.campaignId(), puzzleId, move, key), this.wait);
  }
}

/** A riddle answer or a cipher text: the moves the server judges (and may count as an attempt). */
function isTypedAnswer(move: MessageInitShape<typeof PuzzleMoveSchema>): boolean {
  const kind = move.kind?.case;
  return kind === 'riddle' || kind === 'cipher';
}
