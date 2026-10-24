import { ChangeDetectionStrategy, Component, computed, inject, input, signal } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';
import { RouterLink } from '@angular/router';

import { PHONE_QUERY, mediaQuery } from '../../../shared/map-view/media-query';
import { CampaignRoster } from '../characters/campaign-roster';
import { MemberRowVm } from '../campaign-detail.copy';
import { PanelHead } from '../panel-head/panel-head';
import { PendingMembers } from '../pending-members/pending-members';

/**
 * "Membros" on the campaign page (MR-002; board C2-01): the people at the table, each with the
 * first letter of their name in a circle, the name and the role, and a "Pendente" mark on a
 * player whose character waits for the master's approval. The master's panel also lists who has
 * not made a character yet (`PendingMembers`, MR-024).
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-campaign-members',
  imports: [MatIconModule, PanelHead, PendingMembers, RouterLink],
  templateUrl: './campaign-members.html',
  styleUrl: './campaign-members.scss',
})
export class CampaignMembers {
  private readonly roster = inject(CampaignRoster, { optional: true });

  readonly campaignId = input.required<string>();
  readonly campaignName = input.required<string>();
  readonly isMaster = input.required<boolean>();
  readonly members = input.required<readonly MemberRowVm[]>();

  protected readonly count = computed(() => `${this.members().length} na mesa`);
  /** A phone folds the panel to one row ("Membros · 4 na mesa", with "1 pendente"); it opens in place. */
  private readonly phone = mediaQuery(PHONE_QUERY);
  /** Only the master's phone folds it: a player's page keeps its panels as they are. */
  protected readonly foldable = computed(() => this.phone() && this.isMaster());
  protected readonly open = signal(false);
  /** How many characters wait for the master, said on the folded row (the master only). */
  protected readonly waiting = computed(() =>
    this.isMaster() ? (this.roster?.awaiting().length ?? 0) : 0,
  );

  /** The players who own a character that waits for approval, by the name the table sees. */
  private readonly waitingPlayers = computed(
    () => new Set(this.roster?.awaiting().map((c) => c.playerDisplayName) ?? []),
  );

  /** A member with a character waiting for the master (the master sees it; a player never asks). */
  protected isWaiting(m: MemberRowVm): boolean {
    return this.isMaster() && m.hasName && this.waitingPlayers().has(m.name);
  }

  protected initial(m: MemberRowVm): string {
    return m.name.charAt(0).toUpperCase();
  }
}
