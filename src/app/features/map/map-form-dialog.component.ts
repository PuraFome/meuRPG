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
import { StoreService } from '../../core/store/store.service';
import type { MapData } from '../../core/models/map';
import { FileUploadComponent } from '../../shared/components/file-upload.component';

export interface MapFormDialogData {
  map?: MapData;
}

function defaultGrid() {
  return { cellSize: 50, columns: 20, rows: 20, visible: false };
}

function defaultFog() {
  return { explored: [], visible: false };
}

function createDefaultMap(): MapData {
  const now = new Date();
  return {
    id: crypto.randomUUID(),
    name: '',
    description: '',
    width: 1024,
    height: 768,
    layers: [],
    grid: defaultGrid(),
    fogOfWar: defaultFog(),
    markers: [],
    submaps: [],
    createdAt: now,
    updatedAt: now,
  };
}

@Component({
  selector: 'app-map-form-dialog',
  standalone: true,
  imports: [
    FormsModule,
    MatButtonModule,
    MatDialogModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    FileUploadComponent,
  ],
  template: `
    <h2 mat-dialog-title>
      {{ data.map ? 'Editar Mapa' : 'Novo Mapa' }}
    </h2>
    <mat-dialog-content>
      <div class="map-form">
        <mat-form-field appearance="fill" class="full-width">
          <mat-label>Nome do mapa</mat-label>
          <input matInput [(ngModel)]="name" placeholder="Ex: Mundo de D&D" />
        </mat-form-field>

        <mat-form-field appearance="fill" class="full-width">
          <mat-label>Descrição</mat-label>
          <textarea
            matInput
            [(ngModel)]="description"
            rows="3"
            placeholder="Ex: Continente de Faerûn — o mundo onde os heróis vivem."
          ></textarea>
        </mat-form-field>

        <label class="section-label">Imagem do mapa (fundo)</label>
        <app-file-upload
          [acceptedTypes]="['image/']"
          [maxSize]="15 * 1024 * 1024"
          [showPreview]="false"
          (fileChange)="onImageSelected($event)"
        />

        @if (imagePreview()) {
          <div class="preview-wrapper">
            <img [src]="imagePreview()" class="preview" alt="Preview do mapa" />
            <button
              mat-icon-button
              class="remove-btn"
              (click)="removeImage(); $event.stopPropagation()"
              aria-label="Remover imagem"
            >
              <mat-icon>close</mat-icon>
            </button>
            <span class="preview-dims">{{ imageWidth() }} × {{ imageHeight() }} px</span>
          </div>
        }
      </div>
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <button mat-button (click)="onCancel()">Cancelar</button>
      <button
        mat-raised-button
        color="primary"
        [disabled]="!name.trim()"
        (click)="onSave()"
      >
        <mat-icon>check</mat-icon>
        {{ data.map ? 'Salvar' : 'Criar' }}
      </button>
    </mat-dialog-actions>
  `,
  styles: [
    `
      .map-form {
        display: flex;
        flex-direction: column;
        gap: 16px;
        min-width: min(420px, 90vw);
        padding: 8px 0;
      }
      .full-width {
        width: 100%;
      }
      .section-label {
        font-size: 0.8125rem;
        font-weight: 500;
        color: rgba(255,255,255,0.7);
      }
      .preview-wrapper {
        position: relative;
        display: inline-block;
        align-self: center;
      }
      .preview {
        max-width: min(320px, 100%);
        max-height: 220px;
        object-fit: contain;
        border-radius: 8px;
        display: block;
        border: 1px solid rgba(255,255,255,0.12);
      }
      .remove-btn {
        position: absolute;
        top: -8px;
        right: -8px;
        background: rgba(0, 0, 0, 0.6);
      }
      .preview-dims {
        position: absolute;
        bottom: 6px;
        right: 8px;
        font-size: 0.7rem;
        color: rgba(255,255,255,0.85);
        background: rgba(0,0,0,0.55);
        padding: 2px 8px;
        border-radius: 10px;
      }
    `,
  ],
})
export class MapFormDialogComponent {
  private readonly dialogRef = inject(
    MatDialogRef<MapFormDialogComponent, MapData | null>,
  );
  readonly data = inject<MapFormDialogData>(MAT_DIALOG_DATA);
  private readonly store = inject(StoreService<MapData>);

  protected name = this.data.map?.name ?? '';
  protected description = this.data.map?.description ?? '';
  protected imagePreview = signal<string | null>(
    this.data.map?.backgroundImage ?? null,
  );
  protected imageWidth = signal(this.data.map?.width ?? 1024);
  protected imageHeight = signal(this.data.map?.height ?? 768);

  onCancel(): void {
    this.dialogRef.close(null);
  }

  onImageSelected(file: File): void {
    const reader = new FileReader();
    reader.onload = () => {
      const dataUrl = reader.result as string;
      this.imagePreview.set(dataUrl);
      const img = new Image();
      img.onload = () => {
        this.imageWidth.set(img.naturalWidth);
        this.imageHeight.set(img.naturalHeight);
      };
      img.src = dataUrl;
    };
    reader.readAsDataURL(file);
  }

  removeImage(): void {
    this.imagePreview.set(null);
    this.imageWidth.set(1024);
    this.imageHeight.set(768);
  }

  onSave(): void {
    if (!this.name.trim()) return;

    const existing = this.data.map;
    const base: MapData = existing ?? createDefaultMap();
    const hasImage = !!this.imagePreview();
    const saved: MapData = {
      ...base,
      name: this.name.trim(),
      description: this.description.trim() || undefined,
      backgroundImage: this.imagePreview() ?? undefined,
      width: hasImage ? this.imageWidth() : 1024,
      height: hasImage ? this.imageHeight() : 768,
      updatedAt: new Date(),
    };

    if (existing) {
      this.store.update('maps', existing.id, saved);
    } else {
      this.store.set('maps', saved);
    }

    this.dialogRef.close(saved);
  }
}
