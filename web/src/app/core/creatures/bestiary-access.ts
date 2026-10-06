import { Injectable, inject } from '@angular/core';
import { Code, ConnectError } from '@connectrpc/connect';

import { Role } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { CampaignsService } from '../campaigns/campaigns.service';
import { bestiaryErrorMessage } from './bestiary-errors';

/** Whether the person may use the bestiary of a campaign, and what to say when not. */
export type BestiaryAccess =
  | { readonly status: 'master'; readonly campaignName: string }
  /** A player (or a member waiting for approval): the SRD is public, but the app shows the bestiary only to the master. */
  | { readonly status: 'forbidden' }
  | { readonly status: 'not-found' }
  | { readonly status: 'error'; readonly message: string };

/**
 * Who may open "Bestiário" (MR-042). The server lets any active member read the SRD's creatures
 * (it is public), so the app asks for the campaign's role itself: only the master gets the page,
 * and the campaign page shows its entry point to the master alone.
 */
@Injectable({ providedIn: 'root' })
export class BestiaryAccessCheck {
  private readonly campaigns = inject(CampaignsService);

  async check(campaignId: string): Promise<BestiaryAccess> {
    try {
      const { campaign } = await this.campaigns.getCampaign(campaignId);
      if (!campaign) {
        return { status: 'not-found' };
      }
      return campaign.myRole === Role.MASTER && !campaign.awaitingApproval ? { status: 'master', campaignName: campaign.name } : { status: 'forbidden' };
    } catch (err) {
      return ConnectError.from(err, Code.Unavailable).code === Code.NotFound
        ? { status: 'not-found' }
        : { status: 'error', message: bestiaryErrorMessage(err, 'list') };
    }
  }
}
