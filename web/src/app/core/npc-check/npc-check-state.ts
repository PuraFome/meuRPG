import { computed, signal } from '@angular/core';

import type { NpcCheckRead } from '../../../gen/meurpg/play/v1/scene_pb';
import { ActionKey, newKey } from '../connect/idempotency';
import { type NpcCardVm, cardOf } from './npc-check-card';
import { skipErrorText } from './npc-check-errors';

/** What the master's card needs from the server. */
export interface NpcCheckApi {
  /** The latest check of the open session, `null` when it has had none or is not open. */
  latest(campaignId: string): Promise<NpcCheckRead | null>;
  skip(campaignId: string, holdId: string, key: string): Promise<NpcCheckRead>;
}

/**
 * The master's card of the NPC check: the check the server holds or settled last (`GetNpcCheck`, read again on every hint that
 * the held rolls changed and after a reload), "Continuar sem esperar", and the "X" that hides a settled card on this device
 * only, in memory (nothing is stored in the browser). A hidden card comes back when the next check is rolled.
 */
export class NpcCheckState {
  private readonly current = signal<NpcCheckRead | null>(null);
  private readonly hidden = signal('');
  private reads = 0;
  private readonly keys: ActionKey;
  readonly error = signal('');
  readonly busy = signal(false);

  /** The card to draw: the check as the server last said it, unless the master hid it. */
  readonly card = computed<NpcCardVm | null>(() => {
    const read = this.current();
    const card = read ? cardOf(read) : null;
    return card && card.key === this.hidden() && card.settled ? null : card;
  });

  constructor(
    private readonly api: NpcCheckApi,
    make: () => string = newKey,
  ) {
    this.keys = new ActionKey(make);
  }

  /** Reads the latest check. A read that a newer one overtook is dropped, and a failed one keeps what the card shows. */
  async load(campaignId: string): Promise<void> {
    const mine = ++this.reads;
    try {
      const read = await this.api.latest(campaignId);
      if (mine === this.reads) {
        this.current.set(read);
      }
    } catch {
      // The next hint reads again; the card stays as it was.
    }
  }

  /** A check the master just rolled: it is the new card, and an "X" of an older one never hides it. */
  apply(read: NpcCheckRead): void {
    this.reads++;
    this.hidden.set('');
    this.current.set(read);
  }

  /** The "X" of a settled card. */
  dismiss(): void {
    const card = this.card();
    if (card?.settled) {
      this.hidden.set(card.key);
    }
  }

  /** "Continuar sem esperar": closes every question that still waits; the answer is the settled check. */
  async skip(campaignId: string): Promise<void> {
    const hold = this.current()?.holdId;
    if (!hold || this.busy()) {
      return;
    }
    this.busy.set(true);
    this.error.set('');
    try {
      const read = await this.api.skip(campaignId, hold, this.keys.keyFor({ skip: hold }));
      this.keys.renew();
      this.reads++;
      this.current.set(read);
    } catch (err) {
      this.error.set(skipErrorText(err));
    } finally {
      this.busy.set(false);
    }
  }
}
