import { type WritableSignal, computed, signal } from '@angular/core';
import { Code, ConnectError } from '@connectrpc/connect';

import type { AnswerHeldRollResponse } from '../../../gen/meurpg/play/v1/resources_pb';
import {
  PuzzleBlockedReason,
  type PuzzleRun,
  type TryPuzzleHintResponse,
} from '../../../gen/meurpg/play/v1/puzzles_pb';
import { ExtraDiceState } from '../effects/extra-dice-state';
import { HeldRollController } from '../held-rolls/held-roll-controller';
import { type HeldAnswer, heldOfferOf } from '../held-rolls/held-roll-offer';
import { needsTwoD20, takesTwo } from '../play/check-roll';
import { puzzleBlocked, puzzleErrorMessage } from './puzzle-errors';
import { withRetry } from './puzzle-play-support';
import type { HintTry, PlayApi } from './puzzle-play-api';
import type { HintDie } from './puzzles-client';

/** What the tries for a hint need from the play that owns them: the run on screen, the line for messages, and the reads. */
export interface HintHost {
  readonly api: PlayApi;
  readonly campaignId: () => string;
  readonly wait: (ms: number) => Promise<void>;
  readonly makeKey: () => string;
  readonly run: () => PuzzleRun | null;
  readonly gone: () => boolean;
  readonly message: WritableSignal<string>;
  /** Counts the puzzles opened: an answer that began for an earlier one never touches the one on screen. */
  readonly openSeq: () => number;
  apply(run: PuzzleRun): boolean;
  refresh(): Promise<void>;
}

/**
 * The player's tries for a hint by a skill check (RN-18) on the open puzzle, apart from its moves: the try in flight, its
 * result, the questions it may wait on (the halfling's Lucky, a Bardic Inspiration die) and the dice a real roll asks for.
 */
export class PuzzleHints {
  /** A try for a hint is in flight. */
  private readonly trying = signal(false);
  /** A try, or the answer to the question it waits on, is in flight. */
  readonly busy = computed(() => this.trying() || this.held.busy());
  /** The last try for a hint, until the next one or the next change of the puzzle. */
  readonly last = signal<HintTry | null>(null);
  /** The try's d20 was rolled and kept and the try waits for a question: the halfling's Lucky about a 1 first, then the
   * Bardic Inspiration die's. The player decides before the result. */
  readonly held: HeldRollController;
  /** The next try with a real die takes two d20: the server said so, or the last result had advantage or disadvantage. */
  readonly pair = signal(false);
  /** The d4 an effect adds to a try with a real die (Orientação): asked when the server says the roll takes them. */
  readonly extraDice = new ExtraDiceState();
  /** The server refused a try because the table rolls its dice the other way: the page reads the campaign's dice mode again. */
  readonly diceModeStale = signal(0);
  /** The puzzle and the run an answer began for: an answer that comes after another puzzle was opened touches nothing. */
  private answering: { readonly seq: number; readonly run: PuzzleRun } | null = null;

  constructor(private readonly host: HintHost) {
    this.held = new HeldRollController({
      send: (holdId, answer, key) =>
        host.api.answerHeldRoll!(host.campaignId(), holdId, answer, key),
      settle: (res) => this.answeredHeld(res),
      newKey: host.makeKey,
      describeError: (err, answer) =>
        puzzleErrorMessage(
          err,
          answer.kind === 'lucky'
            ? 'responder ao Sortudo'
            : 'responder sobre a Inspiração de Bardo',
          'player',
        ),
    });
  }

  /** A puzzle was opened: nothing of the one before stays on the form. */
  reset(): void {
    this.trying.set(false);
    this.held.ask(null);
    this.last.set(null);
    this.pair.set(false);
    this.extraDice.asked.set([]);
  }

