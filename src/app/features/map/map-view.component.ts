import { Component, ElementRef, inject, OnDestroy, OnInit, signal, viewChild } from '@angular/core';
import { Router, ActivatedRoute } from '@angular/router';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { MapService } from './map.service';
import { MapThreeService } from './map-three.service';
import type { Marker3D } from './map-three.service';
import { MapConfigPanelComponent } from './map-config-panel.component';
import { PageHeaderComponent, BreadcrumbItem } from '../../shared/components/page-header.component';
import { LoadingSpinnerComponent } from '../../shared/components/loading-spinner.component';
import { EmptyStateComponent } from '../../shared/components/empty-state.component';
import { SubmapPinDialogComponent } from './submap-pin-dialog.component';
import type { MapData } from '../../core/models/map';
import { StoreService } from '../../core/store/store.service';

@Component({
  selector: 'app-map-view',
  standalone: true,
  imports: [
    MatButtonModule,
    MatIconModule,
    MapConfigPanelComponent,
    PageHeaderComponent,
    LoadingSpinnerComponent,
    EmptyStateComponent,
  ],
  template: `
    <!-- Page header with breadcrumbs -->
    <app-page-header
      [title]="mapTitle"
      [breadcrumbs]="breadcrumbs"
    />

    @if (loading()) {
      <app-loading-spinner [isLoading]="true" message="Carregando mapa..." />
    } @else if (error()) {
      <div class="error-state">
        <app-empty-state
          icon="map"
          [message]="error()!"
          actionLabel="Voltar"
          (action)="goBack()"
        />
      </div>
    } @else {
      <!-- Toolbar -->
      <div class="toolbar-row">
        @if (parentMapId) {
          <button mat-stroked-button (click)="goToParent()">
            <mat-icon>arrow_back</mat-icon>
            Voltar ao mapa pai
          </button>
        }
        <button
          mat-stroked-button
          [class.active]="pinPlacementMode"
          (click)="togglePinPlacement()"
        >
          <mat-icon>push_pin</mat-icon>
          Adicionar Pin
        </button>
        @if (pinPlacementMode) {
          <span class="pin-hint">Clique no mapa para posicionar o pin</span>
        }
      </div>

      <!-- 2D OpenLayers container -->
      <div #mapContainer class="map-container"></div>

      <!-- 2.5D Three.js container (hidden by default) -->
      <div #threeContainer class="three-container" style="display: none"></div>

      <!-- Layer config panel (floating) -->
      @if (showConfigPanel) {
        <app-map-config-panel />
      }

      <!-- Layer config toggle -->
      <button
        mat-fab
        class="fab-btn config-btn"
        (click)="showConfigPanel = !showConfigPanel"
        aria-label="Configurações do mapa"
      >
        <mat-icon>layers</mat-icon>
      </button>

      <!-- Fullscreen button -->
      <button
        mat-fab
        class="fab-btn fullscreen-btn"
        (click)="toggleFullscreen()"
        aria-label="Alternar tela cheia"
      >
        <mat-icon>fullscreen</mat-icon>
      </button>

      <!-- 2D / 2.5D toggle button -->
      <button
        mat-fab
        class="fab-btn toggle3d-btn"
        (click)="toggle3D()"
        aria-label="Alternar 2D/3D"
      >
        <mat-icon>{{ is3D ? 'map' : 'view_in_ar' }}</mat-icon>
      </button>

      <!-- Mode label -->
      <span class="mode-label">{{ is3D ? '2.5D' : '2D' }}</span>
    }
  `,
  styles: `
    :host {
      display: flex;
      flex-direction: column;
      width: 100%;
      height: calc(100vh - 64px - 48px);
      position: relative;
      overflow: hidden;
    }

    .toolbar-row {
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 8px 16px;
      background: rgba(0,0,0,0.3);
      z-index: 10;
      flex-shrink: 0;
    }

    .toolbar-row button.active {
      background: rgba(124,77,255,0.25);
      border-color: #7c4dff;
    }

    .pin-hint {
      font-size: 0.8125rem;
      color: rgba(255,255,255,0.6);
      font-style: italic;
    }

    .map-container,
    .three-container {
      flex: 1;
      width: 100%;
      position: relative;
    }

    .fab-btn {
      position: absolute;
      z-index: 10;
    }

    .fullscreen-btn {
      bottom: 24px;
      right: 24px;
    }

    .toggle3d-btn {
      bottom: 24px;
      right: 88px;
    }

    .config-btn {
      top: 16px;
      right: 16px;
    }

    .mode-label {
      position: absolute;
      bottom: 32px;
      right: 152px;
      z-index: 10;
      color: rgba(255, 255, 255, 0.85);
      font-size: 0.8125rem;
      font-weight: 500;
      letter-spacing: 0.5px;
      background: rgba(0, 0, 0, 0.45);
      padding: 4px 12px;
      border-radius: 12px;
      backdrop-filter: blur(4px);
      user-select: none;
      pointer-events: none;
    }

    .error-state {
      display: flex;
      justify-content: center;
      align-items: center;
      flex: 1;
      min-height: 300px;
    }

    :host ::ng-deep app-page-header {
      flex-shrink: 0;
    }
  `,
})
export class MapViewComponent implements OnInit, OnDestroy {
  private readonly router = inject(Router);
  private readonly route = inject(ActivatedRoute);
  private readonly dialog = inject(MatDialog);
  private readonly mapService = inject(MapService);
  private readonly mapThreeService = inject(MapThreeService);
  private readonly store = inject(StoreService<MapData>);

  private readonly mapContainer =
    viewChild.required<ElementRef<HTMLElement>>('mapContainer');
  private readonly threeContainer =
    viewChild.required<ElementRef<HTMLElement>>('threeContainer');

