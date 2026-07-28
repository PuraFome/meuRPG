import { Component, ElementRef, inject, OnDestroy, OnInit, viewChild } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MapService } from './map.service';

@Component({
  selector: 'app-map-view',
  standalone: true,
  imports: [MatButtonModule, MatIconModule],
  template: `
    <div #mapContainer class="map-container"></div>
    <button
      mat-fab
      class="fullscreen-btn"
      (click)="toggleFullscreen()"
      aria-label="Alternar tela cheia"
    >
      <mat-icon>fullscreen</mat-icon>
    </button>
  `,
  styles: `
    :host {
      display: block;
      width: 100%;
      height: calc(100vh - 64px - 48px);
      position: relative;
    }

    .map-container {
      width: 100%;
      height: 100%;
    }

    .fullscreen-btn {
      position: absolute;
      bottom: 24px;
      right: 24px;
      z-index: 10;
    }
  `,
})
export class MapViewComponent implements OnInit, OnDestroy {
  private readonly mapService = inject(MapService);
  private readonly mapContainer = viewChild.required<ElementRef<HTMLElement>>('mapContainer');

  async ngOnInit() {
    await this.mapService.initialize(this.mapContainer().nativeElement, {
      zoom: 10,
      center: [-46.6333, -23.5505],
    });
  }

  ngOnDestroy() {
    this.mapService.destroy();
  }

  toggleFullscreen(): void {
    this.mapService.toggleFullscreen(this.mapContainer().nativeElement);
  }
}
