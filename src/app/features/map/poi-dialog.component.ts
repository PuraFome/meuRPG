import { Component, inject, signal } from '@angular/core';
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
import type { MapData, MapKind, MapMarker } from '../../core/models/map';
import { FileUploadComponent } from '../../shared/components/file-upload.component';
import type { MapImageInput } from './map-defaults';

export interface PoiDialogData {
  x: number;
  y: number;
  existing?: MapMarker;
  availableMaps: MapData[];
  currentMapId: string | null;
}

export interface NewSubmapRequest {
  name: string;
  kind: MapKind;
  image?: MapImageInput;
}

export type PoiDialogResult =
  | { action: 'save'; marker: MapMarker; newSubmap?: NewSubmapRequest }
  | { action: 'delete' }
  | { action: 'open' }
  | null;

const NEW_SUBMAP_VALUE = '__new__';

const POI_COLORS = [
  '#7c4dff', '#e53935', '#ff6d00', '#ffd600', '#00c853',
  '#2979ff', '#00bcd4', '#ff4081', '#6d4c41', '#78909c',
];

const POI_ICONS = [
  '🍺', '🍷', '🍖', '🛏️', '🏰', '🏛️', '⛪', '🏠', '🏘️', '🛒',
  '⚒️', '🧙', '👺', '👹', '🧌', '🐉', '💀', '🧟', '🕷️', '🐺',
  '🦇', '⚔️', '🛡️', '🏹', '🔥', '🕳️', '🗝️', '💎', '🪙', '🧪',
  '📜', '🚪', '⚓', '⛵', '🌲', '⛰️', '🏕️', '🕯️', '🪦', '🧭',
];

const SUBMAP_KINDS: { value: MapKind; label: string }[] = [
  { value: 'dungeon', label: 'Masmorra' },
  { value: 'city', label: 'Cidade' },
  { value: 'local', label: 'Local (loja, taverna...)' },
  { value: 'world', label: 'Mundo / Região' },
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
    FileUploadComponent,
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
          <mat-label>Destino do ponto</mat-label>
          <mat-select [(ngModel)]="destination">
            <mat-option value="">Nenhum</mat-option>
            <mat-option [value]="newSubmapValue">
              <mat-icon class="opt-icon">add</mat-icon>
              Criar novo submapa...
            </mat-option>
            @for (map of availableMaps; track map.id) {
              <mat-option [value]="map.id">{{ map.name }}</mat-option>
            }
          </mat-select>
          <mat-hint>Ao clicar no ponto, abre este mapa.</mat-hint>
        </mat-form-field>

        @if (destination === newSubmapValue) {
          <div class="new-submap">
            <mat-form-field appearance="fill" class="full-width">
              <mat-label>Nome do novo submapa</mat-label>
              <input
                matInput
                [(ngModel)]="newSubmapName"
                placeholder="Ex: Cripta subterrânea"
              />
            </mat-form-field>
            <mat-form-field appearance="fill" class="full-width">
              <mat-label>Tipo do novo submapa</mat-label>
              <mat-select [(ngModel)]="newSubmapKind">
                @for (kind of submapKinds; track kind.value) {
                  <mat-option [value]="kind.value">{{ kind.label }}</mat-option>
                }
              </mat-select>
            </mat-form-field>

            <label class="section-label">Imagem do submapa (opcional)</label>
            <app-file-upload
              [acceptedTypes]="['image/']"
              [maxSize]="15 * 1024 * 1024"
              [showPreview]="false"
              (fileChange)="onSubmapImageSelected($event)"
            />
            @if (submapImage()) {
              <div class="submap-preview">
                <img [src]="submapImage()!.dataUrl" alt="Preview do submapa" />
                <span>{{ submapImage()!.width }} × {{ submapImage()!.height }} px</span>
              </div>
            }
          </div>
        }

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
              <span class="icon-emoji">{{ icon }}</span>
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
        [disabled]="!label.trim() || (destination === newSubmapValue && !newSubmapName.trim())"
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
      .opt-icon {
        font-size: 1.1rem;
        width: 1.1rem;
        height: 1.1rem;
        margin-right: 8px;
        vertical-align: middle;
      }
      .new-submap {
        display: flex;
        flex-direction: column;
        gap: 8px;
        padding: 12px;
        border: 1px dashed rgba(124, 77, 255, 0.5);
        border-radius: 10px;
        background: rgba(124, 77, 255, 0.06);
      }
      .submap-preview {
        display: flex;
        flex-direction: column;
        align-items: center;
        gap: 4px;
      }
      .submap-preview img {
        max-width: 100%;
        max-height: 160px;
        object-fit: contain;
        border-radius: 8px;
        border: 1px solid rgba(255,255,255,0.12);
      }
      .submap-preview span {
        font-size: 0.7rem;
        color: rgba(255,255,255,0.7);
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
      .icon-emoji {
        font-size: 1.35rem;
        line-height: 1;
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
  protected readonly submapKinds = SUBMAP_KINDS;
  protected readonly newSubmapValue = NEW_SUBMAP_VALUE;

  protected existing: MapMarker | undefined = this.data.existing;
  protected availableMaps = this.data.availableMaps;

  protected label = this.data.existing?.label ?? '';
  protected description = this.data.existing?.description ?? '';
  protected destination: string = this.data.existing?.targetMapId ?? '';
  protected newSubmapName = '';
  protected newSubmapKind: MapKind = 'dungeon';
  protected readonly submapImage = signal<MapImageInput | null>(null);
  protected selectedColor = this.data.existing?.color ?? POI_COLORS[0];
  protected selectedIcon = this.data.existing?.icon ?? POI_ICONS[0];

  onCancel(): void {
    this.dialogRef.close(null);
  }

  onDelete(): void {
    this.dialogRef.close({ action: 'delete' });
  }

  onSubmapImageSelected(file: File): void {
    const reader = new FileReader();
    reader.onload = () => {
      const dataUrl = reader.result as string;
      const img = new Image();
      img.onload = () => {
        this.submapImage.set({
          dataUrl,
          width: img.naturalWidth,
          height: img.naturalHeight,
        });
      };
      img.src = dataUrl;
    };
    reader.readAsDataURL(file);
  }

  onOpen(): void {
    this.dialogRef.close({ action: 'open' });
  }

  onSave(): void {
    if (!this.label.trim()) return;

    const isNewSubmap = this.destination === NEW_SUBMAP_VALUE;
    if (isNewSubmap && !this.newSubmapName.trim()) return;

    const marker: MapMarker = {
      id: this.existing?.id ?? crypto.randomUUID(),
      x: this.data.x,
      y: this.data.y,
      label: this.label.trim(),
      description: this.description.trim() || undefined,
      icon: this.selectedIcon,
      color: this.selectedColor,
      targetMapId:
        !isNewSubmap && this.destination ? this.destination : undefined,
    };

    this.dialogRef.close({
      action: 'save',
      marker,
      newSubmap: isNewSubmap
        ? {
            name: this.newSubmapName.trim(),
            kind: this.newSubmapKind,
            image: this.submapImage() ?? undefined,
          }
        : undefined,
    });
  }
}
