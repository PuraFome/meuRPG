import { ChangeDetectionStrategy, Component, inject } from '@angular/core';

import { CampaignCharacters } from '../characters/campaign-characters';
import { CampaignInvites } from '../invites/invites';
import { CampaignMembers } from '../members/campaign-members';
import { CampaignView } from '../campaign-view';
import { ExperiencePanel } from '../experience/experience-panel';
import { PastSessionsPanel } from '../past-sessions/past-sessions-panel';

/**
 * The master's "Mesa" tab, `/campaigns/:id` (the default): what runs the table. On the left the
 * players' characters (the ones waiting for approval first), "Experiência" and "Sessões
 * anteriores"; on the right "Membros" and "Convites".
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-mesa-tab',
  imports: [
    CampaignCharacters,
    CampaignInvites,
    CampaignMembers,
    ExperiencePanel,
    PastSessionsPanel,
  ],
  template: `
    @if (view.data(); as d) {
      <div class="tab-layout">
        <div class="tab-layout__column">
          <app-campaign-characters [campaignId]="d.campaign.id" [isMaster]="true" />
          <app-experience-panel
            [campaignId]="d.campaign.id"
            [campaignName]="d.campaign.name"
            [isMaster]="true"
          />
          <app-past-sessions-panel [campaignId]="d.campaign.id" [isMaster]="true" />
        </div>
        <div class="tab-layout__column">
          <app-campaign-members
            [campaignId]="d.campaign.id"
            [campaignName]="d.campaign.name"
            [isMaster]="true"
            [members]="d.members"
          />
          <app-campaign-invites [campaignId]="d.campaign.id" />
        </div>
      </div>
    }
  `,
  styleUrl: './mesa-tab.scss',
})
export class MesaTab {
  protected readonly view = inject(CampaignView);
}
