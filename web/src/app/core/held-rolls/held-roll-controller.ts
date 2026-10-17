import { type WritableSignal, signal } from '@angular/core';
import { timestampMs } from '@bufbuild/protobuf/wkt';

import type { HeldRollOffer } from '../../../gen/meurpg/play/v1/combat_pb';
import type { AnswerHeldRollResponse } from '../../../gen/meurpg/play/v1/resources_pb';
import { ActionKey, newKey } from '../connect/idempotency';
import { type HeldRefusal, heldRollErrorMessage, heldRollRefusal } from './held-roll-errors';
import type { HeldAnswer } from './held-roll-offer';

/** How long the master waits before answering for the roller (the roller answers first), in seconds. */
export const MASTER_WAIT_SECONDS = 10;
const MS_PER_SECOND = 1000;

/** The questions this browser answered, each as the hold and the question it asked (Lucky's and the die's of one roll are
 * two answers). A place that sees its question vanish tells the master's answer from its own by it. */
const answeredHere = new Set<string>();

const answeredKey = (offer: HeldRollOffer) => `${offer.holdId}:${offer.question.case}`;

/** Remembers that this browser sends the answer to the question: from the moment it goes, as the stream's hint may come
 * back before the response does. */
export function markAnsweredHere(offer: HeldRollOffer): void {
  answeredHere.add(answeredKey(offer));
}

/** Whether this browser answered the question. */
export function wasAnsweredHere(offer: HeldRollOffer): boolean {
  return answeredHere.has(answeredKey(offer));
}

/** Forgets one answer (it did not get through), or all of them (the specs). */
export function forgetAnsweredHere(offer?: HeldRollOffer): void {
  if (offer) {
    answeredHere.delete(answeredKey(offer));
  } else {
    answeredHere.clear();
  }
}

/** What the controller needs from the place that asks. */
export interface HeldRollHost {
  /** Sends `AnswerHeldRoll` for the hold: a function, so a place that never asks does not build the client. */
  readonly send: (
    holdId: string,
    answer: HeldAnswer,
    idempotencyKey: string,
  ) => Promise<AnswerHeldRollResponse>;
  /** Takes the answer: the path's own result (`res.result`) finishes the roll, or `res.next` is the question that comes
   * after it (already the controller's `offer`). `asked` is the question that was answered. May throw: the question
   * then stays and the error is worded. */
  readonly settle: (
    res: AnswerHeldRollResponse,
    answered: { readonly answer: HeldAnswer; readonly asked: HeldRollOffer },
  ) => void | Promise<void>;
  /** The sheet's own flags, when the question shares them with the rest of the sheet; else the controller keeps its own. */
  readonly busy?: WritableSignal<boolean>;
  readonly error?: WritableSignal<string>;
  /** The master answers for the roller (the same RPC, the other side of the table): `waitSeconds` counts. */
  readonly master?: boolean;
  /** The time in milliseconds, for the master's wait; the page corrects it by the server's clock. */
  readonly now?: () => number;
  /** Mints the idempotency keys. */
  readonly newKey?: () => string;
  /** Words a failure in the place's own terms (a puzzle says "Este quebra-cabeça não existe mais"); a lost race and the
   * master's early answer keep the controller's words. */
  readonly describeError?: (err: unknown, answer: HeldAnswer) => string;
}

/**
 * The question a roll that waits asks, and its answers, whoever answers: the roller's player on a sheet, or the master
 * for the roller. One instance per question on screen. It holds the offer (`ask` it, answer it with `answer`), sends
 * `ResourceService.AnswerHeldRoll` under one idempotency key per answer (renewed once the answer got through, so a
 * retry after a lost answer never rolls twice), takes `next` as the roll's following question (Lucky's, then the
 * die's) and hands the response to the host to finish. Plain class a sheet or a panel builds.
 */
export class HeldRollController {
  /** The question that waits, or `null`. */
  readonly offer = signal<HeldRollOffer | null>(null);
  readonly busy: WritableSignal<boolean>;
  readonly error: WritableSignal<string>;
  /** Why the last answer was refused when that is not a failure: the race was lost, or the master was too early. */
  readonly refusal = signal<HeldRefusal | null>(null);
  private readonly keys: ActionKey;
  private readonly now: () => number;

  constructor(private readonly host: HeldRollHost) {
    this.busy = host.busy ?? signal(false);
    this.error = host.error ?? signal('');
    this.keys = new ActionKey(host.newKey ?? newKey);
    this.now = host.now ?? Date.now;
  }

  /** Shows the question (or none): a roll that was held, or one the screen read again. */
  ask(offer: HeldRollOffer | null | undefined): void {
    this.offer.set(offer ?? null);
    this.refusal.set(null);
  }

  /**
   * Seconds until the master may answer for the roller, counted from when the roll was held; `0` for the roller, and
   * once the time has passed. A refusal for being too early shows the same number again, so the countdown resumes.
   */
  waitSeconds(now: number = this.now()): number {
    const created = this.offer()?.createdAt;
    if (!this.host.master || !created) {
      return 0;
    }
    const left = timestampMs(created) + MASTER_WAIT_SECONDS * MS_PER_SECOND - now;
    return Math.max(0, Math.ceil(left / MS_PER_SECOND));
  }

  /** The player's answer to the question that waits. Nothing happens without a question, or while one is on the way. */
  async answer(answer: HeldAnswer): Promise<void> {
    const asked = this.offer();
    if (!asked || this.busy()) {
      return;
    }
    this.refusal.set(null);
    this.busy.set(true);
    this.error.set('');
    markAnsweredHere(asked);
    try {
      const res = await this.host.send(
        asked.holdId,
        answer,
        this.keys.keyFor({ hold: asked.holdId, answer }),
      );
      await this.host.settle(res, { answer, asked });
      this.keys.renew();
      this.offer.set(res.next ?? null);
    } catch (err) {
      forgetAnsweredHere(asked);
      const refusal = heldRollRefusal(err);
      this.refusal.set(refusal);
      this.error.set(
        (!refusal && this.host.describeError?.(err, answer)) ||
          heldRollErrorMessage(err, answer, {
            roller: asked.rollerLabel,
            seconds: this.waitSeconds(),
          }),
      );
    } finally {
      this.busy.set(false);
    }
  }
}
