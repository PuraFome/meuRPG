import { signal } from '@angular/core';
import type { MessageInitShape } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import type { DiceRoll } from '../../../gen/meurpg/play/v1/combat_pb';
import type { PuzzleMoveSchema, PuzzleRun, TryPuzzleHintResponse } from '../../../gen/meurpg/play/v1/puzzles_pb';
import { timestampDate } from '@bufbuild/protobuf/wkt';
import { newKey } from '../connect/idempotency';
import { isTransient, puzzleBlocked, puzzleErrorMessage } from './puzzle-errors';
import type { HintDie, MoveAnswer } from './puzzles-client';

/** What the player's page needs from the client; `PuzzlesClient` is one. */
export interface PlayApi {
  run(campaignId: string, puzzleId: string): Promise<PuzzleRun>;
  move(campaignId: string, puzzleId: string, move: MessageInitShape<typeof PuzzleMoveSchema>, idempotencyKey: string): Promise<MoveAnswer>;
  tryHint(campaignId: string, puzzleId: string, die: HintDie, idempotencyKey: string): Promise<TryPuzzleHintResponse>;
}

/** What a move came to, for the page that sent it: a typed answer or a deciphered message is judged, not applied. */
export interface MoveVerdict {
  /** The move reached the server and was judged. */
  readonly sent: boolean;
  /** The judged move was wrong (the player's own: a wrong answer, a wrong bell). */
  readonly wrong: boolean;
  readonly solved: boolean;
}

/** The player's own try for a hint by a skill check, as the page says it ("Você conseguiu." / "Não deu desta vez."). */
export interface HintTry {
  readonly passed: boolean;
  /** The player's own roll: the d20 and the total with the bonus. Never the DC. */
  readonly roll: DiceRoll | undefined;
}

/** Calls `fn` after `ms` and returns what cancels it: the sequence's reveal reads the run again at each step. */
export type Schedule = (ms: number, fn: () => void) => () => void;

const timeoutSchedule: Schedule = (ms, fn) => {
  const id = setTimeout(fn, ms);
  return () => clearTimeout(id);
};

/** A little after the server's `next_in_ms`, so the read finds the step revealed, never the one before it. */
export const REVEAL_SLACK_MS = 60;

