import { signal } from '@angular/core';

import type { HeldRollOffer } from '../../../gen/meurpg/play/v1/combat_pb';
import { wasAnsweredHere } from './held-roll-controller';

/** What the roller reads when the question they were shown is gone and they did not answer it. */
export const MASTER_ANSWERED_TEXT = 'O mestre respondeu por você.';

/** The question as the roller sees it: the hold and which of its questions (Lucky's or the die's) it asks. */
const questionKey = (offer: HeldRollOffer | null) =>
  offer ? `${offer.holdId}:${offer.question.case}` : '';

/**
 * Watches the question a place shows its roller. When it is gone, or the roll moved on to its next question, and this
 * browser did not send the answer (`wasAnsweredHere`), the master answered for them (or the roll was dropped), and
 * `notice` says so until a new question comes or it is dismissed. The player's view carries no result of the master's
 * answer, so the line is the short form. Plain class a place builds and feeds with the question it reads.
 */
export class HeldVanishWatch {
  readonly notice = signal('');
  private last: HeldRollOffer | null = null;

  /** The question the place reads now (or `null`): call it every time the place reads its questions. */
  observe(offer: HeldRollOffer | null | undefined): void {
    const before = this.last;
    this.last = offer ?? null;
    if (questionKey(before) === questionKey(this.last)) {
      return;
    }
    if (before && !wasAnsweredHere(before)) {
      this.notice.set(MASTER_ANSWERED_TEXT);
    } else if (this.last) {
      this.notice.set('');
    }
  }

  dismiss(): void {
    this.notice.set('');
  }
}
