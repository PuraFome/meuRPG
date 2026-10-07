import { NgTemplateOutlet } from '@angular/common';
import {
  Component,
  ElementRef,
  OnInit,
  computed,
  effect,
  inject,
  input,
  output,
  signal,
  untracked,
} from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';

import type { Milestone } from '../../../../gen/meurpg/progression/v1/progression_pb';
import { AuthService } from '../../../core/auth/auth.service';
import { ExperienceStore } from '../../../core/progression/experience-store';
import {
  givenConfirmation,
  markedIds,
  playerAnnouncement,
  reachedConfirmation,
} from '../../../core/progression/milestones';
import { MilestonesStore } from '../../../core/progression/milestones-store';
import { milestoneText } from '../../../core/progression/xp-labels';
import { openMilestone } from '../../../shared/xp/xp-give-button';
import { MilestoneCharacters } from './milestone-characters';
import { type MarkReachedResult, openMarkReached } from './mark-reached-sheet';
import { PlannedMilestones } from './planned-milestones';
import { ReachedMilestones } from './reached-milestones';

/**
 * What "Experiência" shows in a campaign that levels by milestones (E8-14,
 * MR-016, question 45). The master has the planned list (his own) and the
 * reached ones; "Marcar como alcançado" opens the sheet of who levels up, and
 * "Registrar um marco fora da lista" is the old ad hoc flow, kept as a text
 * action. A player reads only the reached milestones, with when and who, and
 * their own character: nothing here hints that others are planned, and the
 * character row waits for the first milestone. The page's `ExperienceStore`
 * has the characters; this panel's `MilestonesStore` has the milestones.
 * What the master just did is said once, in the host's polite status
 * (`confirmed`); what a player's screen learns on its own (a tab that comes
 * back to the front) is said in this panel's own.
 */
@Component({
  selector: 'app-milestones-panel',
  imports: [
    MatButtonModule,
    MatIconModule,
    MilestoneCharacters,
    NgTemplateOutlet,
    PlannedMilestones,
    ReachedMilestones,
  ],
  providers: [MilestonesStore],
  templateUrl: './milestones-panel.html',
  styleUrl: './milestones-panel.scss',
  host: { '(document:visibilitychange)': 'visible()' },
})
export class MilestonesPanel implements OnInit {
  protected readonly store = inject(MilestonesStore);
  protected readonly experience = inject(ExperienceStore);
  private readonly auth = inject(AuthService);
  private readonly dialog = inject(MatDialog);
  private readonly bottomSheet = inject(MatBottomSheet);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

  readonly campaignId = input.required<string>();
  readonly campaignName = input('');
  readonly isMaster = input(false);

  /** What the master just did, for the host's status line. */
  readonly confirmed = output<string>();

  /** What a player's screen learned by itself, for this panel's live region. */
  protected readonly announcement = signal('');

  protected readonly lead = computed(() =>
    this.isMaster()
      ? 'Campanha por marcos: o nível sobe quando você marca um marco como alcançado, sem contar XP.'
      : 'Campanha por marcos: o mestre marca quando o grupo cumpre um marco.',
  );
  protected readonly emptyReached = computed(() =>
    this.isMaster()
      ? 'Nenhum marco alcançado ainda. Quando você marcar um, ele aparece aqui com o dia, a hora e quem subiu de nível.'
      : 'Nenhum marco alcançado ainda. Quando o grupo cumprir um, ele aparece aqui.',
  );

  private readonly viewerId = computed(() => {
    const auth = this.auth.state();
    return auth.status === 'signed-in' ? auth.user.id : '';
  });

