import {
  AfterViewInit,
  Component,
  ElementRef,
  inject,
  OnDestroy,
  signal,
  viewChild,
} from '@angular/core';
import { Router, ActivatedRoute } from '@angular/router';
import { Subscription } from 'rxjs';
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
import { ConfirmDialogComponent, ConfirmDialogData } from '../../shared';
import { PoiDialogComponent, PoiDialogData, PoiDialogResult } from './poi-dialog.component';
import { MapFormDialogComponent, MapFormDialogData } from './map-form-dialog.component';
import type { MapData, MapMarker } from '../../core/models/map';
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
      icon="map"
      [breadcrumbs]="breadcrumbs"
    />

    <div class="map-stage">
      <!-- 2D OpenLayers container (always in DOM — never conditionally hidden) -->
      <div #mapContainer class="map-container"></div>

      <!-- 2.5D Three.js container (hidden by default) -->
      <div #threeContainer class="three-container" style="display: none"></div>

      @if (loading()) {
        <div class="overlay">
          <app-loading-spinner [isLoading]="true" message="Carregando mapa..." />
        </div>
      } @else if (error()) {
        <div class="overlay">
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
            [class.active]="pinPlacementMode()"
            (click)="togglePinPlacement()"
          >
            <mat-icon>add_location</mat-icon>
            Adicionar Ponto
          </button>
          @if (pinPlacementMode()) {
            <span class="pin-hint">Clique no mapa para posicionar o ponto</span>
          }
          @if (currentMapData) {
            <span class="toolbar-spacer"></span>
            <button mat-stroked-button (click)="openEditMapDialog()">
              <mat-icon>edit</mat-icon>
              Editar Mapa
            </button>
            <button mat-stroked-button color="warn" (click)="deleteMap()">
              <mat-icon>delete</mat-icon>
              Excluir
            </button>
          }
        </div>

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
    </div>
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

    .map-stage {
      position: relative;
      flex: 1;
      min-height: 0;
    }

    .toolbar-row {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: 8px;
      row-gap: 8px;
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

    .toolbar-spacer {
      flex: 1;
    }

    .map-container,
    .three-container {
      position: absolute;
      inset: 0;
      width: 100%;
      height: 100%;
    }

    .overlay {
      position: absolute;
      inset: 0;
      z-index: 20;
      display: flex;
      align-items: center;
      justify-content: center;
      background: rgba(10, 10, 18, 0.6);
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

    @media (max-width: 480px) {
      .config-btn {
        top: 8px;
        right: 8px;
      }
      .fullscreen-btn {
        bottom: 16px;
        right: 16px;
      }
      .toggle3d-btn {
        bottom: 16px;
        right: 80px;
      }
      .mode-label {
        bottom: 24px;
        right: 144px;
      }
    }

    :host ::ng-deep app-page-header {
      flex-shrink: 0;
    }
  `,
})
export class MapViewComponent implements AfterViewInit, OnDestroy {
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
  protected readonly pinPlacementMode = signal(false);
  protected mapTitle = 'Mapa';
  protected breadcrumbs: BreadcrumbItem[] = [];
  protected parentMapId: string | null = null;
  protected currentMapData: MapData | null = null;
  readonly loading = signal(true);
  readonly error = signal<string | null>(null);

  private mapId: string | null = null;
  private routeSub: Subscription | null = null;
  private resizeObserver: ResizeObserver | null = null;
  private clickUnregister: (() => void) | null = null;
  private featureUnregister: (() => void) | null = null;

  async ngAfterViewInit() {
    this.loading.set(true);
    this.error.set(null);

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

    // Setup click handlers (pin placement + feature clicks)
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

    // Reload map data whenever the route id changes (initial load included)
    this.routeSub = this.route.paramMap.subscribe((params) => {
      this.mapId = params.get('id');
      void this.loadMapData();
    });
  }

  ngOnDestroy() {
    this.clickUnregister?.();
    this.featureUnregister?.();
    this.routeSub?.unsubscribe();
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
    this.pinPlacementMode.update((v) => !v);
  }

  protected goToParent(): void {
    if (this.parentMapId) {
      this.router.navigate(['/mapa', this.parentMapId]);
    }
  }

  protected goBack(): void {
    this.router.navigate(['/mapa']);
  }

  protected openEditMapDialog(): void {
    if (!this.currentMapData) return;

    const ref = this.dialog.open(MapFormDialogComponent, {
      width: '520px',
      maxWidth: '95vw',
      data: { map: this.currentMapData } as MapFormDialogData,
    });
    ref.afterClosed().subscribe((saved) => {
      if (saved) {
        void this.loadMapData();
      }
    });
  }

  protected deleteMap(): void {
    if (!this.currentMapData) return;

    const ref = this.dialog.open(ConfirmDialogComponent, {
      width: '420px',
      data: {
        title: 'Excluir Mapa',
        message: `Tem certeza que deseja excluir "${this.currentMapData.name}"?`,
        confirmText: 'Excluir',
        cancelText: 'Cancelar',
      } as ConfirmDialogData,
    });
    ref.afterClosed().subscribe((confirmed) => {
      if (confirmed && this.currentMapData) {
        this.store.delete('maps', this.currentMapData.id);
        this.router.navigate(['/mapa']);
      }
    });
  }

  private async loadMapData(): Promise<void> {
    this.loading.set(true);

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

        // Switch base layer: custom image or OSM tiles
        if (mapData.backgroundImage) {
          await this.mapService.setImageBackground(
            mapData.backgroundImage,
            mapData.width || 1024,
            mapData.height || 768,
          );
        } else {
          await this.mapService.clearImageBackground();
        }

        // Render POIs and submap pins
        await this.mapService.renderPois(mapData.markers ?? []);
        await this.mapService.showPoisLayer();
        await this.mapService.renderSubmapPins(mapData.submaps ?? []);
        await this.mapService.showSubmapPinsLayer();

        this.loading.set(false);
        return;
      }

      // Map ID provided but not found — 404
      this.currentMapData = null;
      this.mapService.setCurrentMapId(null);
      this.error.set('Mapa não encontrado');
      this.loading.set(false);
      return;
    }

    // No specific map id — back to the list
    this.currentMapData = null;
    this.mapService.setCurrentMapId(null);
    this.mapTitle = 'Mapa';
    this.breadcrumbs = [];
    this.parentMapId = null;
    this.loading.set(false);
  }

  private setupClickHandler(): void {
    // Feature clicks take priority: POI expands, submap navigates
    this.featureUnregister = this.mapService.onFeatureClick((result) => {
      if (this.pinPlacementMode()) return;

      if (result.type === 'poi' && result.marker) {
        this.openPoiDialog(result.marker);
      } else if (result.type === 'submap' && result.targetMapId) {
        this.router.navigate(['/mapa', result.targetMapId]);
      }
    });

    // Empty clicks only matter in pin placement mode
    this.clickUnregister = this.mapService.onMapClick((coords) => {
      if (!this.pinPlacementMode()) return;

      this.pinPlacementMode.set(false);

      const ref = this.dialog.open(PoiDialogComponent, {
        data: {
          x: coords[0],
          y: coords[1],
          availableMaps: this.mapService
            .getAllMaps()
            .filter((m) => m.id !== this.mapId),
          currentMapId: this.mapId,
        } as PoiDialogData,
        width: '460px',
        maxWidth: '95vw',
      });

      ref.afterClosed().subscribe((result: PoiDialogResult) => {
        if (result?.action === 'save' && result.marker) {
          void this.mapService.addPoi(result.marker);
        }
      });
    });
  }

  private openPoiDialog(marker: MapMarker): void {
    const ref = this.dialog.open(PoiDialogComponent, {
      data: {
        x: marker.x,
        y: marker.y,
        existing: marker,
        availableMaps: this.mapService
          .getAllMaps()
          .filter((m) => m.id !== this.mapId),
        currentMapId: this.mapId,
      } as PoiDialogData,
      width: '460px',
      maxWidth: '95vw',
    });

    ref.afterClosed().subscribe((result: PoiDialogResult) => {
      if (result?.action === 'save' && result.marker) {
        void this.mapService.updatePoi(result.marker);
      } else if (result?.action === 'delete') {
        void this.mapService.deletePoi(marker.id);
      } else if (result?.action === 'open' && marker.targetMapId) {
        this.router.navigate(['/mapa', marker.targetMapId]);
      }
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

    const sampleMarkers: Marker3D[] = [];
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
