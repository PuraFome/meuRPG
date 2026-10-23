import { Injectable, inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';

import { PlayService } from '../../../gen/meurpg/play/v1/play_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';
import { type FreeRollVm, WIRE_MODE, type FreeMode, toFreeRollVm } from './free-roll';

/** What `RollFreeDice` is asked. `idempotencyKey` is one per tap, sent again on a retry. */
export interface FreeRollRequest {
  readonly campaignId: string;
  readonly sides: number;
  readonly count: number;
  readonly modifier: number;
  readonly mode: FreeMode;
  readonly secret: boolean;
  readonly idempotencyKey: string;
}

/** `ListFreeRolls`: the log of the table, and the caller's own newest rolls. */
export interface FreeRollLists {
  readonly table: FreeRollVm[];
  readonly mine: FreeRollVm[];
}

/**
 * Thin wrapper around the free dice roller's two `PlayService` calls (MR-052). `providedIn: 'root'`, imported only by
 * the session page. Tests replace it with `{ provide: FreeDiceClient, useValue }`.
 */
@Injectable({ providedIn: 'root' })
export class FreeDiceClient {
  private readonly client = createClient(PlayService, inject(CONNECT_TRANSPORT));

  async list(campaignId: string): Promise<FreeRollLists> {
    const res = await this.client.listFreeRolls({ campaignId });
    return { table: res.table.map(toFreeRollVm), mine: res.mine.map(toFreeRollVm) };
  }

  async roll(request: FreeRollRequest): Promise<FreeRollVm> {
    const res = await this.client.rollFreeDice({
      campaignId: request.campaignId,
      sides: request.sides,
      count: request.count,
      modifier: request.modifier,
      mode: WIRE_MODE[request.mode],
      secret: request.secret,
      idempotencyKey: request.idempotencyKey,
    });
    if (!res.roll) {
      throw new Error('RollFreeDice answered without the roll');
    }
    return toFreeRollVm(res.roll);
  }
}
