import {
  AfterViewInit,
  Component,
  ElementRef,
  inject,
  OnDestroy,
  signal,
  viewChild,
} from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { StoreService } from '../../core/store/store.service';
import { MapService } from '../map/map.service';
import { DungeonService } from '../map/dungeon.service';
import { SessionBroadcastService } from './session-broadcast.service';
import type { MapData } from '../../core/models/map';
import type { SessionState } from '../../core/models/session';

@Component({
  selector: 'app-player-view',
  standalone: true,
  imports: [MatButtonModule, MatIconModule],
  template: `
    <div class="presentation">
      <div #mapContainer class="map-container"></div>

      @if (!activeMapId()) {
        <div class="waiting">
          <mat-icon class="waiting-icon">cast</mat-icon>
          <p class="waiting-text">Aguardando o mestre escolher um mapa...</p>
        </div>
      } @else {
        <div class="map-label">{{ activeMapName() }}</div>
      }

      <button
        mat-icon-button
        class="fullscreen-btn"
        (click)="toggleFullscreen()"
        aria-label="Alternar tela cheia"
      >
        <mat-icon>fullscreen</mat-icon>
      </button>
    </div>
  `,
  styles: `
    :host {
      display: block;
      width: 100vw;
      height: 100vh;
      overflow: hidden;
      background: #0a0a12;
    }

    .presentation {
      position: relative;
      width: 100%;
      height: 100%;
    }

    .map-container {
      position: absolute;
      inset: 0;
      width: 100%;
      height: 100%;
    }

    .waiting {
      position: absolute;
      inset: 0;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      gap: 16px;
      background: #0a0a12;
      color: rgba(255, 255, 255, 0.55);
      pointer-events: none;
    }

    .waiting-icon {
      font-size: 3.5rem;
      width: 3.5rem;
      height: 3.5rem;
      opacity: 0.5;
    }

    .waiting-text {
      margin: 0;
      font-size: 1.1rem;
    }

    .map-label {
      position: absolute;
      top: 16px;
      left: 20px;
      z-index: 5;
      padding: 6px 14px;
      border-radius: 12px;
      background: rgba(0, 0, 0, 0.55);
      color: rgba(255, 255, 255, 0.9);
      font-size: 0.9rem;
      font-weight: 500;
      letter-spacing: 0.3px;
      pointer-events: none;
    }

    .fullscreen-btn {
      position: absolute;
      top: 12px;
      right: 12px;
      z-index: 5;
      opacity: 0.15;
      transition: opacity 0.2s ease;
    }

    .fullscreen-btn:hover {
      opacity: 0.9;
    }
  `,
})
export class PlayerViewComponent implements AfterViewInit, OnDestroy {
  private readonly route = inject(ActivatedRoute);
  private readonly mapService = inject(MapService);
  private readonly dungeonService = inject(DungeonService);
  private readonly broadcast = inject(SessionBroadcastService);
  private readonly sessionStore = inject<StoreService<SessionState>>(StoreService);
  private readonly mapStore = inject<StoreService<MapData>>(StoreService);

  private readonly mapContainer =
    viewChild.required<ElementRef<HTMLElement>>('mapContainer');

  readonly activeMapId = signal<string | null>(null);
  readonly activeMapName = signal('');

  private sessionId: string | null = null;
  private maps: MapData[] = [];
  private renderedMapId: string | null = null;
  private subscriptions: { unsubscribe: () => void }[] = [];
  private broadcastCleanup: (() => void) | null = null;

  async ngAfterViewInit(): Promise<void> {
    this.sessionId = this.route.snapshot.paramMap.get('id');

    await this.mapService.initialize(this.mapContainer().nativeElement, {
      zoom: 0,
      center: [0, 0],
    });
    this.mapService.hideBaseLayer();

    this.subscriptions.push(
      this.mapStore.getAll('maps').subscribe((maps) => {
        this.maps = maps;
        const activeId = this.activeMapId();
        if (activeId) this.applyActiveMap(activeId);
      }),
    );

    if (this.sessionId) {
      this.subscriptions.push(
        this.sessionStore.get('sessions', this.sessionId).subscribe((session) => {
          this.applyActiveMap(session?.activeMapId ?? null);
        }),
      );
    }

    this.broadcastCleanup = this.broadcast.subscribe((message) => {
      if (message.sessionId !== this.sessionId) return;
      this.applyActiveMap(message.activeMapId);
    });

    const last = this.broadcast.lastPublished();
    if (last && last.sessionId === this.sessionId) {
      this.applyActiveMap(last.activeMapId);
    }
  }

  ngOnDestroy(): void {
    this.subscriptions.forEach((s) => s.unsubscribe());
    this.broadcastCleanup?.();
    this.dungeonService.destroy();
    this.mapService.destroy();
  }

  toggleFullscreen(): void {
    if (!document.fullscreenElement) {
      document.documentElement.requestFullscreen().catch(() => {});
    } else {
      document.exitFullscreen().catch(() => {});
    }
  }

  private applyActiveMap(mapId: string | null): void {
    this.activeMapId.set(mapId);

    if (!mapId) {
      this.activeMapName.set('');
      this.renderedMapId = null;
      return;
    }

    const map = this.maps.find((m) => m.id === mapId);
    this.activeMapName.set(map?.name ?? '');
    if (map && map.id !== this.renderedMapId) {
      this.renderedMapId = map.id;
      void this.renderMap(map);
    }
  }

  private async renderMap(map: MapData): Promise<void> {
    this.renderedMapId = map.id;
    const width = map.width || 1024;
    const height = map.height || 768;

    if (map.backgroundImage) {
      await this.mapService.setImageBackground(map.backgroundImage, width, height);
    } else {
      await this.mapService.clearImageBackground();
    }

    await this.mapService.setGridConfig(map.grid);
    await this.mapService.setGridVisible(false);

    await this.mapService.renderPois(map.markers ?? []);
    await this.mapService.showPoisLayer();
    await this.mapService.renderSubmapPins(map.submaps ?? []);
    await this.mapService.showSubmapPinsLayer();

    await this.dungeonService.configure({
      mapId: map.id,
      extent: [0, 0, width, height],
      cellSize: map.grid?.cellSize ?? 50,
      columns: map.grid?.columns ?? 24,
      rows: map.grid?.rows ?? 18,
    });
    await this.dungeonService.load(map.dungeon);
    await this.dungeonService.show();
  }
}
