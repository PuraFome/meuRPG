import { Injectable, inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';

import type { CharacterVitals } from '../../../gen/meurpg/play/v1/play_pb';
import {
  type ConvertSpellSlotResponse,
  type CreateSpellSlotResponse,
  type GiveBardicInspirationResponse,
  LayOnHandsCure,
  type UseLayOnHandsResponse,
  type GetRestPreviewResponse,
  type RestKind,
  ResourceService,
  type SpendHitDiceResponse,
} from '../../../gen/meurpg/play/v1/resources_pb';
import { type AttackResult, attackResult } from '../combat/combat-client';
import { CONNECT_TRANSPORT } from '../connect/transport';

/** How the die of a spent hit die comes (RN-18): the app rolls it, or the player typed the face of a real die. */
export type HitDieRoll = { readonly inApp: true } | { readonly face: number };

/** What a Lay on Hands touch does: restore `amount` hit points (1 to what the pool has left), or cure one disease or
 * neutralize one poison (5 points). */
export type LayOnHandsEffect =
  { readonly amount: number } | { readonly cure: 'disease' | 'poison' };

/** How the die of a Bardic Inspiration comes when the player uses it: the app rolls it, or the typed face. */
export type InspirationRoll = HitDieRoll;

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
    withoutFoodOrDrink = false,
  ): Promise<CharacterVitals[]> {
    const res = await this.client.takeRest({
      campaignId,
      kind,
      idempotencyKey,
      withoutFoodOrDrink,
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

  /** Lay on Hands: the touch, as an action of the paladin's turn. The answer never says why a touch did nothing. */
  useLayOnHands(
    campaignId: string,
    encounterId: string,
    actorId: string,
    targetId: string,
    effect: LayOnHandsEffect,
    idempotencyKey: string,
  ): Promise<UseLayOnHandsResponse> {
    return this.client.useLayOnHands({
      campaignId,
      encounterId,
      actorId,
      targetId,
      idempotencyKey,
      effect:
        'amount' in effect
          ? { case: 'amount', value: effect.amount }
          : {
              case: 'cure',
              value: effect.cure === 'poison' ? LayOnHandsCure.POISON : LayOnHandsCure.DISEASE,
            },
    });
  }

  /** Flexible Casting, points to slot: a slot of level 1 to 5 as a bonus action. */
  createSpellSlot(
    campaignId: string,
    encounterId: string,
    actorId: string,
    slotLevel: number,
    idempotencyKey: string,
  ): Promise<CreateSpellSlotResponse> {
    return this.client.createSpellSlot({
      campaignId,
      encounterId,
      actorId,
      slotLevel,
      idempotencyKey,
    });
  }

  /** Flexible Casting, slot to points: a free slot is expended for as many points as its level. */
  convertSpellSlot(
    campaignId: string,
    encounterId: string,
    actorId: string,
    slotLevel: number,
    idempotencyKey: string,
  ): Promise<ConvertSpellSlotResponse> {
    return this.client.convertSpellSlot({
      campaignId,
      encounterId,
      actorId,
      slotLevel,
      idempotencyKey,
    });
  }

  /** Bardic Inspiration: the die to a creature, as a bonus action. */
  giveBardicInspiration(
    campaignId: string,
    encounterId: string,
    actorId: string,
    targetId: string,
    idempotencyKey: string,
  ): Promise<GiveBardicInspirationResponse> {
    return this.client.giveBardicInspiration({
      campaignId,
      encounterId,
      actorId,
      targetId,
      idempotencyKey,
    });
  }

  /** The answer to a held attack roll: use the die (rolled in the app, or its typed face) or keep it. Resolves the
   * attack as `RollAttack` would have. `roll` is only for `use`. */
  async answerBardicInspiration(
    campaignId: string,
    encounterId: string,
    holdId: string,
    use: boolean,
    roll: InspirationRoll | null,
    idempotencyKey: string,
  ): Promise<AttackResult> {
    const res = await this.client.answerBardicInspiration({
      campaignId,
      encounterId,
      holdId,
      use,
      idempotencyKey,
      roll:
        use && roll
          ? 'inApp' in roll
            ? { case: 'rollInApp', value: true }
            : { case: 'typedFace', value: roll.face }
          : { case: undefined },
    });
    if (!res.attack) {
      throw new Error('AnswerBardicInspiration answered without its attack');
    }
    return attackResult(res.attack);
  }
}