  /**
   * "Tentar uma dica": rolls the puzzle's skill check for the next hint, in the app or from the face of a real die. A pass puts the
   * hint in `run.hints` (this player's alone); the answer is the run as this player reads it, and their own roll. The DC never
   * reaches here. The same key goes with a retry, so a lost answer never rolls twice.
   */
  async try(die: HintDie): Promise<void> {
    const host = this.host;
    const run = host.run();
    if (!run || !run.canTryHint || this.busy() || host.gone()) {
      return;
    }
    const extra = this.extraDice.take(!('inApp' in die));
    if (extra === null) {
      host.message.set(this.extraDice.missingText());
      return;
    }
    const seq = host.openSeq();
    const key = host.makeKey();
    this.trying.set(true);
    host.message.set('');
    try {
      const answer = await withRetry(
        () => host.api.tryHint(host.campaignId(), run.puzzleId, die, key, extra),
        host.wait,
      );
      if (seq === host.openSeq()) {
        this.settle(answer, run);
      }
    } catch (err) {
      if (seq === host.openSeq()) {
        await this.failed(err, die);
      }
    } finally {
      if (seq === host.openSeq()) {
        this.trying.set(false);
      }
    }
  }

  /** What a failed try says, and whether the screen is stale (then the run is read again). */
  private async failed(err: unknown, die: HintDie): Promise<void> {
    if (this.askForMore(err, die)) {
      return;
    }
    this.host.message.set(puzzleErrorMessage(err, 'tentar a dica', 'player'));
    if (puzzleBlocked(err)?.reason === PuzzleBlockedReason.WRONG_DICE_MODE) {
      // The table's dice mode changed under the page: read it again, so the way to roll on screen is the one the server wants.
      this.diceModeStale.update((n) => n + 1);
    }
    if (puzzleBlocked(err) || ConnectError.from(err, Code.Unavailable).code === Code.NotFound) {
      await this.host.refresh();
    }
  }

  /** What a try came to: the question it waits on, or the run as this player reads it, their roll and the form for the
   * next one. */
  private settle(answer: TryPuzzleHintResponse, run: PuzzleRun): void {
    const asked = heldOfferOf(answer);
    if (asked) {
      // Nothing was written: the question is the whole answer, the Lucky one first, then the die's.
      this.held.ask(asked);
      return;
    }
    this.held.ask(null);
    this.host.apply(answer.run ?? run);
    this.last.set({
      passed: answer.passed,
      roll: answer.roll,
      mode: answer.mode,
      sources: answer.sources,
    });
    this.pair.set(takesTwo(answer.mode));
    this.extraDice.asked.set([]);
  }

  /**
   * The player's answer to the question the try waits on: the try is made now with the new d20 (and the die when it was
   * used), or the die's question comes next, on the new roll. An answer that began for a puzzle closed since is not told.
   */
  async answer(answer: HeldAnswer): Promise<void> {
    const host = this.host;
    const run = host.run();
    const asked = this.held.offer();
    if (!asked || !run || this.busy() || !host.api.answerHeldRoll) {
      return;
    }
    const seq = host.openSeq();
    this.answering = { seq, run };
    host.message.set('');
    await this.held.answer(answer);
    if (seq === host.openSeq()) {
      host.message.set(this.held.error());
    } else if (this.held.offer()?.holdId === asked.holdId) {
      // The question of the puzzle left behind: the one on screen has none.
      this.held.ask(null);
    }
  }

  /** The answer came back: the try written, or the next question (already the controller's). */
  private answeredHeld(res: AnswerHeldRollResponse): void {
    if (res.result.case !== 'hintTry') {
      throw new Error('AnswerHeldRoll answered without its hint try');
    }
    const began = this.answering;
    if (!began || began.seq !== this.host.openSeq() || res.next) {
      return;
    }
    this.settle(res.result.value, began.run);
  }

  /** A real die refused because the roll takes more dice (the second d20, or the d4 of an effect): the form asks for them. */
  private askForMore(err: unknown, die: HintDie): boolean {
    if (needsTwoD20(err) && 'face' in die) {
      // The form now asks for both d20, and nothing is said as an error.
      this.pair.set(true);
      return true;
    }
    const more = this.extraDice.fromRefusal(err);
    if (more) {
      this.host.message.set(more);
    }
    return more !== '';
  }
}
