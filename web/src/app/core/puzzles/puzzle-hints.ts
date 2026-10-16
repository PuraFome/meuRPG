import { type WritableSignal, signal } from '@angular/core';
import { Code, ConnectError } from '@connectrpc/connect';

import type { LuckyOffer, OutsideInspirationOffer } from '../../../gen/meurpg/play/v1/combat_pb';
import {
  PuzzleBlockedReason,
  type PuzzleRun,
  type TryPuzzleHintResponse,
} from '../../../gen/meurpg/play/v1/puzzles_pb';
import { ActionKey } from '../connect/idempotency';
import { ExtraDiceState } from '../effects/extra-dice-state';
import { needsTwoD20, takesTwo } from '../play/check-roll';
import type { InspirationRoll, LuckyRoll } from '../resources/resources-client';
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
  readonly busy = signal(false);
  /** The last try for a hint, until the next one or the next change of the puzzle. */
  readonly last = signal<HintTry | null>(null);
  /** The try's d20 was rolled and kept: the character holds a Bardic Inspiration die, and the player decides before the result. */
  readonly offer = signal<OutsideInspirationOffer | null>(null);
  /** The try's d20 was rolled and kept because a halfling's d20 shows a 1: the Lucky question, asked before the die's. */
  readonly lucky = signal<LuckyOffer | null>(null);
  /** The next try with a real die takes two d20: the server said so, or the last result had advantage or disadvantage. */
  readonly pair = signal(false);
  /** The d4 an effect adds to a try with a real die (Orientação): asked when the server says the roll takes them. */
  readonly extraDice = new ExtraDiceState();
  /** The server refused a try because the table rolls its dice the other way: the page reads the campaign's dice mode again. */
  readonly diceModeStale = signal(0);
  /** One key per Lucky answer (roll again this way, or keep the 1): a retry of a lost answer never rolls the d20 twice. */
  private readonly luckyKeys = new ActionKey();

  constructor(private readonly host: HintHost) {}

  /** A puzzle was opened: nothing of the one before stays on the form. */
  reset(): void {
    this.busy.set(false);
    this.last.set(null);
    this.pair.set(false);
    this.extraDice.fields.set([]);
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
    this.busy.set(true);
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
        this.busy.set(false);
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

  /** What a try came to: the run as this player reads it, their roll and the form for the next one. */
  private settle(answer: TryPuzzleHintResponse, run: PuzzleRun): void {
    if (answer.luckyOffer || answer.inspirationOffer) {
      // Nothing was written: the question is the whole answer, the Lucky one first, then the die's.
      this.lucky.set(answer.luckyOffer ?? null);
      this.offer.set(answer.luckyOffer ? null : (answer.inspirationOffer ?? null));
      return;
    }
    this.offer.set(null);
    this.lucky.set(null);
    this.host.apply(answer.run ?? run);
    this.last.set({
      passed: answer.passed,
      roll: answer.roll,
      mode: answer.mode,
      sources: answer.sources,
    });
    this.pair.set(takesTwo(answer.mode));
    this.extraDice.fields.set([]);
  }

  /** The answer about the die: the try is made now, with the d20 it rolled and the die when it was used. */
  async answerDie(use: boolean, die: InspirationRoll | null): Promise<void> {
    const host = this.host;
    const offer = this.offer();
    const run = host.run();
    if (!offer || !run || this.busy() || !host.api.answerHintDie) {
      return;
    }
    const seq = host.openSeq();
    this.busy.set(true);
    host.message.set('');
    try {
      const answer = await host.api.answerHintDie(
        host.campaignId(),
        offer.holdId,
        use,
        die,
        host.makeKey(),
      );
      if (seq === host.openSeq()) {
        this.settle(answer, run);
      }
    } catch (err) {
      if (seq === host.openSeq()) {
        host.message.set(
          puzzleErrorMessage(err, 'responder sobre a Inspiração de Bardo', 'player'),
        );
      }
    } finally {
      if (seq === host.openSeq()) {
        this.busy.set(false);
      }
    }
  }

  /** "Rolar de novo" (rolled in the app, or the typed face) or "Ficar com o 1": the try is made now with the new d20, or the
   * Bardic Inspiration question comes next, on the new roll. The key stays the same until the answer gets through. */
  async answerLucky(reroll: boolean, die: LuckyRoll | null): Promise<void> {
    const host = this.host;
    const offer = this.lucky();
    const run = host.run();
    if (!offer || !run || this.busy() || !host.api.answerHintLucky) {
      return;
    }
    const seq = host.openSeq();
    this.busy.set(true);
    host.message.set('');
    try {
      const answer = await host.api.answerHintLucky(
        host.campaignId(),
        offer.holdId,
        reroll,
        die,
        this.luckyKeys.keyFor({ hold: offer.holdId, reroll, die }),
      );
      this.luckyKeys.renew();
      if (seq === host.openSeq()) {
        this.settle(answer, run);
      }
    } catch (err) {
      if (seq === host.openSeq()) {
        host.message.set(puzzleErrorMessage(err, 'responder ao Sortudo', 'player'));
      }
    } finally {
      if (seq === host.openSeq()) {
        this.busy.set(false);
      }
    }
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
