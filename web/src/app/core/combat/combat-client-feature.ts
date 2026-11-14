import { inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';

import type { Encounter } from '../../../gen/meurpg/play/v1/combat_pb';
import { FeatureService } from '../../../gen/meurpg/play/v1/features_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';
import { need } from './combat-client-wire';

/** What one use of a feature did: the combat as it is now, and what it healed and gave (the master's and the user's player's; 0 for the others). */
export interface FeatureUseResult {
  readonly encounter: Encounter;
  readonly healed: number;
  readonly tempHp: number;
}

/** What the player picked to send with `UseFeature`. */
export interface FeatureUseArgs {
  readonly campaignId: string;
  readonly encounterId: string;
  readonly combatantId: string;
  /** The key of the feature's action ("feature:<key>"). */
  readonly actionKey: string;
  /** The creatures the use is aimed at; none takes the user. */
  readonly targetIds?: readonly string[];
  /** One of the keys the feature offers; none when it offers none. */
  readonly choice?: string;
}

/** The call that uses a feature of a combatant (`FeatureService.UseFeature`): a table feature whose action declares a `use`. */
export class CombatFeatureCalls {
  readonly client = createClient(FeatureService, inject(CONNECT_TRANSPORT));

  /** `UseFeature`, under the caller's idempotency key: a second tap or a try after a lost answer sends the same key. */
  async useFeature(args: FeatureUseArgs, key: string): Promise<FeatureUseResult> {
    const res = await this.client.useFeature({
      campaignId: args.campaignId,
      encounterId: args.encounterId,
      combatantId: args.combatantId,
      actionKey: args.actionKey,
      targetIds: [...(args.targetIds ?? [])],
      choice: args.choice ?? '',
      idempotencyKey: key,
    });
    return {
      encounter: need(res.encounter, 'UseFeature'),
      healed: res.healed,
      tempHp: res.tempHp,
    };
  }
}
