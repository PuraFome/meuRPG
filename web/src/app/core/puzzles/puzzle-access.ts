import { Injectable, inject } from '@angular/core';
import { Code, ConnectError } from '@connectrpc/connect';

import { Role } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { CampaignsService } from '../campaigns/campaigns.service';
import { describeConnectError } from '../connect/connect-errors';

/** Whether the person may make puzzles in a campaign, and what to say when not. */
export type PuzzleAccess =
  | { readonly status: 'master'; readonly campaignName: string }
  /** A player (or a member waiting for approval): the puzzles' pages are the master's. */
  | { readonly status: 'forbidden' }
  | { readonly status: 'not-found' }
  | { readonly status: 'error'; readonly message: string };

/**
 * Who may open the master's puzzle pages (MR-038). The server refuses a player every master call (`permission_denied`), but the
 * pages ask for the campaign's role first, so a player who opens the address reads "Só o mestre faz quebra-cabeças" instead of
 * an error.
 */
@Injectable({ providedIn: 'root' })
export class PuzzleAccessCheck {
  private readonly campaigns = inject(CampaignsService);

  async check(campaignId: string): Promise<PuzzleAccess> {
    try {
      const { campaign } = await this.campaigns.getCampaign(campaignId);
      if (!campaign) {
        return { status: 'not-found' };
      }
      return campaign.myRole === Role.MASTER && !campaign.awaitingApproval ? { status: 'master', campaignName: campaign.name } : { status: 'forbidden' };
    } catch (err) {
      return ConnectError.from(err, Code.Unavailable).code === Code.NotFound
        ? { status: 'not-found' }
        : {
            status: 'error',
            message: describeConnectError(err, { [Code.Unavailable]: 'Não deu para abrir a campanha: o servidor não respondeu. Tente de novo.' }),
          };
    }
  }
}
