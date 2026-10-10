import { computed, signal } from '@angular/core';

import type {
  GroupCheckMemberView,
  GroupCheckView,
} from '../../../gen/meurpg/play/v1/contest_types_pb';
import type { ContestClient } from './contest-client';

/**
 * The group check of the open session as this player reads it (`GetGroupCheck`): the check that is open, or the latest closed
 * one. A player reads only their own roll, and "passou" or "falhou" only when the master shows the DC (RN-20): the server
 * leaves the rest out and this keeps nothing more. The page reads it again on each `group_check_changed`; a slow answer that
 * arrives after a newer question is dropped. Nothing is kept in the browser.
 */
export class GroupCheckState {
  readonly view = signal<GroupCheckView | null>(null);
  /** The player's own character in the check, the only member a player is sent. */
  readonly own = computed<GroupCheckMemberView | null>(() => this.view()?.members[0] ?? null);
  private asked = 0;

  async load(api: ContestClient, campaignId: string): Promise<void> {
    const mine = ++this.asked;
    try {
      const res = await api.groupCheck(campaignId);
      if (mine === this.asked) {
        this.view.set(res);
      }
    } catch {
      // Best effort: the next `group_check_changed` asks again.
    }
  }

  /** The answer of the player's own roll: the check as it is now. */
  apply(view: GroupCheckView): void {
    this.asked++;
    this.view.set(view);
  }

  clear(): void {
    this.asked++;
    this.view.set(null);
  }
}
