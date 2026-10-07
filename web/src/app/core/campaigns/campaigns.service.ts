import { Injectable, inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';
import { durationFromMs } from '@bufbuild/protobuf/wkt';

import {
  CampaignService,
  DiceMode,
  DicePreference,
  XpMode,
} from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';

const dayMs = 24 * 60 * 60 * 1000;

/**
 * Thin wrapper around the generated CampaignService Connect client, in the
 * same one-service-per-module shape as `ServerInfoService` and
 * `AuthService`'s `IdentityService` client. Every method here needs a
 * session cookie; callers behind `authGuard` (the campaigns pages) already
 * have one, so this class does not itself deal with `unauthenticated` —
 * each caller maps the Connect codes it cares about with
 * `describeConnectError`.
 */
@Injectable({ providedIn: 'root' })
export class CampaignsService {
  private readonly client = createClient(CampaignService, inject(CONNECT_TRANSPORT));

  listMyCampaigns() {
    return this.client.listMyCampaigns({});
  }

  /** `idempotencyKey`: one per create action, sent again on a retry (see `ActionKey`). */
  createCampaign(name: string, xpMode: XpMode, idempotencyKey: string) {
    return this.client.createCampaign({ name, xpMode, idempotencyKey });
  }

  getCampaign(campaignId: string) {
    return this.client.getCampaign({ campaignId });
  }

  listMembers(campaignId: string) {
    return this.client.listMembers({ campaignId });
  }

  /** Pending members who have not created a character yet (RN-15, MR-024); master only. */
  listPendingMembers(campaignId: string) {
    return this.client.listPendingMembers({ campaignId });
  }

  /** Removes one of them; `failed_precondition` if they created a character meanwhile. */
  removePendingMember(campaignId: string, userId: string) {
    return this.client.removePendingMember({ campaignId, userId });
  }

  /** `validityDays` follows CreateInviteRequest.expires_in: 5 minutes to 30
   * days; the app only ever offers the 1/7/30-day presets (MR-002).
   * `requiresApproval` makes whoever accepts it a pending member, whose
   * character the master approves first (RN-15, MR-024). */
  createInvite(
    campaignId: string,
    maxUses: number,
    validityDays: number,
    requiresApproval = false,
  ) {
    return this.client.createInvite({
      campaignId,
      maxUses,
      expiresIn: durationFromMs(validityDays * dayMs),
      requiresApproval,
    });
  }

  listInvites(campaignId: string) {
    return this.client.listInvites({ campaignId });
  }

  revokeInvite(campaignId: string, inviteId: string) {
    return this.client.revokeInvite({ campaignId, inviteId });
  }

  acceptInvite(token: string) {
    return this.client.acceptInvite({ token });
  }

  /** Master only (RN-18): how the campaign's players roll dice. */
  setDiceMode(campaignId: string, mode: DiceMode) {
    return this.client.setCampaignDiceMode({ campaignId, mode });
  }

  /** Any member (RN-18): the caller's own choice, used while the campaign
   * lets players choose. */
  setDicePreference(campaignId: string, preference: DicePreference) {
    return this.client.setMyDicePreference({ campaignId, preference });
  }
}
