import { Component, inject } from '@angular/core';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatSliderModule } from '@angular/material/slider';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MapService } from './map.service';

@Component({
  selector: 'app-map-config-panel',
  standalone: true,
  imports: [MatCheckboxModule, MatSliderModule, MatButtonModule, MatIconModule],
  template: `
    <div class="config-panel">
      <h3 class="panel-title">Camadas</h3>

      <div class="checkbox-group">
        <mat-checkbox (change)="toggleGrid('hex')">
          Grade Hexagonal
        </mat-checkbox>
        <mat-checkbox (change)="toggleGrid('square')">
          Grade Quadrada
        </mat-checkbox>
        <mat-checkbox (change)="toggleFogOfWar()">
          Névoa da Guerra
        </mat-checkbox>
        <mat-checkbox (change)="toggleMarkers()">
          Marcadores do Mestre
        </mat-checkbox>
        <mat-checkbox (change)="toggleSubmapPins()">
          Pins de Submapa
        </mat-checkbox>
      </div>

      <div class="slider-section">
        <label class="slider-label">Opacidade da Névoa</label>
        <mat-slider min="0" max="1" step="0.1">
          <input matSliderThumb (valueChange)="setFogOpacity($event)" />
        </mat-slider>
      </div>

      <button
        mat-stroked-button
        class="reset-btn"
        (click)="resetFog()"
      >
        <mat-icon>refresh</mat-icon>
        Reset Fog of War
      </button>
    </div>
  `,
  styles: `
    :host {
      position: absolute;
      right: 16px;
      top: 64px;
      z-index: 10;
      width: 240px;
    }

    .config-panel {
      background: #1e1e1e;
      border: 1px solid rgba(255, 255, 255, 0.12);
      border-radius: 12px;
      padding: 16px;
      display: flex;
      flex-direction: column;
      gap: 12px;
      box-shadow: 0 4px 24px rgba(0, 0, 0, 0.4);
    }

    .panel-title {
      margin: 0;
      font-size: 0.875rem;
      font-weight: 500;
      color: rgba(255, 255, 255, 0.87);
      text-transform: uppercase;
      letter-spacing: 0.5px;
    }

    .checkbox-group {
      display: flex;
      flex-direction: column;
      gap: 8px;
    }

    :host ::ng-deep .checkbox-group .mdc-checkbox {
      flex-shrink: 0;
    }

    .slider-section {
      display: flex;
      flex-direction: column;
      gap: 4px;
    }

    .slider-label {
      font-size: 0.75rem;
      color: rgba(255, 255, 255, 0.6);
    }

    .reset-btn {
      width: 100%;
    }
  `,
})
export class MapConfigPanelComponent {
  private readonly mapService = inject(MapService);

  toggleGrid(type: 'hex' | 'square'): void {
    this.mapService.toggleGrid(type);
  }

  toggleFogOfWar(): void {
    this.mapService.toggleFogOfWar();
  }

  toggleMarkers(): void {
    this.mapService.toggleMarkers();
  }

  toggleSubmapPins(): void {
    this.mapService.toggleSubmapPins();
  }

  setFogOpacity(value: number | null): void {
    if (value !== null) {
      this.mapService.setFogOpacity(value);
    }
  }

  resetFog(): void {
    this.mapService.resetFogOfWar();
  }
}
