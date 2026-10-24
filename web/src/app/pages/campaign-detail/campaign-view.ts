import { Injectable, computed, signal } from '@angular/core';

import { Campaign, Role } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { MemberRowVm } from './campaign-detail.copy';

/** The campaign a master's tab draws, and the people at its table. */
export interface CampaignViewData {
  readonly campaign: Campaign;
  readonly members: readonly MemberRowVm[];
}

/**
 * What `CampaignDetail` has read (the campaign and its members), handed to the tab the router shows
 * under it ("Mesa", "Preparação", "Regras"): a tab is a route of its own and reads this instead of
 * asking again, so changing tab makes no call. Provided by `CampaignDetail`, which fills it.
 */
@Injectable()
export class CampaignView {
  /** `null` until the page has the campaign (and while it loads another one). */
  readonly data = signal<CampaignViewData | null>(null);

  readonly isMaster = computed(() => this.data()?.campaign.myRole === Role.MASTER);
}
