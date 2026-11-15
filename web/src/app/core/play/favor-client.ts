import { Injectable, inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';

import { FavorService } from '../../../gen/meurpg/play/v1/favor_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';

/** One character that has the master's Inspiração. */
export interface FavorHolderVm {
  readonly characterId: string;
  readonly name: string;
}

/**
 * The only client of `FavorService` (the master's Inspiração, RN-43): the read and the grant. The pass is not built on the
 * server, so it is not here. Callers map a refusal with `favorBlocked` (`favor-errors.ts`). `providedIn: 'root'`.
 */
@Injectable({ providedIn: 'root' })
export class FavorClient {
  private readonly client = createClient(FavorService, inject(CONNECT_TRANSPORT));

  /** Who has it: the master reads every holder of the campaign, a player only their own characters (RN-10). */
  async read(campaignId: string): Promise<readonly FavorHolderVm[]> {
    const response = await this.client.getFavor({ campaignId });
    return response.holders.map((h) => ({ characterId: h.characterId, name: h.characterName }));
  }

  /** The master's grant, in an open session. `idempotencyKey` is one per tap: a retry of the same tap answers as the first. */
  async grant(campaignId: string, characterId: string, idempotencyKey: string): Promise<void> {
    await this.client.grantFavor({ campaignId, characterId, idempotencyKey });
  }
}
