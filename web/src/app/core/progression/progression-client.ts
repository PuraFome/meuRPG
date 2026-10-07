import { Injectable, inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';

import {
  type AddMilestoneResponse,
  type AwardXPResponse,
  type GetCampaignExperienceResponse,
  type GiveMilestoneToResponse,
  type ListMilestonesResponse,
  type ListTreasuresToConvertResponse,
  type ListXPAwardsResponse,
  type MarkMilestoneReachedResponse,
  type MarkMilestoneResponse,
  MilestoneDirection,
  type MoveMilestoneResponse,
  ProgressionService,
  type RemoveMilestoneResponse,
  type UpdateMilestoneResponse,
  XPAwardMode,
  type UndoLastXPAwardResponse,
} from '../../../gen/meurpg/progression/v1/progression_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';

/** What a master gives (`AwardXPRequest`): one of the three XP modes. The
 * mode decides which of the other fields the server reads. */
export type AwardInput =
  | { readonly mode: 'enemies'; readonly encounterId: string }
  | { readonly mode: 'manual'; readonly amount: number }
  | { readonly mode: 'gold'; readonly gold: number }
  /** "Voltar à cidade": the found treasures to convert; the server sums their PO. */
  | { readonly mode: 'town'; readonly treasurePointIds: readonly string[] };

/** The awards a page asks for at once; "Mostrar mais" asks for the next page. */
export const AWARDS_PAGE_SIZE = 20;

/**
 * Thin wrapper around the generated `ProgressionService` client (MR-016), in
 * the shape of `CombatClient`. `providedIn: 'root'`, and only lazy code
 * imports it, so the generated code stays out of the initial bundle. Every
 * write takes the idempotency key the caller made once for the action, so a
 * retry after a lost answer never gives XP twice. Callers map errors to
 * Portuguese with `xp-errors.ts`.
 */
@Injectable({ providedIn: 'root' })
export class ProgressionClient {
  private readonly client = createClient(ProgressionService, inject(CONNECT_TRANSPORT));

  award(
    campaignId: string,
    input: AwardInput,
    reason: string,
    characterIds: readonly string[],
    idempotencyKey: string,
  ): Promise<AwardXPResponse> {
    return this.client.awardXP({
      campaignId,
      reason,
      characterIds: [...characterIds],
      idempotencyKey,
      mode:
        input.mode === 'enemies'
          ? XPAwardMode.XP_AWARD_MODE_ENEMIES
          : input.mode === 'gold' || input.mode === 'town'
            ? XPAwardMode.XP_AWARD_MODE_GOLD
            : XPAwardMode.XP_AWARD_MODE_MANUAL,
      encounterId: input.mode === 'enemies' ? input.encounterId : '',
      amount: input.mode === 'manual' ? input.amount : 0,
      gold: input.mode === 'gold' ? input.gold : 0,
      treasurePointIds: input.mode === 'town' ? [...input.treasurePointIds] : [],
    });
  }

  /** The found treasures waiting to be converted (master only): at most 100,
   * the oldest finds, and `total` says how many there are. */
  listTreasures(campaignId: string): Promise<ListTreasuresToConvertResponse> {
    return this.client.listTreasuresToConvert({ campaignId });
  }

  markMilestone(
    campaignId: string,
    reason: string,
    characterIds: readonly string[],
    idempotencyKey: string,
  ): Promise<MarkMilestoneResponse> {
    return this.client.markMilestone({
      campaignId,
      reason,
      characterIds: [...characterIds],
      idempotencyKey,
    });
  }

  /** `expectedAwardId` is the award the screen shows as the last one: when
   * another is the last now, the server answers `aborted` and changes nothing. */
  undoLast(
    campaignId: string,
    expectedAwardId: string,
    idempotencyKey: string,
  ): Promise<UndoLastXPAwardResponse> {
    return this.client.undoLastXPAward({ campaignId, expectedAwardId, idempotencyKey });
  }

  listAwards(campaignId: string, pageToken = ''): Promise<ListXPAwardsResponse> {
    return this.client.listXPAwards({ campaignId, pageSize: AWARDS_PAGE_SIZE, pageToken });
  }

  /** The campaign's milestones: every one for the master, the reached ones
   * for a player (`ListMilestones`). */
  listMilestones(campaignId: string): Promise<ListMilestonesResponse> {
    return this.client.listMilestones({ campaignId });
  }

  addMilestone(
    campaignId: string,
    text: string,
    idempotencyKey: string,
  ): Promise<AddMilestoneResponse> {
    return this.client.addMilestone({ campaignId, text, idempotencyKey });
  }

  updateMilestone(
    campaignId: string,
    milestoneId: string,
    text: string,
  ): Promise<UpdateMilestoneResponse> {
    return this.client.updateMilestone({ campaignId, milestoneId, text });
  }

  moveMilestone(
    campaignId: string,
    milestoneId: string,
    direction: 'up' | 'down',
  ): Promise<MoveMilestoneResponse> {
    return this.client.moveMilestone({
      campaignId,
      milestoneId,
      direction: direction === 'up' ? MilestoneDirection.UP : MilestoneDirection.DOWN,
    });
  }

  removeMilestone(campaignId: string, milestoneId: string): Promise<RemoveMilestoneResponse> {
    return this.client.removeMilestone({ campaignId, milestoneId });
  }

  /** "Marcar como alcançado": the characters in `characterIds` can level up. */
  markMilestoneReached(
    campaignId: string,
    milestoneId: string,
    characterIds: readonly string[],
    idempotencyKey: string,
  ): Promise<MarkMilestoneReachedResponse> {
    return this.client.markMilestoneReached({
      campaignId,
      milestoneId,
      characterIds: [...characterIds],
      idempotencyKey,
    });
  }

  /** "Dar a mais alguém": the same reached milestone, for others. */
  giveMilestoneTo(
    campaignId: string,
    milestoneId: string,
    characterIds: readonly string[],
    idempotencyKey: string,
  ): Promise<GiveMilestoneToResponse> {
    return this.client.giveMilestoneTo({
      campaignId,
      milestoneId,
      characterIds: [...characterIds],
      idempotencyKey,
    });
  }

  experience(campaignId: string): Promise<GetCampaignExperienceResponse> {
    return this.client.getCampaignExperience({ campaignId });
  }
}
