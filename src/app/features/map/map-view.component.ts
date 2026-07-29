import { Component, ElementRef, inject, OnDestroy, OnInit, viewChild } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MapService } from './map.service';
import { MapThreeService } from './map-three.service';
import type { Marker3D } from './map-three.service';
import { MapConfigPanelComponent } from './map-config-panel.component';

@Component({
  selector: 'app-map-view',
  standalone: true,
  imports: [MatButtonModule, MatIconModule, MapConfigPanelComponent],
  template: `
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
  `,
  styles: `
    :host {
      display: block;
      width: 100%;
      height: calc(100vh - 64px - 48px);
      position: relative;
      overflow: hidden;
    }

    .map-container,
    .three-container {
      width: 100%;
      height: 100%;
      position: absolute;
      inset: 0;
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
      font-size: 13px;
      font-weight: 500;
      letter-spacing: 0.5px;
      background: rgba(0, 0, 0, 0.45);
      padding: 4px 12px;
      border-radius: 12px;
      backdrop-filter: blur(4px);
      user-select: none;
      pointer-events: none;
    }
  `,
})
export class MapViewComponent implements OnInit, OnDestroy {
  private readonly mapService = inject(MapService);
  private readonly mapThreeService = inject(MapThreeService);

  private readonly mapContainer =
    viewChild.required<ElementRef<HTMLElement>>('mapContainer');
  private readonly threeContainer =
    viewChild.required<ElementRef<HTMLElement>>('threeContainer');

  protected is3D = false;
  protected showConfigPanel = false;

  private resizeObserver: ResizeObserver | null = null;

  async ngOnInit() {
    // Initialise the 2D OpenLayers map
    await this.mapService.initialize(this.mapContainer().nativeElement, {
      zoom: 10,
      center: [-46.6333, -23.5505],
    });

    // Watch container resize so the 3D renderer stays in sync
    this.resizeObserver = new ResizeObserver(() => {
      if (this.is3D) {
        this.mapThreeService.resize();
      }
    });
    this.resizeObserver.observe(this.mapContainer().nativeElement.parentElement!);
  }

  ngOnDestroy() {
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

  // ── 2D / 2.5D switching ──

  private async switchTo3D(): Promise<void> {
    const threeContainerEl = this.threeContainer().nativeElement;
    const mapContainerEl = this.mapContainer().nativeElement;

    // Capture current OL view state
    const olCanvas = this.mapService.getCanvas();
    const center = this.mapService.getCenter();
    const zoom = this.mapService.getZoom();

    // Init Three.js scene (lazy — dynamic import happens here)
    await this.mapThreeService.init(threeContainerEl);

    // Activate 3D view with the OL snapshot as texture
    await this.mapThreeService.activate(olCanvas, center, zoom);

    // Render a grid and a few sample markers to demonstrate 2.5D depth
    this.mapThreeService.renderGrid();

    const sampleMarkers: Marker3D[] = [
      { lon: -46.6333, lat: -23.5505, label: 'Centro', color: '#ff6b6b' },
      { lon: -46.6, lat: -23.55, label: 'Marker 1', color: '#4ecdc4' },
      { lon: -46.65, lat: -23.52, label: 'Marker 2', color: '#ffe66d' },
    ];
    this.mapThreeService.renderMarkers(sampleMarkers);

    // Swap visibility
    mapContainerEl.style.display = 'none';
    threeContainerEl.style.display = 'block';

    // Give renderer a moment to paint, then sync size
    requestAnimationFrame(() => this.mapThreeService.resize());

    this.is3D = true;
  }

  private switchTo2D(): void {
    const threeContainerEl = this.threeContainer().nativeElement;
    const mapContainerEl = this.mapContainer().nativeElement;

    // Hide 3D, show 2D
    threeContainerEl.style.display = 'none';
    mapContainerEl.style.display = 'block';

    this.mapThreeService.deactivate();
    this.is3D = false;
  }
}
