import { Component, effect, inject, input, signal, untracked } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { ExperienceStore } from '../../../core/progression/experience-store';
import { XpChanges } from '../../../core/progression/xp-changes';
import { givenText, milestoneText } from '../../../core/progression/xp-labels';
import { type GiveResult, XpGiveButton } from '../../../shared/xp/xp-give-button';

/**
 * The master's "Dar XP" (or "Registrar marco") on the session page (E7-07):
 * the button for the "Grupo" panel's heading, and under it what was just
 * given, in a polite status. XP can be given at any time, in a combat or out
 * of one. It reads the campaign's XP itself (the characters to pick from) and
 * again on `xp_changed`, so the sheet it opens always lists who is alive now.
 */
@Component({
  selector: 'app-session-xp',
  imports: [MatIconModule, XpGiveButton],
  providers: [ExperienceStore],
  template: `
    <app-xp-give-button
      [campaignId]="campaignId()"
      [campaignName]="campaignName()"
      [xpMode]="store.xpMode()"
      [rows]="store.rows()"
      (given)="given($event)"
    />
    <div class="status" role="status">
      @if (confirmation()) {
        <p class="mr-notice mr-notice--success">
          <mat-icon aria-hidden="true">check_circle</mat-icon>{{ confirmation() }}
        </p>
      }
    </div>
  `,
  styles: `
    // The host adds its two parts to the panel's heading grid: the button in the
    // last column, the news under the whole row.
    :host {
      display: contents;
    }

    .status {
      grid-column: 1 / -1;
    }

    .status:empty {
      display: none;
    }

    .mr-notice {
      margin: 0;
    }
  `,
})
export class SessionXp {
  protected readonly store = inject(ExperienceStore);
  private readonly changes = inject(XpChanges);

  readonly campaignId = input.required<string>();
  readonly campaignName = input('');

  protected readonly confirmation = signal('');

  constructor() {
    effect(() => {
      const id = this.campaignId();
      untracked(() => void this.store.load(id, false, true));
    });
    // `xp_changed`: the characters' XP and who is alive may have moved.
    let seen = untracked(() => this.changes.version());
    effect(() => {
      const v = this.changes.version();
      if (v !== seen) {
        seen = v;
        untracked(() => void this.store.refresh());
      }
    });
  }

  protected async given(result: GiveResult): Promise<void> {
    this.confirmation.set(
      result.kind === 'xp'
        ? givenText(result.result.award.totalXp, result.result.xpEach, result.result.lostXp)
        : milestoneText(result.award, result.award.shares.length === this.store.rows().length),
    );
    await this.store.refresh();
  }
}
