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

import type { Map as MapMessage } from '../../../gen/meurpg/maps/v1/maps_pb';
import type { Encounter } from '../../../gen/meurpg/play/v1/combat_pb';
import { AuthService } from '../../core/auth/auth.service';
import { CombatClient } from '../../core/combat/combat-client';
import { CombatState } from '../../core/combat/combat-state';
import { MapState } from '../../core/maps/map-state';
import { MapsClient } from '../../core/maps/maps-client';
import { OpenSessions } from '../../shell/live-notice/open-sessions';
import { AdjustVitals } from './adjust-vitals/adjust-vitals';
import { AdjustVitalsData, AdjustVitalsResult } from './adjust-vitals/adjust-vitals.types';
import {
  CampaignInfoVm,
  LiveSessionSource,
  LiveSessionVm,
  PartyMemberInfoVm,
  PlayerSheetVm,
  ShownImageVm,
  VitalsVm,
} from './live-session.types';
import { CombatLaunch } from './combat/combat-launch';
import { DiceDialog, DiceDialogData } from './combat/dice-dialog';
import { CombatView } from './combat/combat-view';
import { LiveStream } from './live-stream';
import { PartyPanel } from './party-panel/party-panel';
import { PlayerVitals } from './player-vitals/player-vitals';
import { SessionBlocked } from './session-blocked/session-blocked';
import { SessionHeader } from './session-header/session-header';
import { SessionMap } from './session-map/session-map';
import { SessionTokens } from './session-tokens/session-tokens';
import { LeftImagesBlock } from './left-images-block/left-images-block';
import { ShownImageBlock } from './shown-image-block/shown-image-block';
import { ShownImagePanel } from './shown-image-panel/shown-image-panel';
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
 * The map half (MR-012): the snapshot carries the current map and the
 * shown image (MR-028), and the stream's `current_map_changed`,
 * `map_changed`, `token_moved` and `shown_image_changed` keep them current
 * without a reload. `map_changed` reads the map again; if that answers
 * `not_found`, a player lost sight of it and gets the empty state.
 */
