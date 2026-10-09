import { Injectable, inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';

import type { CharacterVitals } from '../../../gen/meurpg/play/v1/play_pb';
import {
  type GetRestPreviewResponse,
  type RestKind,
  ResourceService,
  type SpendHitDiceResponse,
} from '../../../gen/meurpg/play/v1/resources_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';

/** How the die of a spent hit die comes (RN-18): the app rolls it, or the player typed the face of a real die. */
export type HitDieRoll = { readonly inApp: true } | { readonly face: number };

/** How many dice of each size come back to one character on a long rest (`HitDiceChoice`). */
export interface HitDiceChoiceSpec {
  readonly characterId: string;
  readonly dice: readonly { readonly faces: number; readonly count: number }[];
}

/**
 * Thin wrapper around the generated `ResourceService` client (the rests and the hit dice), in the shape of
 * `CombatClient`. Only lazy code imports it, so the generated code stays out of the initial bundle. Every write takes
 * the idempotency key its caller made once for that request (`ActionKey`); callers word the errors with
 * `resources-errors.ts`.
 */
@Injectable({ providedIn: 'root' })
export class ResourceClient {
  private readonly client = createClient(ResourceService, inject(CONNECT_TRANSPORT));

  /** What a rest would give back to each living player character; nothing is written. Master only. */
  restPreview(campaignId: string, kind: RestKind): Promise<GetRestPreviewResponse> {
    return this.client.getRestPreview({ campaignId, kind });
  }

  /** The master's rest. Answers the new vitals of every character it touched. */
  async takeRest(
    campaignId: string,
    kind: RestKind,
    idempotencyKey: string,
    hitDiceChoices: readonly HitDiceChoiceSpec[] = [],
  ): Promise<CharacterVitals[]> {
    const res = await this.client.takeRest({
      campaignId,
      kind,
      idempotencyKey,
      hitDiceChoices: hitDiceChoices.map((c) => ({
        characterId: c.characterId,
        dice: c.dice.map((d) => ({ faces: d.faces, count: d.count })),
      })),
    });
    return res.vitals;
  }

  /** One hit die spent on a short rest: the face, the Constitution modifier and the hit points regained. */
  spendHitDice(
    campaignId: string,
    characterId: string,
    faces: number,
    roll: HitDieRoll,
    idempotencyKey: string,
  ): Promise<SpendHitDiceResponse> {
    return this.client.spendHitDice({
      campaignId,
      characterId,
      faces,
      idempotencyKey,
      roll:
        'inApp' in roll
          ? { case: 'rollInApp', value: true }
          : { case: 'typedFace', value: roll.face },
    });
  }
}
