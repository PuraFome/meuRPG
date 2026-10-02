import {
  Component,
  DOCUMENT,
  DestroyRef,
  Injector,
  computed,
  effect,
  inject,
  signal,
  untracked,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { ActivatedRoute, Router } from '@angular/router';

import { AuthService } from '../../core/auth/auth.service';
import { OpenSessions } from '../../shell/live-notice/open-sessions';
import { AdjustVitals } from './adjust-vitals/adjust-vitals';
import { AdjustVitalsData, AdjustVitalsResult } from './adjust-vitals/adjust-vitals.types';
import {
  CampaignInfoVm,
  LiveSessionSource,
  LiveSessionVm,
  PartyMemberInfoVm,
  PlayerSheetVm,
  VitalsVm,
} from './live-session.types';
import { LiveStream } from './live-stream';
import { PartyPanel } from './party-panel/party-panel';
import { PlayerVitals } from './player-vitals/player-vitals';
import { SessionBlocked } from './session-blocked/session-blocked';
import { SessionHeader } from './session-header/session-header';
import { SessionMap } from './session-map/session-map';
import { applySnapshot, applyVitals, partyRowSub } from './vitals';

/**
 * Where the page stands. `live` covers reconnecting too: the numbers stay
 * on screen, readable, while the status line says "Reconectando…".
 */
type Phase = 'loading' | 'live' | 'no-access' | 'no-session' | 'ended' | 'error';

/**
 * The session page, `/campanhas/:id/sessao` (MR-011, MR-012, RN-02, RN-06,
 * RN-07; artboards E5-02 to E5-08).
 *
 * It asks for the campaign (its name, and whether the person is its
 * master), then opens the live stream (`LiveStream`: ADR-0005's client
 * rules). On every `ready` it reads the snapshot (`GetLiveSession`), and
 * applies each `vitals_changed` whose revision is newer. The player sees
 * their own character's vitals; the master sees the party, with "Ajustar".
 *
 * The map half comes with the maps slice: `SessionMap` shows its empty
 * state for now.
 */
@Component({
  selector: 'app-live-session',
  imports: [
    MatButtonModule,
    MatIconModule,
    PartyPanel,
    PlayerVitals,
    SessionBlocked,
    SessionHeader,
    SessionMap,
  ],
  templateUrl: './live-session.html',
  styleUrl: './live-session.scss',
})
export class LiveSession {
  private readonly source = inject(LiveSessionSource);
  private readonly auth = inject(AuthService);
  private readonly router = inject(Router);
  private readonly document = inject(DOCUMENT);
  private readonly dialog = inject(MatDialog);
  private readonly bottomSheet = inject(MatBottomSheet);
  private readonly openSessions = inject(OpenSessions);
  private readonly destroyRef = inject(DestroyRef);
  /** The adjust sheet needs this route's `LiveSessionSource`: MatDialog
   * and MatBottomSheet live in the root injector, which doesn't have it. */
  private readonly injector = inject(Injector);

  protected readonly campaignId = signal('');
  protected readonly phase = signal<Phase>('loading');
  protected readonly campaign = signal<CampaignInfoVm | null>(null);
  protected readonly session = signal<LiveSessionVm | null>(null);
  protected readonly vitals = signal<readonly VitalsVm[]>([]);
  protected readonly playerSheet = signal<PlayerSheetVm | null>(null);
  protected readonly partyInfo = signal<ReadonlyMap<string, PartyMemberInfoVm>>(new Map());

  protected readonly stream = signal<LiveStream | null>(null);
  protected readonly connection = computed(() => this.stream()?.status() ?? 'connecting');
  protected readonly lastUpdate = computed(() => this.stream()?.lastMessageAt() ?? null);
  protected readonly isMaster = computed(() => this.campaign()?.isMaster ?? false);
  /** The player's own character: the only one the server sends them. */
  protected readonly ownVitals = computed(() => this.vitals()[0] ?? null);

  /** The campaign of the current load; `null` once it turned out the page
   * can't show it (no access, a pending member, an error). */
  private campaignLoad: Promise<CampaignInfoVm | null> = Promise.resolve(null);
  private loadedSheetFor: string | null = null;
  private partyInfoIds = new Set<string>();
  private generation = 0;

  constructor() {
    inject(ActivatedRoute)
      .paramMap.pipe(takeUntilDestroyed())
      .subscribe((params) => {
        const id = params.get('id');
        if (id) {
          this.load(id);
        }
      });
    this.destroyRef.onDestroy(() => this.closeStream());

    // Waiting on the link before the master starts: the app's light poll
    // (RN-06) notices the new session, and the page opens it by itself.
    effect(() => {
      const id = this.campaignId();
      if (this.phase() === 'no-session' && this.openSessions.liveCampaignIds().has(id)) {
        untracked(() => this.load(id));
      }
    });
  }

  private load(campaignId: string): void {
    const generation = ++this.generation;
    this.closeStream();
    this.campaignId.set(campaignId);
    this.phase.set('loading');
    this.campaign.set(null);
    this.session.set(null);
    this.vitals.set([]);
    this.playerSheet.set(null);
    this.partyInfo.set(new Map());
    this.loadedSheetFor = null;
    this.partyInfoIds = new Set();

    // The campaign (its name, and whether the person is its master) and the
    // stream start together: at the table, every round trip is a wait.
    this.campaignLoad = this.source.getCampaign(campaignId).then(
      (campaign) => {
        if (generation !== this.generation) {
          return null;
        }
        if (campaign.awaitingApproval) {
          // A pending member isn't a member yet (RN-15): same page as a
          // stranger, without the campaign's name.
          this.closeStream();
          this.phase.set('no-access');
          return null;
        }
        this.campaign.set(campaign);
        return campaign;
      },
      (err: unknown) => {
        if (generation === this.generation) {
          this.closeStream();
          this.fail(err);
        }
        return null;
      },
    );
    this.openStream(campaignId, generation);
  }

  private openStream(campaignId: string, generation: number): void {
    const stream = new LiveStream({
      open: (signal) => this.source.watch(campaignId, signal),
      classify: (err) => this.source.classifyError(err),
      document: this.document,
      handlers: {
        onReady: () => void this.readSnapshot(campaignId, generation),
        onVitals: (v) => this.vitals.update((list) => applyVitals(list, v)),
        onEnded: () => this.ended(),
        onFatal: (kind) => {
          if (kind === 'signed-out') {
            this.auth.signIn(this.router.url);
          } else {
            this.phase.set('no-access');
          }
        },
      },
    });
    this.stream.set(stream);
    stream.start();
  }

  private async readSnapshot(campaignId: string, generation: number): Promise<void> {
    try {
      const [snapshot, campaign] = await Promise.all([
        this.source.getLiveSession(campaignId),
        this.campaignLoad,
      ]);
      if (generation !== this.generation || !campaign) {
        return;
      }
      this.session.set(snapshot.session);
      this.vitals.update((list) => applySnapshot(list, snapshot.vitals));
      this.phase.set('live');
      this.stream()?.confirmHealthy();
      // Here already: the notice about this session has nothing to add.
      this.openSessions.dismiss(snapshot.session.sessionId);
      this.loadDetails(campaignId, generation);
    } catch (err) {
      if (generation !== this.generation) {
        return;
      }
      switch (this.source.classifyError(err)) {
        case 'no-session':
          this.ended();
          return;
        case 'no-access':
          this.closeStream();
          this.phase.set('no-access');
          return;
        case 'signed-out':
          this.closeStream();
          this.auth.signIn(this.router.url);
          return;
        default:
          // Try the whole thing again: the next `ready` reads a new one.
          this.stream()?.restart();
      }
    }
  }

  /** What the vitals come with: the player's CA and class line, or the
   * master's class and player names per character. Best effort: the live
   * numbers don't wait for these. */
  private loadDetails(campaignId: string, generation: number): void {
    if (this.isMaster()) {
      const ids = this.vitals().map((v) => v.characterId);
      if (ids.every((id) => this.partyInfoIds.has(id))) {
        return;
      }
      ids.forEach((id) => this.partyInfoIds.add(id));
      this.source.getPartyInfo(campaignId).then(
        (info) => generation === this.generation && this.partyInfo.set(info),
        () => undefined,
      );
      return;
    }
    const own = this.ownVitals();
    if (!own || this.loadedSheetFor === own.characterId) {
      return;
    }
    this.loadedSheetFor = own.characterId;
    this.source.getPlayerSheet(campaignId, own.characterId).then(
      (sheet) => generation === this.generation && this.playerSheet.set(sheet),
      () => undefined,
    );
  }

  private ended(): void {
    this.closeStream();
    // Without a snapshot, there was never a session on this page.
    this.phase.set(this.session() ? 'ended' : 'no-session');
    void this.openSessions.refresh();
  }

  private fail(err: unknown): void {
    switch (this.source.classifyError(err)) {
      case 'no-access':
        this.phase.set('no-access');
        return;
      case 'signed-out':
        this.auth.signIn(this.router.url);
        return;
      default:
        this.phase.set('error');
    }
  }

  private closeStream(): void {
    this.stream()?.stop();
    this.stream.set(null);
  }

  protected retry(): void {
    this.load(this.campaignId());
  }

  protected sessionEndedHere(): void {
    this.ended();
  }

  /** "Ajustar": a bottom sheet on a phone, a dialog from a tablet up, with
   * the same content (README-A). */
  protected adjust(vitals: VitalsVm): void {
    const info = this.partyInfo().get(vitals.characterId);
    const data: AdjustVitalsData = {
      campaignId: this.campaignId(),
      vitals,
      sub: partyRowSub(info),
      playerName: info?.playerName ?? null,
    };
    const onPhone = this.document.defaultView?.matchMedia('(max-width: 767.98px)').matches ?? false;
    const closed = onPhone
      ? this.bottomSheet
          .open<AdjustVitals, AdjustVitalsData, AdjustVitalsResult>(AdjustVitals, {
            data,
            injector: this.injector,
            ariaLabel: `Ajustar ${vitals.name}`,
            // The title first, so a stray Enter can't take 5 HP.
            autoFocus: 'first-heading',
          })
          .afterDismissed()
      : this.dialog
          .open<AdjustVitals, AdjustVitalsData, AdjustVitalsResult>(AdjustVitals, {
            data,
            injector: this.injector,
            width: '440px',
            maxWidth: 'calc(100vw - 32px)',
            ariaLabelledBy: 'adjust-title',
            autoFocus: 'first-heading',
          })
          .afterClosed();
    closed.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((result) => {
      if (result?.kind === 'saved') {
        this.vitals.update((list) => applyVitals(list, result.vitals));
      } else if (result?.kind === 'ended') {
        this.ended();
      }
    });
  }
}
