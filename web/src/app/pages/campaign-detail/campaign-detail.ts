import { Component, DestroyRef, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { Code, ConnectError } from '@connectrpc/connect';

import { Campaign, Member, Role } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  displayNameOrFallback,
  roleLabel,
  xpModeLabel,
} from '../../core/campaigns/campaign-labels';
import { CampaignsService } from '../../core/campaigns/campaigns.service';
import { describeConnectError } from '../../core/connect/connect-errors';
import { CampaignInvites } from './invites/invites';

type PageState =
  | { status: 'loading' }
  | { status: 'not-found' }
  | { status: 'error'; message: string }
  | { status: 'ready'; campaign: Campaign; members: Member[] };

/**
 * "/campanhas/:id" (guarded by authGuard): GetCampaign + ListMembers
 * (MR-001, MR-002), plus the master's "Convites" section.
 *
 * A campaign the caller is not a member of, and one that does not exist,
 * both come back as `not_found` (ADR-0011) — this page shows the same
 * "campanha não encontrada" message for both, never telling the two apart.
 */
@Component({
  selector: 'app-campaign-detail',
  imports: [CampaignInvites, MatButtonModule, MatCardModule, MatProgressSpinnerModule, RouterLink],
  templateUrl: './campaign-detail.html',
  styleUrl: './campaign-detail.scss',
})
export class CampaignDetail {
  private readonly campaigns = inject(CampaignsService);
  private readonly route = inject(ActivatedRoute);
  private readonly destroyRef = inject(DestroyRef);

  protected readonly state = signal<PageState>({ status: 'loading' });
  protected readonly Role = Role;
  protected readonly roleLabel = roleLabel;
  protected readonly xpModeLabel = xpModeLabel;
  protected readonly displayNameOrFallback = displayNameOrFallback;

  constructor() {
    this.route.paramMap.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((params) => {
      const id = params.get('id');
      if (id) {
        this.load(id);
      }
    });
  }

  private load(campaignId: string): void {
    this.state.set({ status: 'loading' });
    Promise.all([
      this.campaigns.getCampaign(campaignId),
      this.campaigns.listMembers(campaignId),
    ]).then(
      ([campaignRes, membersRes]) => {
        const campaign = campaignRes.campaign;
        if (!campaign) {
          this.state.set({ status: 'not-found' });
          return;
        }
        this.state.set({ status: 'ready', campaign, members: membersRes.members });
      },
      (err: unknown) => {
        const connectErr = ConnectError.from(err, Code.Unavailable);
        if (connectErr.code === Code.NotFound) {
          // Same message for "does not exist" and "not a member": ADR-0011.
          this.state.set({ status: 'not-found' });
          return;
        }
        this.state.set({ status: 'error', message: describeConnectError(connectErr, {}) });
      },
    );
  }
}
