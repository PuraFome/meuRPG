import { Component, DestroyRef, Injector, computed, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { Code, ConnectError } from '@connectrpc/connect';

import { Role } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import type { Map as MapMessage } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { CampaignsService } from '../../../core/campaigns/campaigns.service';
import { mapErrorMessage } from '../../../core/maps/map-errors';
import { MapState } from '../../../core/maps/map-state';
import { MapsClient } from '../../../core/maps/maps-client';
import { OpenSessionLookup } from '../../../core/play/open-session';
import { openImagePicker } from '../../../shared/gallery-picker/image-picker-dialog/image-picker-dialog';
import { PHONE_QUERY, mediaQuery } from '../../../shared/map-view/media-query';
import { MapEditor } from '../map-editor/map-editor';
import { MapHead } from '../map-head/map-head';
import { MapManage } from '../map-manage/map-manage';
import { PlayerMap } from '../player-map/player-map';

type Phase = 'loading' | 'ready' | 'gone' | 'error';

/**
 * `/campanhas/:id/mapas/:mapId` (MR-008, MR-009): one map, in the form that
 * fits who is looking and where.
 *
 * - The master on a computer gets the editor (E5-23).
 * - The master on a phone gets the pan-and-zoom map with the reveal lists
 *   (E5-24).
 * - A player gets the viewer (E5-25, E5-26): revealed points and visible
 *   tokens only. A map the player may not see is `not_found`, the same
 *   answer as a map that does not exist.
 *
 * The page loads the campaign (to know the role), the map and the campaign's
 * maps, and runs the header's calls ("Esconder", "Trocar imagem",
 * "Renomear", "Apagar mapa"). For the master it also asks whether the open
 * session is on this map, so deleting it warns about that (E6-27).
 */
@Component({
  selector: 'app-map-page',
  imports: [
    MapEditor,
    MapHead,
    MapManage,
    MatIconModule,
    MatProgressSpinnerModule,
    PlayerMap,
    RouterLink,
  ],
  templateUrl: './map-page.html',
  styleUrl: './map-page.scss',
})
export class MapPage {
  private readonly api = inject(MapsClient);
  private readonly campaigns = inject(CampaignsService);
  private readonly dialog = inject(MatDialog);
  private readonly injector = inject(Injector);
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly sessions = inject(OpenSessionLookup);

  protected readonly phone = mediaQuery(PHONE_QUERY);
  protected readonly campaignId = signal('');
  protected readonly phase = signal<Phase>('loading');
  protected readonly isMaster = signal(false);
  protected readonly maps = signal<readonly MapMessage[]>([]);
  protected readonly busy = signal(false);
  protected readonly notice = signal<string | null>(null);
  protected readonly fromSession = signal(false);
  /** The open session's number when it is on this map (the master only). */
  protected readonly currentSession = signal<number | null>(null);

  protected readonly state = new MapState((mapId) => this.api.get(this.campaignId(), mapId));
  protected readonly map = this.state.map;
  protected readonly others = computed(() =>
    this.maps().map((m) => ({ id: m.id, name: m.name })),
  );
  /** The player's list: revealed maps, and the one open. */
  protected readonly playerMaps = computed(() => this.maps());

  private generation = 0;

  constructor() {
    this.route.paramMap.pipe(takeUntilDestroyed(inject(DestroyRef))).subscribe((params) => {
      const id = params.get('id');
      const mapId = params.get('mapId');
      if (id && mapId) {
        void this.load(id, mapId);
      }
    });
    this.route.queryParamMap.pipe(takeUntilDestroyed(inject(DestroyRef))).subscribe((q) => {
      this.fromSession.set(q.get('de') === 'sessao');
    });
  }

  private async load(campaignId: string, mapId: string): Promise<void> {
    const generation = ++this.generation;
    const campaignChanged = campaignId !== this.campaignId();
    this.campaignId.set(campaignId);
    this.notice.set(null);
    if (campaignChanged || this.phase() !== 'ready') {
      this.phase.set('loading');
    }
    try {
      if (campaignChanged || !this.isMaster()) {
        const res = await this.campaigns.getCampaign(campaignId);
        if (generation !== this.generation) {
          return;
        }
        if (res.campaign?.awaitingApproval) {
          this.phase.set('gone');
          return;
        }
        this.isMaster.set(res.campaign?.myRole === Role.MASTER);
      }
      await this.state.open(mapId);
      if (generation !== this.generation) {
        return;
      }
      if (this.state.status() === 'gone') {
        this.phase.set('gone');
        return;
      }
      if (this.state.status() === 'error') {
        this.phase.set('error');
        return;
      }
      this.phase.set('ready');
      void this.reloadMaps();
      if (this.isMaster()) {
        void this.loadSession(mapId, generation);
      }
    } catch (err) {
      if (generation !== this.generation) {
        return;
      }
      this.phase.set(ConnectError.from(err).code === Code.NotFound ? 'gone' : 'error');
    }
  }

  protected retry(): void {
    const mapId = this.route.snapshot.paramMap.get('mapId');
    if (mapId) {
      void this.load(this.campaignId(), mapId);
    }
  }

  private async reloadMaps(): Promise<void> {
    try {
      this.maps.set(await this.api.list(this.campaignId()));
    } catch {
      // The list is a convenience (targets, the player's other maps).
    }
  }

  private async loadSession(mapId: string, generation: number): Promise<void> {
    const session = await this.sessions.currentMap(this.campaignId());
    if (generation === this.generation) {
      this.currentSession.set(session?.mapId === mapId ? session.sessionNumber : null);
    }
  }

  /** "Salvar nome" (E6-27): the header shows the message of a refusal. */
  protected readonly saveName = async (name: string): Promise<void> => {
    const map = this.map();
    if (!map) {
      return;
    }
    try {
      this.state.setMap(await this.api.update(this.campaignId(), map.id, map.revision, { name }));
    } catch (err) {
      throw new Error(mapErrorMessage(err, 'renomear o mapa'));
    }
    void this.reloadMaps();
  };

  /** "Apagar mapa", confirmed: the map is gone, so the page goes back to the
   * campaign, whose "Mapas" panel no longer lists it. */
  protected readonly deleteMap = async (): Promise<void> => {
    const map = this.map();
    if (!map) {
      return;
    }
    try {
      await this.api.delete(this.campaignId(), map.id);
    } catch (err) {
      throw new Error(mapErrorMessage(err, 'apagar o mapa'));
    }
    await this.router.navigate(['/campanhas', this.campaignId()]);
  };

  protected async toggleReveal(): Promise<void> {
    const map = this.map();
    if (!map || this.busy()) {
      return;
    }
    this.busy.set(true);
    this.notice.set(null);
    try {
      this.state.setMap(await this.api.setRevealed(this.campaignId(), map.id, !map.revealed));
      void this.reloadMaps();
    } catch (err) {
      this.notice.set(mapErrorMessage(err, 'mudar o mapa'));
    } finally {
      this.busy.set(false);
    }
  }

  protected changeImage(): void {
    const map = this.map();
    if (!map) {
      return;
    }
    const current = map.image ? { id: map.image.id, name: map.image.name } : null;
    openImagePicker(
      this.dialog,
      {
        campaignId: this.campaignId(),
        title: 'Trocar a imagem do mapa',
        confirmLabel: 'Usar esta imagem',
        current,
        currentTag: 'Imagem do mapa',
        currentNote: 'Essa já é a imagem do mapa.',
        note: () =>
          'Os pontos e tokens ficam onde estão: as posições são relativas ao tamanho da imagem.',
        hiddenMapImages: new Map(),
        emptyError: 'Escolha uma imagem para o mapa.',
        submit: async (image) => {
          const saved = await this.api.update(this.campaignId(), map.id, map.revision, {
            imageId: image.id,
          });
          this.state.setMap(saved);
        },
        errorMessage: (err) => mapErrorMessage(err, 'trocar a imagem'),
      },
      this.injector,
      this.phone(),
    );
  }
}
