import { Injectable, computed, inject, signal } from '@angular/core';

import { describeCharacterError } from '../../../core/characters/character-errors';
import {
  CampaignCharacterListItemVm,
  CampaignCharactersSource,
  CampaignCharactersVm,
} from './campaign-characters.types';

export type CampaignRosterState =
  | { readonly status: 'loading' }
  | { readonly status: 'ready'; readonly vm: CampaignCharactersVm }
  | { readonly status: 'error'; readonly message: string };

/**
 * The campaign's characters, read once per page (`ListCharacters`) and shared by
 * what draws them: the players' panel on "Mesa", the NPCs' panel on "Preparação"
 * and the count on the "Mesa" tab ("Mesa · 1": one character waits for the
 * master's approval). Provided by `CampaignDetail`, so a second reader never
 * means a second call, and a tab change keeps the list.
 */
@Injectable()
export class CampaignRoster {
  private readonly source = inject(CampaignCharactersSource);

  readonly state = signal<CampaignRosterState>({ status: 'loading' });

  /** The players' characters that wait for the master's approval (RN-15, MR-024), in the order the server sends them. */
  readonly awaiting = computed<readonly CampaignCharacterListItemVm[]>(() => {
    const s = this.state();
    return s.status === 'ready' ? s.vm.playerCharacters.filter((c) => c.state === 'pending') : [];
  });

  private campaignId = '';
  private seq = 0;
  private inFlight: Promise<void> | null = null;

  /** Reads the list unless this campaign's is already here or on its way. */
  ensureLoaded(campaignId: string): Promise<void> {
    if (campaignId === this.campaignId && (this.inFlight || this.state().status === 'ready')) {
      return this.inFlight ?? Promise.resolve();
    }
    return this.reload(campaignId);
  }

  /**
   * Reads the list again; an answer of an older read is dropped. `quietly` keeps
   * what is on screen while it reads (a change the master made, not a new page).
   */
  reload(campaignId: string = this.campaignId, quietly = false): Promise<void> {
    const mine = ++this.seq;
    const keep = quietly && campaignId === this.campaignId && this.state().status === 'ready';
    this.campaignId = campaignId;
    if (!keep) {
      this.state.set({ status: 'loading' });
    }
    const read = this.source.listCharacters(campaignId).then(
      (vm) => {
        if (mine === this.seq) {
          this.state.set({ status: 'ready', vm });
        }
      },
      (err: unknown) => {
        if (mine === this.seq) {
          this.state.set({ status: 'error', message: describeCharacterError(err) });
        }
      },
    );
    this.inFlight = read;
    void read.finally(() => {
      if (this.inFlight === read) {
        this.inFlight = null;
      }
    });
    return read;
  }
}