@Component({
  selector: 'app-live-session',
  imports: [
    MatButtonModule,
    MatIconModule,
    CombatLaunch,
    CombatView,
    PartyPanel,
    PlayerVitals,
    SessionBlocked,
    SessionHeader,
    SessionMap,
    SessionTokens,
    LeftImagesBlock,
    ShownImageBlock,
    ShownImagePanel,
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
  private readonly mapsApi = inject(MapsClient);
  private readonly combatApi = inject(CombatClient);
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

  /** The session's current map, the image on show, and the map itself. */
  protected readonly currentMapId = signal<string | null>(null);
  protected readonly shownImage = signal<ShownImageVm | null>(null);
  /** The master's "Deixar com os jogadores" switch for the image on show. */
  protected readonly shownKeep = signal(false);
  /** The images the master left with the players (MR-028). */
  protected readonly leftImages = signal<readonly ShownImageVm[]>([]);
  /** What a player's screen reader hears when the master shows or stops. */
  protected readonly shownNotice = signal('');
  protected readonly campaignMaps = signal<readonly MapMessage[]>([]);
  protected readonly mapState = new MapState((mapId) =>
    this.mapsApi.get(this.campaignId(), mapId),
  );

  /** The session's combat (MR-013): read on every `ready` and after each
   * `encounter_changed`; `turn_changed` and `combatant_moved` apply in place. */
  protected readonly combat = new CombatState();

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
    this.currentMapId.set(null);
    this.shownImage.set(null);
    this.shownKeep.set(false);
    this.leftImages.set([]);
    this.shownNotice.set('');
    this.campaignMaps.set([]);
    void this.mapState.open(null);
    this.combat.clear();
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
        onCurrentMap: (mapId) => this.showMap(mapId),
        onMapChanged: (mapId) => this.mapChanged(mapId),
        onTokenMoved: (move) => {
          // A token the page doesn't know (a missed `map_changed`): read again.
          if (!this.mapState.moveToken(move.mapId, move.characterId, move.xBp, move.yBp)) {
            void this.mapState.refresh();
          }
        },
        onEncounterChanged: (change) => {
          // Read again only when the news is newer than the copy on screen.
          const current = this.combat.encounter();
          if (!current || current.id !== change.encounterId || change.revision > current.revision) {
            void this.loadCombat(generation);
          }
        },
        onTurnChanged: (turn) => {
          if (!this.combat.applyTurn(turn)) {
            void this.loadCombat(generation);
          }
        },
        onCombatantMoved: (move) => {
          if (!this.combat.applyMove(move)) {
            void this.loadCombat(generation);
          }
        },
        onCombatLogChanged: () => this.combat.touchLog(),
        onShownImage: (image) => this.shownImageChanged(image),
        onLeftImages: () => void this.reloadLeftImages(),
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
      this.currentMapId.set(snapshot.currentMapId);
      this.shownImage.set(snapshot.shownImage);
      this.shownKeep.set(snapshot.shownImageKeep);
      void this.reloadLeftImages();
      void this.loadCombat(generation);
      this.combat.touchLog(); // the log is read again too, after a reconnection
      // Each `ready` (a reconnection too) reads the map again: a missed event never leaves it stale.
      void this.mapState.open(snapshot.currentMapId);
      if (this.isMaster()) {
        void this.reloadMaps();
      }
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

  /** The session's combat, as this person may see it (best effort: the
   * screen keeps the copy it has until the next event or `ready`). */
  private async loadCombat(generation: number): Promise<void> {
    try {
      const encounter = await this.combatApi.get(this.campaignId());
      if (generation === this.generation) {
        this.combat.apply(encounter);
      }
    } catch {
      // The stream's next event, or reconnection, reads it again.
    }
  }

  /** "Iniciar combate" answered: the combat is on screen at once. */
  protected combatStarted(encounter: Encounter): void {
    this.combat.apply(encounter);
  }

  /** "Mudar" on the initiative screen: the player's dice choice (RN-18). */
  protected changeDice(): void {
    const campaign = this.campaign();
    if (!campaign) {
      return;
    }
    this.dialog
      .open<DiceDialog, DiceDialogData, boolean>(DiceDialog, {
        data: {
          campaignId: this.campaignId(),
          campaignName: campaign.name,
          mode: campaign.diceMode,
          preference: campaign.dicePreference,
        },
        width: '560px',
        maxWidth: 'calc(100vw - 32px)',
        autoFocus: 'first-heading',
      })
      .afterClosed()
      .subscribe(() => void this.refreshCampaign());
  }

  /** The campaign again, to pick up the dice choice the player just saved. */
  private async refreshCampaign(): Promise<void> {
    const generation = this.generation;
    try {
      const campaign = await this.source.getCampaign(this.campaignId());
      if (generation === this.generation) {
        this.campaign.set(campaign);
      }
    } catch {
      // Best effort: the screen keeps the choice it had.
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

  /** `current_map_changed`, or the master's own choice. */
  protected showMap(mapId: string | null): void {
    this.currentMapId.set(mapId);
    void this.mapState.open(mapId);
    if (this.isMaster()) {
      void this.reloadMaps();
    }
  }

  private mapChanged(mapId: string): void {
    if (mapId === this.currentMapId()) {
      void this.mapState.refresh();
    }
    if (this.isMaster()) {
      void this.reloadMaps();
    }
  }

  /** `shown_image_changed`: the block appears, changes or goes away. */
  private shownImageChanged(image: ShownImageVm | null): void {
    if (!this.isMaster()) {
      const previous = this.shownImage();
      if (image) {
        this.shownNotice.set(`O mestre está mostrando ${image.name}.`);
      } else if (previous) {
        this.shownNotice.set('O mestre parou de mostrar a imagem.');
      }
    }
    // The switch is per image: a new image, or none, starts with it off.
    this.shownKeep.set(false);
    this.shownImage.set(image);
  }

  protected leftWithout(image: ShownImageVm): readonly ShownImageVm[] {
    return this.leftImages().filter((i) => i.id !== image.id);
  }

  /** The images left with the players, read again (best effort: the list
   * keeps what it had). */
  protected async reloadLeftImages(): Promise<void> {
    const generation = this.generation;
    try {
      const images = await this.source.listLeftImages(this.campaignId());
      if (generation === this.generation) {
        this.leftImages.set(images);
      }
    } catch {
      // Best effort.
    }
  }

  /** The master's select and the "Fundo de mapa escondido" tags read the list. */
  private async reloadMaps(): Promise<void> {
    const generation = this.generation;
    try {
      const maps = await this.mapsApi.list(this.campaignId());
      if (generation === this.generation) {
        this.campaignMaps.set(maps);
      }
    } catch {
      // Best effort: the select keeps what it had.
    }
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
