import { Injectable, inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';

import type { Encounter } from '../../../gen/meurpg/play/v1/combat_pb';
import type {
  CharacterEffect,
  EffectSaveResult,
} from '../../../gen/meurpg/play/v1/lasting_effects_pb';
import { LastingEffectService } from '../../../gen/meurpg/play/v1/lasting_effects_service_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';

/** How a player answers the saving throw an effect asks: the d20 in the app, the faces of physical dice, or the master. */
export type EffectSaveAnswer =
  | { readonly kind: 'app' }
  | {
      readonly kind: 'typed';
      readonly face: number;
      readonly second?: number;
      readonly extra: readonly number[];
    }
  | { readonly kind: 'hand' };

/** What `RollEffectSave` answers: the combat and the result (unset when the roll was left to the master). */
export interface EffectSaveOutcome {
  readonly encounter: Encounter;
  readonly result: EffectSaveResult | undefined;
}

/**
 * The calls of `LastingEffectService` a player makes (RN-22): the saving throw an effect asks at a turn, and the
 * effects on their characters outside a combat. The master's calls (adding, changing, ending) are another
 * client's. Every write carries the idempotency key its screen made once.
 */
@Injectable({ providedIn: 'root' })
export class EffectsClient {
  private readonly client = createClient(LastingEffectService, inject(CONNECT_TRANSPORT));

  /** `RollEffectSave`: the window's saving throw, in the app, with physical dice or left to the master. */
  async rollEffectSave(
    campaignId: string,
    encounterId: string,
    windowId: string,
    how: EffectSaveAnswer,
    key: string,
  ): Promise<EffectSaveOutcome> {
    const typed = how.kind === 'typed' ? how : null;
    const res = await this.client.rollEffectSave({
      campaignId,
      encounterId,
      windowId,
      idempotencyKey: key,
      roll:
        how.kind === 'app'
          ? { case: 'rollInApp', value: true }
          : how.kind === 'typed'
            ? { case: 'd20Face', value: how.face }
            : { case: 'delegateToMaster', value: true },
      extraDieFaces: typed ? [...typed.extra] : [],
      ...(typed?.second !== undefined ? { secondD20Face: typed.second } : {}),
    });
    if (!res.encounter) {
      throw new Error('RollEffectSave answered without the combat');
    }
    return { encounter: res.encounter, result: res.result };
  }

  /** `ListCharacterEffects`: what the caller may read of the effects on the characters, outside a running combat. */
  async listCharacterEffects(campaignId: string): Promise<readonly CharacterEffect[]> {
    const res = await this.client.listCharacterEffects({ campaignId });
    return res.effects;
  }
}
