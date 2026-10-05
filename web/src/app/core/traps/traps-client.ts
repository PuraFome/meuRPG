import { Injectable, inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';

import { PlayService } from '../../../gen/meurpg/play/v1/play_pb';
import {
  type ApplyTrapDamageResponse,
  type DiscardTrapDamageResponse,
  type FireTrapResponse,
  type ListTrapActivityResponse,
  type ListTrapDamagesResponse,
  type SearchForTrapsResponse,
  TrapSearchSkill,
} from '../../../gen/meurpg/play/v1/traps_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';

/** How the search's d20 comes (RN-18): the app rolls it, or the player typed the face of a real
 * die; for a Perception search with disadvantage a second face goes with it. */
export type SearchDie = { readonly inApp: true } | { readonly face: number; readonly face2?: number };

export type SearchSkill = 'perception' | 'investigation';

/**
 * Thin wrapper around the trap calls of `PlayService` (MR-035, traps.proto): the player's search, the
 * master's firing and the damage that waits. Callers map errors to Portuguese (`trapErrorMessage`).
 * `providedIn: 'root'`, imported only by lazy code.
 */
@Injectable({ providedIn: 'root' })
export class TrapsClient {
  private readonly client = createClient(PlayService, inject(CONNECT_TRANSPORT));

  /** "Procurar armadilhas". The key is made once per search and sent again on a retry. */
  search(campaignId: string, skill: SearchSkill, die: SearchDie, idempotencyKey: string): Promise<SearchForTrapsResponse> {
    return this.client.searchForTraps({
      campaignId,
      skill: skill === 'perception' ? TrapSearchSkill.PERCEPTION : TrapSearchSkill.INVESTIGATION,
      idempotencyKey,
      roll:
        'inApp' in die
          ? { case: 'rollInApp', value: true }
          : { case: 'd20Face', value: die.face },
      d20Face2: 'inApp' in die ? undefined : die.face2,
    });
  }

  /** `FireTrap`: empty `targetIds` is whoever stands in the area (the server decides). With
   * `extendFiringId` the creatures are added to a firing already made. */
  fire(
    campaignId: string,
    mapId: string,
    pointId: string,
    targetIds: readonly string[],
    idempotencyKey: string,
    extendFiringId = '',
  ): Promise<FireTrapResponse> {
    return this.client.fireTrap({ campaignId, mapId, pointId, targetIds: [...targetIds], idempotencyKey, extendFiringId });
  }

  /** The firings, searches and notices of the open session outside a combat, as the caller reads them. */
  activity(campaignId: string): Promise<ListTrapActivityResponse> {
    return this.client.listTrapActivity({ campaignId });
  }

  /** The master's damages that wait, in a combat or outside one. */
  damages(campaignId: string): Promise<ListTrapDamagesResponse> {
    return this.client.listTrapDamages({ campaignId });
  }

  /** Outside a combat. In a combat the damage is a pending one: `CombatClient.applyDamage`. */
  applyDamage(campaignId: string, id: string, amount: number | undefined, idempotencyKey: string): Promise<ApplyTrapDamageResponse> {
    return this.client.applyTrapDamage({ campaignId, trapDamageId: id, amount, idempotencyKey });
  }

  discardDamage(campaignId: string, id: string, idempotencyKey: string): Promise<DiscardTrapDamageResponse> {
    return this.client.discardTrapDamage({ campaignId, trapDamageId: id, idempotencyKey });
  }
}