  /** The characters that can be listed: the master's whole group, a player's own. */
  protected readonly characters = computed(() =>
    this.isMaster()
      ? this.experience.rows()
      : this.experience.rows().filter((r) => r.playerUserId === this.viewerId()),
  );
  /** The reached milestones some living character does not have yet. */
  protected readonly giveable = computed(() => {
    const rows = this.experience.rows();
    return new Set(
      this.store
        .reached()
        .filter((m) => !m.offList && rows.some((r) => !markedIds(m).has(r.id)))
        .map((m) => m.id),
    );
  });
  /** Nothing is said about anyone before the first milestone. */
  protected readonly showCharacters = computed(
    () => this.store.reached().length > 0 && this.characters().length > 0,
  );
  protected readonly showReached = computed(
    () => !this.isMaster() || this.store.planned().length > 0 || this.store.reached().length > 0,
  );

  private known: ReadonlySet<string> | null = null;

  constructor() {
    // A milestone that appears by itself (the player's tab came back): said once.
    effect(() => {
      const reached = this.store.reached();
      if (this.store.state() !== 'ready') {
        return; // the first read is not news
      }
      untracked(() => {
        const ids = new Set(reached.map((m) => m.id));
        if (this.known && !this.isMaster()) {
          const mine = new Set(this.characters().map((r) => r.id));
          const news = reached.filter((m) => !this.known!.has(m.id));
          if (news.length > 0) {
            this.announcement.set(news.map((m) => playerAnnouncement(m, mine)).join(' '));
          }
        }
        this.known = ids;
      });
    });
  }

  ngOnInit(): void {
    void this.store.load(this.campaignId());
  }

  /** The tab came back to the front: the master may have marked meanwhile. */
  protected visible(): void {
    if (document.visibilityState === 'visible' && this.store.state() === 'ready') {
      void this.reload();
    }
  }

  protected retry(): void {
    void this.reload();
  }

  protected reach(m: Milestone): void {
    openMarkReached(this.dialog, this.bottomSheet, {
      campaignId: this.campaignId(),
      milestoneId: m.id,
      text: m.text,
      rows: this.experience.rows(),
      give: false,
    }).subscribe((result) => void this.afterSheet(result, reachedConfirmationOf, '#planned-title'));
  }

  protected give(m: Milestone): void {
    const has = markedIds(m);
    openMarkReached(this.dialog, this.bottomSheet, {
      campaignId: this.campaignId(),
      milestoneId: m.id,
      text: m.text,
      rows: this.experience.rows().filter((r) => !has.has(r.id)),
      give: true,
    }).subscribe((result) => void this.afterSheet(result, givenConfirmationOf, '#reached-title'));
  }

  /** "Registrar um marco fora da lista": the ad hoc flow of slice 7.4. */
  protected registerOffList(): void {
    openMilestone(this.dialog, this.bottomSheet, {
      campaignId: this.campaignId(),
      campaignName: this.campaignName(),
      rows: this.experience.rows(),
    }).subscribe((award) => {
      if (award) {
        this.confirmed.emit(
          milestoneText(award, award.shares.length === this.experience.rows().length),
        );
        void this.reload();
      }
    });
  }

  protected undone(message: string): void {
    if (message) {
      this.confirmed.emit(message);
    }
    void this.reload().then(() => this.focusHeading('#reached-title'));
  }

  private async afterSheet(
    result: MarkReachedResult | undefined,
    say: (r: MarkReachedResult) => string,
    focus: string,
  ): Promise<void> {
    // Read again even when the master cancelled: a refusal inside the sheet
    // means the list had moved.
    await this.reload();
    if (result) {
      this.confirmed.emit(say(result));
      this.focusHeading(focus);
    }
  }

  private async reload(): Promise<void> {
    await Promise.all([this.store.refresh(), this.experience.refresh()]);
  }

  /** The focus goes to the heading of the list that changed, once it is drawn. */
  private focusHeading(selector: string): void {
    setTimeout(() => this.host.nativeElement.querySelector<HTMLElement>(selector)?.focus());
  }
}

const reachedConfirmationOf = (r: MarkReachedResult): string =>
  reachedConfirmation(r.marked, r.left);
const givenConfirmationOf = (r: MarkReachedResult): string => givenConfirmation(r.marked);
