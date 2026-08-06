import { Component, inject } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import {
  MatDialogModule,
  MatDialogRef,
  MAT_DIALOG_DATA,
} from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import type { MapData, MapMarker } from '../../core/models/map';

export interface PoiDialogData {
  x: number;
  y: number;
  existing?: MapMarker;
  availableMaps: MapData[];
  currentMapId: string | null;
}

export type PoiDialogResult =
  | { action: 'save'; marker: MapMarker }
  | { action: 'delete' }
  | { action: 'open' }
  | null;

const POI_COLORS = [
  '#7c4dff', '#e53935', '#ff6d00', '#ffd600', '#00c853',
  '#2979ff', '#00bcd4', '#ff4081', '#6d4c41', '#78909c',
];

const POI_ICONS = [
  'place', 'location_on', 'flag', 'star', 'circle',
  'room', 'navigation', 'explore', 'my_location', 'castle',
];

@Component({
  selector: 'app-poi-dialog',
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
    <h2 mat-dialog-title>
      {{ existing ? 'Editar Ponto de Interesse' : 'Novo Ponto de Interesse' }}
    </h2>
    <mat-dialog-content>
      <div class="poi-form">
        <mat-form-field appearance="fill" class="full-width">
          <mat-label>Nome do local</mat-label>
          <input matInput [(ngModel)]="label" placeholder="Ex: Porto de Neverwinter" />
        </mat-form-field>

        <mat-form-field appearance="fill" class="full-width">
          <mat-label>Descrição</mat-label>
          <textarea
            matInput
            [(ngModel)]="description"
            rows="3"
            placeholder="O que os aventureiros veem ao chegar aqui?"
          ></textarea>
        </mat-form-field>

        <mat-form-field appearance="fill" class="full-width">
          <mat-label>Mapa de destino (opcional)</mat-label>
          <mat-select [(ngModel)]="selectedMapId">
            <mat-option [value]="null">Nenhum</mat-option>
            @for (map of availableMaps; track map.id) {
              <mat-option [value]="map.id">{{ map.name }}</mat-option>
            }
          </mat-select>
          <mat-hint>Ao clicar no POI, abre este mapa.</mat-hint>
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
      @if (existing) {
        <button mat-button color="warn" (click)="onDelete()">
          <mat-icon>delete</mat-icon>
          Excluir
        </button>
        @if (existing.targetMapId) {
          <button mat-stroked-button (click)="onOpen()">
            <mat-icon>open_in_new</mat-icon>
            Abrir mapa
          </button>
        }
      }
      <button mat-button (click)="onCancel()">Cancelar</button>
      <button
        mat-raised-button
        color="primary"
        [disabled]="!label.trim()"
        (click)="onSave()"
      >
        <mat-icon>check</mat-icon>
        Salvar
      </button>
    </mat-dialog-actions>
  `,
  styles: [
    `
      .poi-form {
        display: flex;
        flex-direction: column;
        gap: 16px;
        min-width: min(380px, 90vw);
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
    `,
  ],
})
export class PoiDialogComponent {
  private readonly dialogRef = inject(
    MatDialogRef<PoiDialogComponent, PoiDialogResult>,
  );
  private readonly data = inject<PoiDialogData>(MAT_DIALOG_DATA);

  protected readonly colors = POI_COLORS;
  protected readonly icons = POI_ICONS;

  protected existing: MapMarker | undefined = this.data.existing;
  protected availableMaps = this.data.availableMaps;

  protected label = this.data.existing?.label ?? '';
  protected description = this.data.existing?.description ?? '';
  protected selectedMapId: string | null =
    this.data.existing?.targetMapId ?? null;
  protected selectedColor = this.data.existing?.color ?? POI_COLORS[0];
  protected selectedIcon = this.data.existing?.icon ?? POI_ICONS[0];

  onCancel(): void {
    this.dialogRef.close(null);
  }

  onDelete(): void {
    this.dialogRef.close({ action: 'delete' });
  }

  onOpen(): void {
    this.dialogRef.close({ action: 'open' });
  }

  onSave(): void {
    if (!this.label.trim()) return;

    const marker: MapMarker = {
      id: this.existing?.id ?? crypto.randomUUID(),
      x: this.data.x,
      y: this.data.y,
      label: this.label.trim(),
      description: this.description.trim() || undefined,
      icon: this.selectedIcon,
      color: this.selectedColor,
      targetMapId: this.selectedMapId ?? undefined,
    };
    this.dialogRef.close({ action: 'save', marker });
  }
}
