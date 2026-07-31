import { Component, inject } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { trigger, transition, style, animate } from '@angular/animations';
import { MatButtonModule } from '@angular/material/button';
import { MatDialogModule, MatDialogRef, MAT_DIALOG_DATA } from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MapService } from './map.service';
import type { MapData, SubmapPin } from '../../core/models/map';

export interface SubmapPinDialogData {
  x: number;
  y: number;
}

const PIN_COLORS = [
  '#7c4dff', '#e53935', '#ff6d00', '#ffd600', '#00c853',
  '#2979ff', '#00bcd4', '#ff4081', '#6d4c41', '#78909c',
];

const PIN_ICONS = [
  'pin_drop', 'location_on', 'flag', 'star', 'circle',
  'room', 'navigation', 'place', 'explore', 'my_location',
];

@Component({
  selector: 'app-submap-pin-dialog',
  standalone: true,
  imports: [
    FormsModule,
    MatButtonModule,
    MatDialogModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatSelectModule,
  ],
  template: `
    <h2 mat-dialog-title>Adicionar Pin</h2>
    <mat-dialog-content>
      <div class="pin-form">
        <mat-form-field appearance="fill" class="full-width">
          <mat-label>Nome do Pin</mat-label>
          <input matInput [(ngModel)]="label" placeholder="Ex: Entrada da Taverna" />
        </mat-form-field>

        <mat-form-field appearance="fill" class="full-width">
          <mat-label>Mapa de destino</mat-label>
          <mat-select [(ngModel)]="selectedMapId">
            @for (map of availableMaps; track map.id) {
              <mat-option [value]="map.id">{{ map.name }}</mat-option>
            }
          </mat-select>
        </mat-form-field>

        <label class="section-label">Cor do pin</label>
        <div class="color-options">
          @for (color of colors; track color) {
            <button
              type="button"
              class="color-swatch"
              [class.selected]="selectedColor === color"
              [style.background]="color"
              (click)="selectedColor = color"
              [attr.aria-label]="'Cor ' + color"
            ></button>
          }
        </div>

        <label class="section-label">Ícone</label>
        <div class="icon-options">
          @for (icon of icons; track icon) {
            <button
              type="button"
              class="icon-option"
              [class.selected]="selectedIcon === icon"
              (click)="selectedIcon = icon"
              [attr.aria-label]="'Ícone ' + icon"
            >
              <mat-icon>{{ icon }}</mat-icon>
            </button>
          }
        </div>
      </div>
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <button mat-button (click)="onCancel()">Cancelar</button>
      <button
        mat-raised-button
        color="primary"
        [disabled]="!label || !selectedMapId"
        (click)="onConfirm()"
      >
        Adicionar
      </button>
    </mat-dialog-actions>
  `,
  styles: [
    `
      .pin-form {
        display: flex;
        flex-direction: column;
        gap: 16px;
        min-width: 360px;
        padding: 8px 0;
      }
      .full-width {
        width: 100%;
      }
      .section-label {
        font-size: 0.8125rem;
        font-weight: 500;
        color: rgba(255,255,255,0.7);
        margin-bottom: 4px;
      }
      .color-options {
        display: flex;
        gap: 8px;
        flex-wrap: wrap;
      }
      .color-swatch {
        width: 32px;
        height: 32px;
        border-radius: 50%;
        border: 2px solid transparent;
        cursor: pointer;
        transition: transform 0.15s, border-color 0.15s;
      }
      .color-swatch:hover {
        transform: scale(1.15);
      }
      .color-swatch.selected {
        border-color: #ffffff;
        transform: scale(1.2);
      }
      .icon-options {
        display: flex;
        gap: 8px;
        flex-wrap: wrap;
      }
      .icon-option {
        width: 40px;
        height: 40px;
        border-radius: 8px;
        border: 2px solid transparent;
        background: rgba(255,255,255,0.08);
        cursor: pointer;
        display: flex;
        align-items: center;
        justify-content: center;
        transition: background 0.15s, border-color 0.15s;
      }
      .icon-option:hover {
        background: rgba(255,255,255,0.16);
      }
      .icon-option.selected {
        border-color: #7c4dff;
        background: rgba(124,77,255,0.2);
      }
      .icon-option .material-symbols-outlined {
        font-size: 22px;
        color: rgba(255,255,255,0.87);
      }
    `,
  ],
  animations: [
    trigger('fadeSlide', [
      transition(':enter', [
        style({ opacity: 0, transform: 'translateY(-12px)' }),
        animate('250ms ease-out', style({ opacity: 1, transform: 'translateY(0)' })),
      ]),
    ]),
  ],
  host: { '[@fadeSlide]': '' },
})
export class SubmapPinDialogComponent {
  private readonly dialogRef = inject(MatDialogRef<SubmapPinDialogComponent, SubmapPin>);
  private readonly data = inject<SubmapPinDialogData>(MAT_DIALOG_DATA);
  private readonly mapService = inject(MapService);

  protected readonly colors = PIN_COLORS;
  protected readonly icons = PIN_ICONS;

  protected label = '';
  protected selectedMapId: string | null = null;
  protected selectedColor = PIN_COLORS[0];
  protected selectedIcon = PIN_ICONS[0];

  protected availableMaps: MapData[] = [];

  constructor() {
    this.availableMaps = this.mapService.getAllMaps().filter((m) => m.id !== this.mapService.getCurrentMapId());
  }

  onCancel(): void {
    this.dialogRef.close();
  }

  onConfirm(): void {
    if (!this.label || !this.selectedMapId) return;

    const pin: SubmapPin = {
      id: crypto.randomUUID(),
      x: this.data.x,
      y: this.data.y,
      targetMapId: this.selectedMapId,
      label: this.label,
      icon: this.selectedIcon,
      color: this.selectedColor,
    };
    this.dialogRef.close(pin);
  }
}
