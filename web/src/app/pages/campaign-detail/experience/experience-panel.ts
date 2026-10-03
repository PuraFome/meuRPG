import { Component, computed, inject, input, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import { XpMode } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { ExperienceStore } from '../../../core/progression/experience-store';
import { experienceLead, givenText, milestoneText } from '../../../core/progression/xp-labels';
import { type GiveResult, XpGiveButton } from '../../../shared/xp/xp-give-button';
import { AwardHistory } from './award-history';
import { XpRows } from './xp-rows';

/**
 * "Experiência" on the campaign page (MR-016, E7-09, E7-08): every member
 * sees each living player character's XP and what is missing for the next
 * level, then the history of awards, newest first. The master also has
 * "Dar XP" (or "Registrar marco" in a campaign that levels by milestones),
 * outlined: the page's one filled button stays "Entrar na sessão". The panel
 * never goes away: with no award yet it shows 0 of the next level and
 * invites the first one. The page loads the `ExperienceStore`; this reads it.
 */
@Component({
  selector: 'app-experience-panel',
  imports: [AwardHistory, MatButtonModule, MatIconModule, XpGiveButton, XpRows],
  templateUrl: './experience-panel.html',
  styleUrl: './experience-panel.scss',
})
export class ExperiencePanel {
  protected readonly store = inject(ExperienceStore);

  readonly campaignId = input.required<string>();
  readonly campaignName = input('');
  readonly isMaster = input(false);

  /** What the master just gave, said once in a polite status. */
  protected readonly confirmation = signal('');

  protected readonly milestones = computed(() => this.store.xpMode() === XpMode.MILESTONES);
  protected readonly lead = computed(() => {
    const rows = this.store.rows();
    const first = rows[0];
    const same =
      first && first.nextLevelXp > 0 && rows.every((r) => r.level === first.level && r.nextLevelXp === first.nextLevelXp);
    return experienceLead(this.store.xpMode(), same ? { level: first.level + 1, xp: first.nextLevelXp } : null);
  });
  protected readonly emptyHistory = computed(() => {
    if (this.milestones()) {
      return this.isMaster()
        ? 'Nenhum marco ainda. Registre um quando o grupo cumprir algo importante na história.'
        : 'Nenhum marco ainda.';
    }
    return this.isMaster()
      ? 'Ninguém recebeu XP ainda. Dê XP depois de um combate ou quando quiser.'
      : 'Ninguém recebeu XP ainda.';
  });

  protected async given(result: GiveResult): Promise<void> {
    if (result.kind === 'xp') {
      this.confirmation.set(
        givenText(result.result.award.totalXp, result.result.xpEach, result.result.lostXp),
      );
    } else {
      this.confirmation.set(
        milestoneText(result.award, result.award.shares.length === this.store.rows().length),
      );
    }
    await this.store.refresh();
  }

  protected retry(): void {
    void this.store.refresh();
  }
}
