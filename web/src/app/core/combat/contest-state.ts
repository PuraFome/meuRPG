import { create } from '@bufbuild/protobuf';
import { computed, signal } from '@angular/core';

import {
  type GetContestStateResponse,
  GetContestStateResponseSchema,
} from '../../../gen/meurpg/play/v1/contests_pb';
import {
  type ContestView,
  type GrappleView,
  type HelpView,
  type HideAttemptView,
  HideAttemptStatus,
} from '../../../gen/meurpg/play/v1/contest_types_pb';
import type { ContestClient } from './contest-client';

/**
 * The contest facts of the combat on this screen (`GetContestState`): the contests the player is in, the Hide actions, who is
 * hidden, the Helps that hold. The page reads it again whenever the combat changes (every contest change reaches the table as
 * `encounter_changed`); a slow answer that arrives after a newer question was sent is dropped. A call's answer (a contest, an
 * attempt) is merged at once, so a sheet shows its result without waiting for the read. Nothing here is kept in the browser.
 */
export class ContestState {
  readonly data = signal<GetContestStateResponse | null>(null);
  /** The contests the caller is in, newest first. */
  readonly contests = computed<readonly ContestView[]>(() => this.data()?.contests ?? []);
  /** The Hide actions of the caller's characters, the latest of each. */
  readonly attempts = computed<readonly HideAttemptView[]>(() => this.data()?.hideAttempts ?? []);
  /** The Helps that hold, for every member. */
  readonly helps = computed<readonly HelpView[]>(() => this.data()?.helps ?? []);
  /** The Hide actions that wait for the master's decision (his only: a player reads their own latest). */
  readonly pendingAttempts = computed<readonly HideAttemptView[]>(() =>
    this.attempts().filter((a) => a.status === HideAttemptStatus.PENDING),
  );
  /** Who holds whom, as the caller reads it (the master: all). */
  readonly grapples = computed<readonly GrappleView[]>(() => this.data()?.grapples ?? []);
  /** The combatants marked surprised. */
  readonly surprised = computed<readonly string[]>(() => this.data()?.surprise?.combatantIds ?? []);
  private asked = 0;
  private subject = '';

  async load(api: ContestClient, campaignId: string, encounterId: string): Promise<void> {
    const mine = ++this.asked;
    // Another combat's contests are never shown for this one while the answer comes.
    if (this.subject !== encounterId) {
      this.subject = encounterId;
      this.data.set(null);
    }
    try {
      const res = await api.state(campaignId, encounterId);
      if (mine === this.asked) {
        this.data.set(res);
      }
    } catch {
      // Best effort: the next change of the combat asks again.
    }
  }

  /** The contest with this id, as it is now. */
  contest(id: string): ContestView | undefined {
    return this.contests().find((c) => c.id === id);
  }

  /** The Hide action with this id, as it is now. */
  attempt(id: string): HideAttemptView | undefined {
    return this.attempts().find((a) => a.id === id);
  }

  /** A contest a call answered with: put in place of the one that was there (or first), without waiting for the read. */
  applyContest(contest: ContestView): void {
    const current = this.data() ?? create(GetContestStateResponseSchema);
    const others = current.contests.filter((c) => c.id !== contest.id);
    this.data.set({ ...current, contests: [contest, ...others] });
  }

  /** A Hide attempt a call answered with. */
  applyAttempt(attempt: HideAttemptView): void {
    const current = this.data() ?? create(GetContestStateResponseSchema);
    const others = current.hideAttempts.filter((a) => a.id !== attempt.id);
    this.data.set({ ...current, hideAttempts: [attempt, ...others] });
  }

  clear(): void {
    this.asked++;
    this.subject = '';
    this.data.set(null);
  }
}
