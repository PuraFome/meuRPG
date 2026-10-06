import { signal } from '@angular/core';
import type { MessageInitShape } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import type { PuzzleMoveSchema, PuzzleRun } from '../../../gen/meurpg/play/v1/puzzles_pb';
import { newKey } from '../connect/idempotency';
import { isTransient, puzzleBlocked, puzzleErrorMessage } from './puzzle-errors';
import type { MoveAnswer } from './puzzles-client';

/** What the player's page needs from the client; `PuzzlesClient` is one. */
export interface PlayApi {
  run(campaignId: string, puzzleId: string): Promise<PuzzleRun>;
  move(campaignId: string, puzzleId: string, move: MessageInitShape<typeof PuzzleMoveSchema>, idempotencyKey: string): Promise<MoveAnswer>;
}

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

  constructor(
    private readonly api: PlayApi,
    private readonly campaignId: () => string,
    private readonly wait: (ms: number) => Promise<void> = (ms) => new Promise((resolve) => setTimeout(resolve, ms)),
    private readonly makeKey: () => string = newKey,
  ) {}

  /** Opens a puzzle: the first read. */
  async open(puzzleId: string): Promise<void> {
    this.run.set(null);
    this.gone.set(false);
    this.message.set('');
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
    if (current && current.puzzleId === next.puzzleId && next.revision <= current.revision) {
      return false;
    }
    this.run.set(next);
    return true;
  }

  /** Makes one move. Never rejects: a failure becomes `message`. */
  async move(move: MessageInitShape<typeof PuzzleMoveSchema>): Promise<void> {
    const run = this.run();
    if (!run || run.solved || run.stopped) {
      return;
    }
    const key = this.makeKey();
    this.pending.update((n) => n + 1);
    this.message.set('');
    try {
      const answer = await this.sendWithRetry(run.puzzleId, move, key);
      this.apply(answer.run);
    } catch (err) {
      this.message.set(puzzleErrorMessage(err, 'fazer essa jogada'));
      const blocked = puzzleBlocked(err);
      const notFound = ConnectError.from(err, Code.Unavailable).code === Code.NotFound;
      if (blocked || notFound) {
        await this.refresh();
      }
    } finally {
      this.pending.update((n) => n - 1);
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
