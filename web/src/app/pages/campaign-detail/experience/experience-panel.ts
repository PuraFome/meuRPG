import { Component, ElementRef, Injector, afterNextRender, computed, effect, inject, input, signal, viewChild } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import { XpMode } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { ExperienceStore } from '../../../core/progression/experience-store';
import { townGivenText } from '../../../core/progression/treasure';
import { experienceLead, givenText, milestoneText } from '../../../core/progression/xp-labels';
import { TreasureStrip } from '../../../shared/xp/treasure-strip';
import { type GiveResult, XpGiveButton } from '../../../shared/xp/xp-give-button';
import { MilestonesPanel } from '../milestones/milestones-panel';
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
  imports: [AwardHistory, MatButtonModule, MatIconModule, MilestonesPanel, TreasureStrip, XpGiveButton, XpRows],
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
  /** What each character got from the award just made ("+105 XP"), until the next change. */
  protected readonly gained = signal<ReadonlyMap<string, number>>(new Map());
  private readonly status = viewChild<ElementRef<HTMLElement>>('status');
  private readonly injector = inject(Injector);

  constructor() {
    // The news sits above the card: bring it into view, whatever the page was showing.
    effect(() => {
      if (this.confirmation()) {
        afterNextRender(() => this.status()?.nativeElement.scrollIntoView?.({ block: 'start' }), { injector: this.injector });
      }
    });
  }

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
      const { award, xpEach, lostXp } = result.result;
      this.confirmation.set(
        award.treasureCount > 0 ? townGivenText(award, xpEach, lostXp) : givenText(award.totalXp, xpEach, lostXp),
      );
      this.gained.set(new Map(award.shares.map((s) => [s.characterId, s.xp])));
    } else {
      this.confirmation.set(
        milestoneText(result.award, result.award.shares.length === this.store.rows().length),
      );
    }
    await this.store.refresh();
  }

  /** An undo: what was said about the award before is no longer true. */
  protected undone(): void {
    this.confirmation.set('');
    this.gained.set(new Map());
  }

  protected retry(): void {
    void this.store.refresh();
  }
}
