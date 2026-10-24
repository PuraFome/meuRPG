import {
  Component,
  DestroyRef,
  Injector,
  afterNextRender,
  computed,
  effect,
  inject,
  signal,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import {
  ActivatedRoute,
  Router,
  RouterLink,
  RouterLinkActive,
  RouterOutlet,
} from '@angular/router';
import { Code, ConnectError } from '@connectrpc/connect';

import { Campaign, Member, Role } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { AuthService } from '../../core/auth/auth.service';
import { CampaignsService } from '../../core/campaigns/campaigns.service';
import { setPageSubject } from '../../core/title/page-title';
import { describeConnectError } from '../../core/connect/connect-errors';
import { LevelUpFeed } from '../../core/levelup/levelup-feed';
import { ExperienceStore } from '../../core/progression/experience-store';
import { LevelUpNotice } from './level-up-notice/level-up-notice';
import { campaignLead } from '../../core/campaigns/campaign-copy';
import { memberRows } from './campaign-detail.copy';
import { CampaignCharacters } from './characters/campaign-characters';
import { CampaignRoster } from './characters/campaign-roster';
import { CampaignView } from './campaign-view';
import { DicePreferencePanel } from '../../shared/dice-preference-panel/dice-preference-panel';
import { ExperiencePanel } from './experience/experience-panel';
import { MapsPanel } from './maps-panel/maps-panel';
import { CampaignSessions } from './game-session/campaign-sessions';
import { GameSessionCard } from './game-session/game-session-card';
import { PastSessionsPanel } from './past-sessions/past-sessions-panel';
import { CampaignMembers } from './members/campaign-members';
import { RuleLinks } from './rule-links/rule-links';

/** A link to an old place on the page lands on its tab: the fragment, and the tab that holds it. */
const FRAGMENT_TABS: Readonly<Record<string, string>> = { maps: 'prep' };

type PageState =
  | { status: 'loading' }
  | { status: 'not-found' }
  | { status: 'error'; message: string }
  | { status: 'ready'; campaign: Campaign; members: Member[] }
  // A pending member (RN-15, MR-024): the server sent only the campaign's
  // name. They see the wait banner and their own character, nothing else.
  | { status: 'pending'; campaign: Campaign };

/**
 * "/campaigns/:id" (guarded by authGuard): GetCampaign + ListMembers
 * (MR-001, MR-002) and everything the campaign page holds.
 *
 * **The master** sees the title, the session strip ("Sessão", MR-006 / RN-01,
 * always on top) and three tabs that are routes of their own, links in a `nav`
 * named "Seções da campanha": "Mesa" (`/campaigns/:id`, the default: the
 * players' characters, "Experiência", "Sessões anteriores", "Membros",
 * "Convites"), "Preparação" (`/prep`: maps, NPCs, encounters, puzzles, bestiary,
 * gallery, the campaign document) and "Regras" (`/setup`: "Dados", the rules,
 * the content, the spells, the package). The tab is drawn by the `router-outlet`
 * under the nav (see `campaign-detail.routes.ts`); a count on "Mesa" says how
 * many characters wait for approval. A tab change keeps the focus on the tab link.
 *
 * **A player** sees one page, with no tabs: "Sessão" (only while a session is
 * open, with "Entrar na sessão", RN-06), their characters, the experience, the
 * dice choice ("Como você rola os dados", E6-18) and the rules links.
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
    CampaignMembers,
    DicePreferencePanel,
    ExperiencePanel,
    MapsPanel,
    GameSessionCard,
    PastSessionsPanel,
    LevelUpNotice,
    MatIconModule,
    MatProgressSpinnerModule,
    RouterLink,
    RouterLinkActive,
    RouterOutlet,
    RuleLinks,
  ],
  // One XP store for the page: the "Experiência" panel and the characters' "Pode subir de nível" tag read it.
  // And one feed of the master's level-ups: the status line and the characters' "O que mudou" read it.
  // And one list of the sessions: "Sessão" and "Sessões anteriores" draw the same answer.
  // And one list of the characters (the "Mesa" count, the players' and the NPCs' panels), and what the tabs read.
  providers: [ExperienceStore, LevelUpFeed, CampaignSessions, CampaignRoster, CampaignView],
  templateUrl: './campaign-detail.html',
  styleUrl: './campaign-detail.scss',
})
export class CampaignDetail {
  private readonly campaigns = inject(CampaignsService);
  private readonly route = inject(ActivatedRoute);
  private readonly injector = inject(Injector);
  private readonly destroyRef = inject(DestroyRef);
  private readonly auth = inject(AuthService);
  private readonly router = inject(Router);
  private readonly view = inject(CampaignView);
  private readonly roster = inject(CampaignRoster);
  protected readonly experience = inject(ExperienceStore);

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

  /** How many characters wait for the master's approval: the count on "Mesa". */
  protected readonly waiting = computed(() => this.roster.awaiting().length);

  constructor() {
    // The tabs read the campaign and its members from here.
    effect(() => {
      const s = this.state();
      this.view.data.set(
        s.status === 'ready' ? { campaign: s.campaign, members: this.members() } : null,
      );
    });
    // The tab's title carries the campaign's name once it is loaded.
    setPageSubject(() => {
      const s = this.state();
      return s.status === 'ready' || s.status === 'pending' ? s.campaign.name : null;
    });
    this.route.paramMap.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((params) => {
      const id = params.get('id');
      if (id) {
        this.load(id);
      }
    });
  }

  /**
   * A link with a fragment ("Abrir os mapas", in "Regras da mesa") lands on its section once the page has rendered it.
   * A master's link to a panel that lives on another tab goes to that tab, fragment kept.
   */
  private landOnFragment(campaign: Campaign): void {
    const fragment = this.route.snapshot?.fragment;
    if (!fragment) {
      return;
    }
    const tab = FRAGMENT_TABS[fragment];
    if (tab && campaign.myRole === Role.MASTER && !this.router.url.split(/[?#]/)[0].endsWith(tab)) {
      void this.router.navigate(['/campaigns', campaign.id, tab], { fragment, replaceUrl: true });
      return;
    }
    afterNextRender(() => document.getElementById(fragment)?.scrollIntoView?.(), {
      injector: this.injector,
    });
  }

  /** A tab link keeps the focus after the click (Safari does not focus a link it was clicked on), so the page does not send it to the title. */
  protected keepFocus(event: Event): void {
    (event.currentTarget as HTMLElement).focus();
  }

  /** The newest `load`: an answer of an older one (the page was reused for another campaign) is dropped. */
  private loadSeq = 0;

  private load(campaignId: string): void {
    const mine = ++this.loadSeq;
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
      (state: PageState) => {
        if (mine !== this.loadSeq) {
          return;
        }
        this.state.set(state);
        if (state.status === 'ready') {
          this.landOnFragment(state.campaign);
          if (state.campaign.myRole === Role.MASTER) {
            void this.roster.ensureLoaded(campaignId);
          }
          // Every member reads the XP; the history comes with it. Only the master
          // reads the treasures waiting to be converted ("Voltar à cidade").
          void this.experience.load(campaignId, true, state.campaign.myRole === Role.MASTER);
        }
      },
      (err: unknown) => {
        if (mine !== this.loadSeq) {
          return;
        }
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
