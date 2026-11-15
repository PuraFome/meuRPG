import { Injectable, computed, inject, signal } from '@angular/core';

import { ChargesClient } from './charges-client';
import {
  dawnWaiting,
  partyChargesOf,
  type PartyChargesVm,
  type PartyMemberSheet,
} from './party-charges';

/**
 * The charges and the ammunition of the party's sheets, read by the master's page: "Cargas e munição" draws them and
 * the long rest's "Amanhecer" row lists the items a dawn would touch. One read for each character, again after the
 * master changes something. Only the master's page loads it.
 */
@Injectable({ providedIn: 'root' })
export class PartyCharges {
  private readonly client = inject(ChargesClient);

  readonly status = signal<'idle' | 'loading' | 'ready' | 'error'>('idle');
  private readonly members = signal<readonly PartyMemberSheet[]>([]);
  readonly view = computed<PartyChargesVm>(() => partyChargesOf(this.members()));
  readonly waiting = computed(() => dawnWaiting(this.view()));

  private latest = 0;

  nameOf(characterId: string): string {
    return this.members().find((m) => m.characterId === characterId)?.name ?? '';
  }

  /** Reads the sheets of the party. A failed read keeps the last list and says `error`. */
  async load(
    campaignId: string,
    party: readonly { readonly characterId: string; readonly name: string }[],
  ): Promise<void> {
    const mine = ++this.latest;
    this.status.set('loading');
    try {
      const members = await Promise.all(
        party.map(async (p) => ({
          characterId: p.characterId,
          name: p.name,
          ...(await this.client.sheetOf(campaignId, p.characterId)),
        })),
      );
      if (mine === this.latest) {
        this.members.set(members);
        this.status.set('ready');
      }
    } catch {
      if (mine === this.latest) {
        this.status.set('error');
      }
    }
  }
}
