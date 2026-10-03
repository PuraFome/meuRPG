import { Component, inject, input, output } from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';

import { XpMode } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import type { XPAward } from '../../../gen/meurpg/progression/v1/progression_pb';
import type { ExperienceRow } from '../../core/progression/experience-store';
import { openSheet } from '../../pages/live-session/combat/sheet-host';
import { type AwardXpData, type AwardXpResult, AwardXpSheet } from './award-xp-sheet';
import { type MilestoneData, MilestoneSheet } from './milestone-sheet';

/** What a master gave from the button: an XP award, or a milestone. */
export type GiveResult =
  | { readonly kind: 'xp'; readonly result: AwardXpResult }
  | { readonly kind: 'milestone'; readonly award: XPAward };

/** Opens "Dar XP" (a dialog on a desktop, a bottom sheet on a phone). It
 * answers the result, or `undefined` when the master cancelled. */
export function openAwardXp(
  dialog: MatDialog,
  bottomSheet: MatBottomSheet,
  data: AwardXpData,
) {
  return openSheet<AwardXpSheet, AwardXpData, AwardXpResult | undefined>(dialog, bottomSheet, AwardXpSheet, {
    data,
    ariaLabel: 'Dar XP',
    labelledBy: 'sheet-t',
    width: '560px',
    focus: '[data-initial-focus]',
  });
}

/** Opens "Registrar marco". */
export function openMilestone(dialog: MatDialog, bottomSheet: MatBottomSheet, data: MilestoneData) {
  return openSheet<MilestoneSheet, MilestoneData, XPAward | undefined>(dialog, bottomSheet, MilestoneSheet, {
    data,
    ariaLabel: 'Registrar marco',
    labelledBy: 'sheet-t',
    width: '560px',
    focus: '[data-initial-focus]',
  });
}

/**
 * The master's "Dar XP" button (outlined: the page's one filled button is
 * somewhere else), or "Registrar marco" in a campaign that levels by
 * milestones. It opens the matching sheet and says what was given; the host
 * (the campaign's "Experiência" panel, the session's "Grupo") reads the XP
 * again and shows the confirmation. Nothing here for a campaign whose mode is
 * not known yet.
 */
@Component({
  selector: 'app-xp-give-button',
  imports: [MatButtonModule, MatIconModule],
  template: `
    @if (milestones()) {
      <button mat-stroked-button type="button" class="give" (click)="openMark()">
        <mat-icon aria-hidden="true">flag</mat-icon>Registrar marco
      </button>
    } @else if (xpMode() !== Mode.UNSPECIFIED) {
      <button mat-stroked-button type="button" class="give" (click)="openXp()">Dar XP</button>
    }
  `,
  styles: `
    :host {
      display: inline-flex;
    }

    .give {
      --mat-button-outlined-label-text-color: var(--mr-accent-text);
    }
  `,
})
export class XpGiveButton {
  private readonly dialog = inject(MatDialog);
  private readonly bottomSheet = inject(MatBottomSheet);

  readonly campaignId = input.required<string>();
  readonly campaignName = input('');
  readonly xpMode = input.required<XpMode>();
  readonly rows = input.required<readonly ExperienceRow[]>();

  /** The master gave XP or marked a milestone. */
  readonly given = output<GiveResult>();

  protected readonly Mode = XpMode;
  protected milestones(): boolean {
    return this.xpMode() === XpMode.MILESTONES;
  }

  protected openXp(): void {
    openAwardXp(this.dialog, this.bottomSheet, {
      campaignId: this.campaignId(),
      xpMode: this.xpMode(),
      rows: this.rows(),
    }).subscribe((result) => {
      if (result) {
        this.given.emit({ kind: 'xp', result });
      }
    });
  }

  protected openMark(): void {
    openMilestone(this.dialog, this.bottomSheet, {
      campaignId: this.campaignId(),
      campaignName: this.campaignName(),
      rows: this.rows(),
    }).subscribe((award) => {
      if (award) {
        this.given.emit({ kind: 'milestone', award });
      }
    });
  }
}
