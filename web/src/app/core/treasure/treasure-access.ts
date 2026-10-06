import { Injectable, inject } from '@angular/core';
import { Code, ConnectError } from '@connectrpc/connect';

import { Role, XpMode } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { CampaignsService } from '../campaigns/campaigns.service';
import { describeConnectError } from '../connect/connect-errors';

/** Whether the person may use "Tesouro" in a campaign, and what the page needs to know of the campaign. */
export type TreasureAccess =
  | { readonly status: 'master'; readonly campaignName: string; readonly xpMode: XpMode }
  /** A player (or a member waiting for approval): the page and every call are the master's. */
  | { readonly status: 'forbidden' }
  | { readonly status: 'not-found' }
  | { readonly status: 'error'; readonly message: string };

/**
 * Who may open "Tesouro" (MR-044). The server answers `not_found` to anyone but the master, but the
 * page asks for the campaign's role first so a player gets a sentence instead of an error, and reads
 * the campaign's name and XP mode (RN-09) for the line that says what happens to the gold (state 5).
 */
@Injectable({ providedIn: 'root' })
export class TreasureAccessCheck {
  private readonly campaigns = inject(CampaignsService);

  async check(campaignId: string): Promise<TreasureAccess> {
    try {
      const { campaign } = await this.campaigns.getCampaign(campaignId);
      if (!campaign) {
        return { status: 'not-found' };
      }
      return campaign.myRole === Role.MASTER && !campaign.awaitingApproval
        ? { status: 'master', campaignName: campaign.name, xpMode: campaign.xpMode }
        : { status: 'forbidden' };
    } catch (err) {
      return ConnectError.from(err, Code.Unavailable).code === Code.NotFound
        ? { status: 'not-found' }
        : {
            status: 'error',
            message: describeConnectError(err, { [Code.Unavailable]: 'Não deu para abrir o tesouro: o servidor não respondeu. Tente de novo.' }),
          };
    }
  }
}
