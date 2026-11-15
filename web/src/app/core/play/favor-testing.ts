import { Code, ConnectError } from '@connectrpc/connect';

import { FavorBlockedReason, FavorBlockedSchema } from '../../../gen/meurpg/play/v1/favor_pb';
import type { FavorClient, FavorHolderVm } from './favor-client';
import { noOpenSession } from './table-screen-testing';

export { noOpenSession };

/** A `failed_precondition` with the typed `FavorBlocked` detail the server sends. */
export function favorRefused(reason: FavorBlockedReason): ConnectError {
  return new ConnectError('blocked', Code.FailedPrecondition, undefined, [
    { desc: FavorBlockedSchema, value: { reason } },
  ]);
}

/** A `FavorClient` the specs steer: what the read answers, and what the grant does. */
export class FakeFavorClient {
  /** What `read` answers; a function can throw to be a failed read. */
  holders: readonly FavorHolderVm[] | (() => readonly FavorHolderVm[]) = [];
  /** What `grant` does: `null` is a grant that works. */
  grantError: unknown = null;
  readonly reads: string[] = [];
  readonly grants: { campaignId: string; characterId: string; key: string }[] = [];

  read(campaignId: string): Promise<readonly FavorHolderVm[]> {
    this.reads.push(campaignId);
    try {
      return Promise.resolve(typeof this.holders === 'function' ? this.holders() : this.holders);
    } catch (err) {
      return Promise.reject(err);
    }
  }

  grant(campaignId: string, characterId: string, key: string): Promise<void> {
    this.grants.push({ campaignId, characterId, key });
    return this.grantError ? Promise.reject(this.grantError) : Promise.resolve();
  }

  asClient(): FavorClient {
    return this as unknown as FavorClient;
  }
}

/** A read that fails the way a down network does. */
export function failedRead(): never {
  throw new ConnectError('down', Code.Unavailable);
}

/** Puts a `FavorState` where a session is open and `holders` (id to name) have the Inspiração, with no read. */
export function openFavor(
  favor: {
    campaignId: { set(v: string): void };
    holders: { set(v: ReadonlyMap<string, string>): void };
  },
  holders: Record<string, string> = {},
): void {
  favor.campaignId.set('camp');
  favor.holders.set(new Map(Object.entries(holders)));
}