  protected is3D = false;
  protected showConfigPanel = false;
  protected pinPlacementMode = false;
  protected mapTitle = 'Mapa';
  protected breadcrumbs: BreadcrumbItem[] = [];
  protected parentMapId: string | null = null;
  readonly loading = signal(true);
  readonly error = signal<string | null>(null);

  private mapId: string | null = null;
  private resizeObserver: ResizeObserver | null = null;
  private clickUnregister: (() => void) | null = null;
  private currentMapData: MapData | null = null;

  async ngOnInit() {
    this.loading.set(true);
    this.error.set(null);

    // Read map id from route params
    this.mapId = this.route.snapshot.paramMap.get('id');

    // Initialise the 2D OpenLayers map
    try {
      await this.mapService.initialize(this.mapContainer().nativeElement, {
        zoom: 10,
        center: [-46.6333, -23.5505],
      });
    } catch (err) {
      console.error('Falha ao inicializar mapa:', err);
      this.error.set('Erro ao carregar mapa');
      this.loading.set(false);
      return;
    }

    // Load map data and set up submap pins
    await this.loadMapData();

    // Setup click handler for pin placement
    this.setupClickHandler();

    // Watch container resize so the 3D renderer stays in sync
    this.resizeObserver = new ResizeObserver(() => {
      if (this.is3D) {
        this.mapThreeService.resize();
      }
    });
    const parentEl = this.mapContainer()?.nativeElement?.parentElement;
    if (parentEl) {
      this.resizeObserver.observe(parentEl);
    }
  }

  ngOnDestroy() {
    this.clickUnregister?.();
    this.mapService.destroy();
    this.mapThreeService.destroy();
    this.resizeObserver?.disconnect();
  }

  toggleFullscreen(): void {
    this.mapService.toggleFullscreen(this.mapContainer().nativeElement);
  }

  protected async toggle3D(): Promise<void> {
    if (this.is3D) {
      this.switchTo2D();
    } else {
      await this.switchTo3D();
    }
  }

  protected togglePinPlacement(): void {
    this.pinPlacementMode = !this.pinPlacementMode;
  }

  protected goToParent(): void {
    if (this.parentMapId) {
      this.router.navigate(['/mapa', this.parentMapId]);
    }
  }

  protected goBack(): void {
    this.router.navigate(['/mapa']);
  }

  private async loadMapData(): Promise<void> {
    if (this.mapId) {
      const mapData = this.store.snapshot('maps').find((m) => m.id === this.mapId);
      if (mapData) {
        this.currentMapData = mapData;
        this.mapService.setCurrentMapId(this.mapId);
        this.mapTitle = mapData.name;

        // Build breadcrumb hierarchy
        const hierarchy = this.mapService.getMapHierarchy(this.mapId);
        this.breadcrumbs = hierarchy.map((m, i) => ({
          label: m.name,
          route: i < hierarchy.length - 1 ? `/mapa/${m.id}` : undefined,
        }));

        // Find parent map
        this.parentMapId = this.mapService.getParentMapId(this.mapId);

        // Render submap pins
        await this.mapService.renderSubmapPins(mapData.submaps ?? []);

        // Ensure submap pins layer is visible
        await this.mapService.showSubmapPinsLayer();

        this.loading.set(false);
        return;
      }

      // Map ID provided but not found — 404
      this.error.set('Mapa não encontrado');
      this.loading.set(false);
      return;
    }

    // No specific map id — show default state
    this.mapService.setCurrentMapId(null);
    this.mapTitle = 'Mapa';
    this.breadcrumbs = [];
    this.parentMapId = null;
    this.loading.set(false);
  }

  private setupClickHandler(): void {
    this.clickUnregister = this.mapService.onMapClick((coords) => {
      if (!this.pinPlacementMode) return;

      this.pinPlacementMode = false;

      const dialogRef = this.dialog.open(SubmapPinDialogComponent, {
        data: { x: coords[0], y: coords[1] },
        width: '420px',
      });

      dialogRef.afterClosed().subscribe((result) => {
        if (result) {
          this.mapService.addSubmapPin(result);
        }
      });
    });
  }

  // ── 2D / 2.5D switching ──

  private async switchTo3D(): Promise<void> {
    const threeContainerEl = this.threeContainer().nativeElement;
    const mapContainerEl = this.mapContainer().nativeElement;

    const olCanvas = this.mapService.getCanvas();
    const center = this.mapService.getCenter();
    const zoom = this.mapService.getZoom();

    await this.mapThreeService.init(threeContainerEl);
    await this.mapThreeService.activate(olCanvas, center, zoom);

    this.mapThreeService.renderGrid();

    const sampleMarkers: Marker3D[] = [
      { lon: -46.6333, lat: -23.5505, label: 'Centro', color: '#ff6b6b' },
      { lon: -46.6, lat: -23.55, label: 'Marker 1', color: '#4ecdc4' },
      { lon: -46.65, lat: -23.52, label: 'Marker 2', color: '#ffe66d' },
    ];
    this.mapThreeService.renderMarkers(sampleMarkers);

    mapContainerEl.style.display = 'none';
    threeContainerEl.style.display = 'block';

    requestAnimationFrame(() => this.mapThreeService.resize());

    this.is3D = true;
  }

  private switchTo2D(): void {
    const threeContainerEl = this.threeContainer().nativeElement;
    const mapContainerEl = this.mapContainer().nativeElement;

    threeContainerEl.style.display = 'none';
    mapContainerEl.style.display = 'block';

    this.mapThreeService.deactivate();
    this.is3D = false;
  }
}