/** Waits before the 1st, 2nd and 3rd retry of a move whose answer never came. */
export const RETRY_WAITS_MS: readonly number[] = [400, 1200, 3000];

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
  /** A try for a hint is in flight. */
  readonly hintBusy = signal(false);
  /** The last try for a hint, until the next one or the next change of the puzzle. */
  readonly hintTry = signal<HintTry | null>(null);

  constructor(
    private readonly api: PlayApi,
    private readonly campaignId: () => string,
    private readonly wait: (ms: number) => Promise<void> = (ms) => new Promise((resolve) => setTimeout(resolve, ms)),
    private readonly makeKey: () => string = newKey,
    /** The player's own character, to tell their wrong answer from another player's. */
    private readonly ownName: () => string = () => '',
    private readonly schedule: Schedule = timeoutSchedule,
  ) {}

  private cancelReveal: (() => void) | null = null;

  /** Stops the timer that reads the sequence again (the page is leaving). */
  dispose(): void {
    this.cancelReveal?.();
    this.cancelReveal = null;
  }

  /** Opens a puzzle: the first read. */
  async open(puzzleId: string): Promise<void> {
    this.dispose();
    this.run.set(null);
    this.gone.set(false);
    this.message.set('');
    this.hintTry.set(null);
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
      this.apply(await this.api.run(this.campaignId(), id));
      this.gone.set(false);
    } catch (err) {
      if (ConnectError.from(err, Code.Unavailable).code === Code.NotFound) {
        this.gone.set(true);
        return;
      }
      // Any other failure keeps what is on screen: the next hint or "ready" reads again.
    }
  }

  /** Applies a run if it is newer than the one on screen (the same puzzle) or the first of another. Returns whether it did. */
  apply(next: PuzzleRun): boolean {
    // A late answer for the puzzle that was open before is not this one's.
    if (this.pendingId !== '' && next.puzzleId !== this.pendingId) {
      return false;
    }
    const current = this.run();
    if (current && current.puzzleId === next.puzzleId && next.revision <= current.revision && !movedOn(current, next)) {
      return false;
    }
    this.run.set(next);
    this.watchReveal(next);
    return true;
  }

  /** While a play runs, read again when the server says the next step is shown (the stream's hint says it too; this is the safety net). */
  private watchReveal(run: PuzzleRun): void {
    this.cancelReveal?.();
    this.cancelReveal = null;
    const playback = run.sequence;
    if (playback?.playing && playback.nextInMs > 0) {
      this.cancelReveal = this.schedule(playback.nextInMs + REVEAL_SLACK_MS, () => void this.refresh());
    }
  }

  /**
   * Makes one move. Never rejects: a failure becomes `message`. The verdict says whether a typed answer or a bell was judged wrong:
   * the server's answer has the run, and a wrong move of the player's own character is its `last_move` (never the answer itself).
   */
  async move(move: MessageInitShape<typeof PuzzleMoveSchema>): Promise<MoveVerdict> {
    const run = this.run();
    if (!run || run.solved || run.stopped) {
      return { sent: false, wrong: false, solved: false };
    }
    const key = this.makeKey();
    this.pending.update((n) => n + 1);
    this.message.set('');
    try {
      const before = run.lastMove?.at ? timestampDate(run.lastMove.at).getTime() : 0;
      const answer = await this.sendWithRetry(run.puzzleId, move, key);
      this.apply(answer.run);
      const last = answer.run.lastMove;
      const at = last?.at ? timestampDate(last.at).getTime() : 0;
      const own = this.ownName();
      const mine = !!last && last.wrong && at !== before && (own === '' || last.characterName === own);
      return { sent: true, wrong: !answer.run.solved && mine, solved: answer.solvedByThisMove || answer.run.solved };
    } catch (err) {
      this.message.set(puzzleErrorMessage(err, 'fazer essa jogada', 'player'));
      const blocked = puzzleBlocked(err);
      const notFound = ConnectError.from(err, Code.Unavailable).code === Code.NotFound;
      if (blocked || notFound) {
        await this.refresh();
      }
      return { sent: false, wrong: false, solved: false };
    } finally {
      this.pending.update((n) => n - 1);
    }
  }

  /**
   * "Tentar uma dica": rolls the puzzle's skill check for the next hint, in the app or from the face of a real die. A pass puts the
   * hint in `run.hints` (this player's alone); the answer is the run as this player reads it, and their own roll. The DC never
   * reaches here. The same key goes with a retry, so a lost answer never rolls twice.
   */
  async tryHint(die: HintDie): Promise<void> {
    const run = this.run();
    if (!run || !run.canTryHint || this.hintBusy()) {
      return;
    }
    const key = this.makeKey();
    this.hintBusy.set(true);
    this.message.set('');
    try {
      const answer = await this.sendHintWithRetry(run.puzzleId, die, key);
      this.apply(answer.run ?? run);
      this.hintTry.set({ passed: answer.passed, roll: answer.roll });
    } catch (err) {
      this.message.set(puzzleErrorMessage(err, 'tentar a dica', 'player'));
      if (puzzleBlocked(err) || ConnectError.from(err, Code.Unavailable).code === Code.NotFound) {
        await this.refresh();
      }
    } finally {
      this.hintBusy.set(false);
    }
  }

  private async sendHintWithRetry(puzzleId: string, die: HintDie, key: string): Promise<TryPuzzleHintResponse> {
    for (let attempt = 0; ; attempt++) {
      try {
        return await this.api.tryHint(this.campaignId(), puzzleId, die, key);
      } catch (err) {
        const waitMs = RETRY_WAITS_MS[attempt];
        if (waitMs === undefined || !isTransient(err)) {
          throw err;
        }
        await this.wait(waitMs);
      }
    }
  }

  private async sendWithRetry(puzzleId: string, move: MessageInitShape<typeof PuzzleMoveSchema>, key: string): Promise<MoveAnswer> {
    for (let attempt = 0; ; attempt++) {
      try {
        return await this.api.move(this.campaignId(), puzzleId, move, key);
      } catch (err) {
        const waitMs = RETRY_WAITS_MS[attempt];
        if (waitMs === undefined || !isTransient(err)) {
          throw err;
        }
        await this.wait(waitMs);
      }
    }
  }
}

/**
 * Whether a read of the same revision is further along than the one on screen. A play of the sequence reveals a step, a time limit
 * stops the puzzle, a clue reaches the notes: none changes the revision, so the page takes the newer of two reads of one revision by what only moves forward.
 */
function movedOn(current: PuzzleRun, next: PuzzleRun): boolean {
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
  return b.shown.length > a.shown.length || (a.playing && !b.playing);
}
