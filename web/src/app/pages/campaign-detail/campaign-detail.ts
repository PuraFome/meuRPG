import { Component, DestroyRef, computed, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { Code, ConnectError } from '@connectrpc/connect';

import { Campaign, Member, Role } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { AuthService } from '../../core/auth/auth.service';
import { CampaignsService } from '../../core/campaigns/campaigns.service';
import { describeConnectError } from '../../core/connect/connect-errors';
import { campaignLead } from '../campaigns/campaign-copy';
import { memberRows } from './campaign-detail.copy';
import { CampaignCharacters } from './characters/campaign-characters';
import { DocumentPanel } from './document-panel/document-panel';
import { GalleryPanel } from './gallery-panel/gallery-panel';
import { GameSessionCard } from './game-session/game-session-card';
import { CampaignInvites } from './invites/invites';

type PageState =
  | { status: 'loading' }
  | { status: 'not-found' }
  | { status: 'error'; message: string }
  | { status: 'ready'; campaign: Campaign; members: Member[] }
  // A pending member (RN-15, MR-024): the server sent only the campaign's
  // name. They see the wait banner and their own character, nothing else.
  | { status: 'pending'; campaign: Campaign };

/**
 * "/campanhas/:id" (guarded by authGuard): GetCampaign + ListMembers
 * (MR-001, MR-002), the "Personagens" section (MR-003, MR-005; everyone),
 * "Sessão" (MR-006 / RN-01 for the master; for a player, only while a
 * session is open, with "Entrar na sessão", RN-06) and the master-only
 * "Convites".
 *
 * From 1024px up the page has two columns: the play on the left (Sessão,
 * then the characters and NPCs) and the table on the right (Membros,
 * Convites). Below that, the same order in one column.
 *
 * A pending member (an invite with approval, RN-15 / MR-024) gets only the
 * campaign's name from GetCampaign (`awaitingApproval`): the page shows
 * "Esperando a aprovação do mestre" and their own character, and never
 * asks for the members, which the server would refuse them.
 *
 * A campaign the caller is not a member of, and one that does not exist,
 * both come back as `not_found` (ADR-0011) — this page shows the same
 * "campanha não encontrada" message for both, never telling the two apart.
 */
@Component({
  selector: 'app-campaign-detail',
  imports: [
    CampaignCharacters,
    CampaignInvites,
    DocumentPanel,
    GalleryPanel,
    GameSessionCard,
    MatIconModule,
    MatProgressSpinnerModule,
    RouterLink,
  ],
  templateUrl: './campaign-detail.html',
  styleUrl: './campaign-detail.scss',
})
export class CampaignDetail {
  private readonly campaigns = inject(CampaignsService);
  private readonly route = inject(ActivatedRoute);
  private readonly destroyRef = inject(DestroyRef);
  private readonly auth = inject(AuthService);

  protected readonly state = signal<PageState>({ status: 'loading' });
  protected readonly Role = Role;
  protected readonly campaignLead = campaignLead;

  /** The signed-in person's id, to mark their own row in "Membros" (and
   * point them to "Meu perfil" when they have no display name yet). */
  private readonly viewerId = computed(() => {
    const auth = this.auth.state();
    return auth.status === 'signed-in' ? auth.user.id : null;
  });

  protected readonly members = computed(() => {
    const s = this.state();
    return s.status === 'ready' ? memberRows(s.members, this.viewerId()) : [];
  });

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
    // One after the other, not in parallel: whether to ask for the members
    // at all depends on the campaign (a pending member may not list them).
    const loaded = this.campaigns.getCampaign(campaignId).then(async (campaignRes) => {
      const campaign = campaignRes.campaign;
      if (!campaign) {
        return { status: 'not-found' } as const;
      }
      if (campaign.awaitingApproval) {
        return { status: 'pending', campaign } as const;
      }
      const membersRes = await this.campaigns.listMembers(campaignId);
      return { status: 'ready', campaign, members: membersRes.members } as const;
    });
    loaded.then(
      (state: PageState) => this.state.set(state),
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
