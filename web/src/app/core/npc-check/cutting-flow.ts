import { signal } from '@angular/core';

import type { HeldRollOffer } from '../../../gen/meurpg/play/v1/combat_pb';
import type { CuttingWordsAnswered } from '../../../gen/meurpg/play/v1/resources_pb';
import { ActionKey, newKey } from '../connect/idempotency';
import { type CuttingResultVm, isCuttingOffer, resultView } from './cutting-question';
import type { CutAnswer } from './npc-check-client';
import { type CutError, cutErrorOf } from './npc-check-errors';

/** What the bard's question needs from the server. */
export interface CutApi {
  answer(
    campaignId: string,
    holdId: string,
    answer: CutAnswer,
    idempotencyKey: string,
  ): Promise<CuttingWordsAnswered>;
}

/**
 * Where the bard is in the question of an NPC check: `asking` (the question is open), `result` (it cut and reads its die),
 * `passed` (it let it pass), `skipped` (the master went on without the answer: the offer is gone) or `late` (its answer came
 * after the master went on). The list of held rolls feeds `sync`; a hold the bard has answered or seen closed is never asked
 * again. The offer holds the die and the uses, never a number of the creature's (RN-10).
 */
export type CutPhase = 'asking' | 'result' | 'passed' | 'skipped' | 'late';

export class CuttingFlow {
  readonly offer = signal<HeldRollOffer | null>(null);
  readonly phase = signal<CutPhase>('asking');
  readonly busy = signal(false);
  readonly error = signal<CutError | null>(null);
  readonly result = signal<CuttingResultVm | null>(null);
  private readonly done = new Set<string>();
  private readonly keys: ActionKey;

  constructor(
    private readonly api: CutApi,
    private readonly campaignId: () => string,
    make: () => string = newKey,
  ) {
    this.keys = new ActionKey(make);
  }

  /** The held rolls the server lists for this player. */
  sync(offers: readonly HeldRollOffer[]): void {
    const next = offers.find((o) => isCuttingOffer(o) && !this.done.has(o.holdId));
    if (next) {
      const same = this.offer()?.holdId === next.holdId;
      this.offer.set(next);
      if (!same) {
        this.phase.set('asking');
        this.error.set(null);
        this.result.set(null);
      }
      return;
    }
    if (this.offer() && this.phase() === 'asking' && !this.busy()) {
      this.phase.set('skipped');
    }
  }

  /** The bard's answer: `cut` with the face of a physical die, or `null` for the app's. */
  async answer(cut: boolean, face: number | null): Promise<void> {
    const offer = this.offer();
    if (!offer || this.busy() || this.phase() !== 'asking') {
      return;
    }
    const answer: CutAnswer = cut ? { cut, die: face === null ? { inApp: true } : { face } } : { cut };
    this.busy.set(true);
    this.error.set(null);
    try {
      const read = await this.api.answer(this.campaignId(), offer.holdId, answer, this.keys.keyFor({ hold: offer.holdId, answer }));
      this.keys.renew();
      this.done.add(offer.holdId);
      this.result.set(cut ? resultView(offer, read) : null);
      this.phase.set(cut ? 'result' : 'passed');
    } catch (err) {
      const failure = cutErrorOf(err, cut);
      this.error.set(failure);
      if (failure.closed) {
        this.done.add(offer.holdId);
        this.phase.set('late');
      }
    } finally {
      this.busy.set(false);
    }
  }

  /** The sheet closed: what was answered or closed is over, and the next hold may come. */
  acknowledge(): void {
    const offer = this.offer();
    if (offer && this.phase() !== 'asking') {
      this.done.add(offer.holdId);
    }
    if (offer && this.phase() === 'asking') {
      return;
    }
    this.offer.set(null);
    this.result.set(null);
    this.error.set(null);
    this.phase.set('asking');
  }
}
