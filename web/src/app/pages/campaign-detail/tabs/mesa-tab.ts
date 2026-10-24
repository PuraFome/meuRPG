import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { NgTemplateOutlet } from '@angular/common';

import { PHONE_QUERY, mediaQuery } from '../../../shared/map-view/media-query';
import { CampaignCharacters } from '../characters/campaign-characters';
import { CampaignInvites } from '../invites/invites';
import { CampaignMembers } from '../members/campaign-members';
import { CampaignView } from '../campaign-view';
import { ExperiencePanel } from '../experience/experience-panel';
import { PastSessionsPanel } from '../past-sessions/past-sessions-panel';

/**
 * The master's "Mesa" tab, `/campaigns/:id` (the default): what runs the table. On the left the
 * players' characters (the ones waiting for approval first), "Experiência" and "Sessões
 * anteriores"; on the right "Membros" and "Convites" (boards C2-01 and C2-02).
 *
 * On a phone (board C6-04) the panels come in one column and the three that fold to a row come
 * last, in the order the board draws them: "Membros", "Convites", "Sessões anteriores". The
 * order in the page is the order on the screen, so the keyboard follows what the eye sees.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-mesa-tab',
  imports: [
    CampaignCharacters,
    CampaignInvites,
    CampaignMembers,
    ExperiencePanel,
    NgTemplateOutlet,
    PastSessionsPanel,
  ],
  template: `
    @if (view.data(); as d) {
      <ng-template #players>
        <app-campaign-characters [campaignId]="d.campaign.id" [isMaster]="true" />
      </ng-template>
      <ng-template #experience>
        <app-experience-panel
          [campaignId]="d.campaign.id"
          [campaignName]="d.campaign.name"
          [isMaster]="true"
        />
      </ng-template>
      <ng-template #past>
        <app-past-sessions-panel [campaignId]="d.campaign.id" [isMaster]="true" />
      </ng-template>
      <ng-template #members>
        <app-campaign-members
          [campaignId]="d.campaign.id"
          [campaignName]="d.campaign.name"
          [isMaster]="true"
          [members]="d.members"
        />
      </ng-template>
      <ng-template #invites>
        <app-campaign-invites [campaignId]="d.campaign.id" />
      </ng-template>

      @if (phone()) {
        <div class="tab-layout">
          <div class="tab-layout__column">
            <ng-container [ngTemplateOutlet]="players" />
            <ng-container [ngTemplateOutlet]="experience" />
            <ng-container [ngTemplateOutlet]="members" />
            <ng-container [ngTemplateOutlet]="invites" />
            <ng-container [ngTemplateOutlet]="past" />
          </div>
        </div>
      } @else {
        <div class="tab-layout">
          <div class="tab-layout__column">
            <ng-container [ngTemplateOutlet]="players" />
            <ng-container [ngTemplateOutlet]="experience" />
            <ng-container [ngTemplateOutlet]="past" />
          </div>
          <div class="tab-layout__column">
            <ng-container [ngTemplateOutlet]="members" />
            <ng-container [ngTemplateOutlet]="invites" />
          </div>
        </div>
      }
    }
  `,
  styleUrl: './mesa-tab.scss',
})
export class MesaTab {
  protected readonly view = inject(CampaignView);
  protected readonly phone = mediaQuery(PHONE_QUERY);
}
