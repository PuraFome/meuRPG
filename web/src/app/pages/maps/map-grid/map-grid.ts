import { Component, DestroyRef, computed, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { Code, ConnectError } from '@connectrpc/connect';

import { Role } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import type { Map as MapMessage } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { CampaignsService } from '../../../core/campaigns/campaigns.service';
import {
  MAX_COLUMNS,
  MIN_COLUMNS,
  gridRows,
} from '../../../core/combat/combat-grid';
import { formatMeters, squaresToMeters } from '../../../core/units';
import { mapErrorMessage } from '../../../core/maps/map-errors';
import { MapsClient } from '../../../core/maps/maps-client';
import { CombatMap } from '../../../shared/combat-map/combat-map';

type Phase = 'loading' | 'ready' | 'gone' | 'forbidden' | 'error';

/** The columns a map without a grid starts with: a common battle map. */
const DEFAULT_COLUMNS = 20;

/**
 * "Grade do mapa" (E6-02, RN-21): the master says how many squares of 1,5 m
 * fit across the map's width (5 to 60 here; the server takes 4 to 200), and
 * the rows follow the image's proportions. The preview draws the grid over
 * the map with one marked square, so the master adjusts until a square is the
 * size of what is drawn (a wagon, a tree). Reached from the map's header, from
 * "Iniciar combate" and, on the session page, from a map without a grid.
 */
@Component({
  selector: 'app-map-grid',
  imports: [CombatMap, MatButtonModule, MatIconModule, MatProgressSpinnerModule, RouterLink],
  templateUrl: './map-grid.html',
  styleUrl: './map-grid.scss',
})
export class MapGrid {
  private readonly api = inject(MapsClient);
  private readonly campaigns = inject(CampaignsService);
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);

  protected readonly campaignId = signal('');
  protected readonly mapId = signal('');
  protected readonly phase = signal<Phase>('loading');
  protected readonly map = signal<MapMessage | null>(null);
  protected readonly campaignName = signal('');
  protected readonly fromSession = signal(false);
  protected readonly typed = signal(String(DEFAULT_COLUMNS));
  protected readonly saving = signal(false);
  protected readonly error = signal('');

  protected readonly Math = Math;
  protected readonly min = MIN_COLUMNS;
  protected readonly max = MAX_COLUMNS;

  /** The number in the field, or `null` while it is not a whole 5 to 60. */
  protected readonly columns = computed(() => {
    const text = this.typed().trim();
    const n = /^\d{1,3}$/.test(text) ? Number(text) : NaN;
    return n >= MIN_COLUMNS && n <= MAX_COLUMNS ? n : null;
  });
  protected readonly invalid = computed(() => this.columns() === null);
  protected readonly image = computed(() => {
    const image = this.map()?.image;
    return image ? { url: image.url, width: image.width, height: image.height } : null;
  });
  protected readonly rows = computed(() => {
    const image = this.image();
    const columns = this.columns();
    return image && columns !== null ? gridRows(columns, image.width, image.height) : null;
  });
  protected readonly squares = computed(() =>
    this.columns() !== null && this.rows() !== null ? `${this.columns()} × ${this.rows()}` : '—',
  );
  protected readonly meters = computed(() =>
    this.columns() !== null && this.rows() !== null
      ? `${formatMeters(squaresToMeters(this.columns()!))} × ${formatMeters(squaresToMeters(this.rows()!))}`
      : '—',
  );
  protected readonly hadGrid = computed(() => (this.map()?.gridColumns ?? 0) > 0);
  protected readonly backLink = computed(() =>
    this.fromSession()
      ? { path: ['/campanhas', this.campaignId(), 'sessao'], label: 'Voltar à sessão' }
      : { path: ['/campanhas', this.campaignId(), 'mapas', this.mapId()], label: 'Voltar ao mapa' },
  );

  constructor() {
    const destroyRef = inject(DestroyRef);
    this.route.queryParamMap.pipe(takeUntilDestroyed(destroyRef)).subscribe((q) => {
      this.fromSession.set(q.get('de') === 'sessao');
    });
    this.route.paramMap.pipe(takeUntilDestroyed(destroyRef)).subscribe((params) => {
      const id = params.get('id');
      const mapId = params.get('mapId');
      if (id && mapId) {
        void this.load(id, mapId);
      }
    });
  }

  private async load(campaignId: string, mapId: string): Promise<void> {
    this.campaignId.set(campaignId);
    this.mapId.set(mapId);
    this.phase.set('loading');
    try {
      const [campaign, map] = await Promise.all([
        this.campaigns.getCampaign(campaignId),
        this.api.get(campaignId, mapId),
      ]);
      if (campaign.campaign?.awaitingApproval || !map.map) {
        this.phase.set('gone');
        return;
      }
      this.campaignName.set(campaign.campaign?.name ?? '');
      if (campaign.campaign?.myRole !== Role.MASTER) {
        this.phase.set('forbidden');
        return;
      }
      this.map.set(map.map);
      this.typed.set(String(map.map.gridColumns > 0 ? map.map.gridColumns : DEFAULT_COLUMNS));
      this.phase.set('ready');
    } catch (err) {
      this.phase.set(ConnectError.from(err).code === Code.NotFound ? 'gone' : 'error');
    }
  }

  protected onType(event: Event): void {
    this.typed.set((event.target as HTMLInputElement).value);
  }

  protected step(delta: number): void {
    const now = this.columns() ?? DEFAULT_COLUMNS;
    this.typed.set(String(Math.min(MAX_COLUMNS, Math.max(MIN_COLUMNS, now + delta))));
  }

  protected async save(): Promise<void> {
    const columns = this.columns();
    if (columns === null || this.saving()) {
      return;
    }
    this.saving.set(true);
    this.error.set('');
    try {
      await this.api.setGrid(this.campaignId(), this.mapId(), columns);
      await this.router.navigate(this.backLink().path);
    } catch (err) {
      this.saving.set(false);
      this.error.set(mapErrorMessage(err, 'salvar a grade'));
    }
  }
}
