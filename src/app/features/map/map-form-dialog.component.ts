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
import { StoreService } from '../../core/store/store.service';
import type { MapData, MapKind } from '../../core/models/map';
import { FileUploadComponent } from '../../shared/components/file-upload.component';
import {
  DEFAULT_COLUMNS,
  DEFAULT_ROWS,
  createBlankBackground,
  createDefaultGrid,
} from './map-defaults';

export interface MapFormDialogData {
  map?: MapData;
}

interface MapKindOption {
  value: MapKind;
  label: string;
  icon: string;
  hint: string;
}

const MAP_KINDS: MapKindOption[] = [
  { value: 'world', label: 'Mundo', icon: 'public', hint: 'Mapa regional sobre o mundo real' },
  { value: 'city', label: 'Cidade', icon: 'location_city', hint: 'Planta de uma cidade ou vila' },
  { value: 'dungeon', label: 'Masmorra', icon: 'castle', hint: 'Grade para desenhar salas e corredores' },
  { value: 'local', label: 'Local', icon: 'storefront', hint: 'Interior de loja, taverna ou cômodo' },
];

function createDefaultMap(): MapData {
  const now = new Date();
  return {
    id: crypto.randomUUID(),
    name: '',
    description: '',
    kind: 'world',
    width: 1024,
    height: 768,
    layers: [],
    grid: createDefaultGrid({ visible: false }),
    fogOfWar: { explored: [], visible: false },
    markers: [],
    submaps: [],
    dungeon: { tiles: {}, objects: {} },
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
    MatSelectModule,
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
          <input matInput [(ngModel)]="name" placeholder="Ex: Masmorra de Ravenloft" />
        </mat-form-field>

        <mat-form-field appearance="fill" class="full-width">
          <mat-label>Tipo de mapa</mat-label>
          <mat-select [(ngModel)]="kind">
            @for (option of kinds; track option.value) {
              <mat-option [value]="option.value">
                <mat-icon class="kind-icon">{{ option.icon }}</mat-icon>
                {{ option.label }}
              </mat-option>
            }
          </mat-select>
          <mat-hint>{{ kindHint() }}</mat-hint>
        </mat-form-field>

        <mat-form-field appearance="fill" class="full-width">
          <mat-label>Descrição</mat-label>
          <textarea
            matInput
            [(ngModel)]="description"
            rows="3"
            placeholder="Ex: Salões abandonados sob a montanha."
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

        <mat-form-field appearance="fill" class="full-width">
          <mat-label>Tamanho da célula (px)</mat-label>
          <input
            matInput
            type="number"
            min="10"
            max="400"
            step="10"
            [(ngModel)]="cellSize"
          />
          <mat-hint>Grade para desenhar a masmorra: {{ gridLabel() }}</mat-hint>
        </mat-form-field>
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
      .kind-icon {
        font-size: 1.1rem;
        width: 1.1rem;
        height: 1.1rem;
        margin-right: 8px;
        vertical-align: middle;
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

  protected readonly kinds = MAP_KINDS;
  protected name = this.data.map?.name ?? '';
  protected description = this.data.map?.description ?? '';
  protected kind: MapKind = this.data.map?.kind ?? 'world';
  protected cellSize = this.data.map?.grid?.cellSize ?? 50;
  protected imagePreview = signal<string | null>(
    this.data.map?.backgroundImage ?? null,
  );
  protected imageWidth = signal(this.data.map?.width ?? 1024);
  protected imageHeight = signal(this.data.map?.height ?? 768);

  protected kindHint(): string {
    return MAP_KINDS.find((k) => k.value === this.kind)?.hint ?? '';
  }

  protected gridLabel(): string {
    const size = Math.max(10, Math.round(this.cellSize || 50));
    if (this.imagePreview()) {
      const cols = Math.max(1, Math.round(this.imageWidth() / size));
      const rows = Math.max(1, Math.round(this.imageHeight() / size));
      return `${cols} × ${rows} células`;
    }
    return `${DEFAULT_COLUMNS} × ${DEFAULT_ROWS} células`;
  }

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
    const latest = existing
      ? this.store.snapshot('maps').find((m) => m.id === existing.id)
      : undefined;
    const base = latest ?? existing ?? createDefaultMap();

    const cellSize = Math.max(10, Math.round(this.cellSize || 50));
    const hasImage = !!this.imagePreview();
    const isWorld = this.kind === 'world';

    let width: number;
    let height: number;
    let backgroundImage: string | undefined;

    if (hasImage) {
      width = this.imageWidth();
      height = this.imageHeight();
      backgroundImage = this.imagePreview() ?? undefined;
    } else if (isWorld) {
      width = base.width || 1024;
      height = base.height || 768;
      backgroundImage = undefined;
    } else {
      width = DEFAULT_COLUMNS * cellSize;
      height = DEFAULT_ROWS * cellSize;
      backgroundImage = createBlankBackground(width, height);
    }

    const columns = Math.max(1, Math.round(width / cellSize));
    const rows = Math.max(1, Math.round(height / cellSize));

    const saved: MapData = {
      ...base,
      name: this.name.trim(),
      description: this.description.trim() || undefined,
      kind: this.kind,
      backgroundImage,
      width,
      height,
      grid: {
        cellSize,
        columns,
        rows,
        color: base.grid?.color,
        visible: existing ? (base.grid?.visible ?? false) : false,
      },
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
