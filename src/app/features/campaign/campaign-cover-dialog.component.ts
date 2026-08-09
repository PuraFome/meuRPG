import { Component, DestroyRef, OnInit, inject, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import {
  MatDialogModule,
  MatDialogRef,
  MAT_DIALOG_DATA,
} from '@angular/material/dialog';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { FileUploadComponent } from '../../shared/components/file-upload.component';
import { IndexedDbFileRepository } from '../../core';

export interface CampaignCoverDialogData {
  folderName: string;
  currentCoverFileId: string | null;
}

function generateThumbnail(file: File, maxWidth = 320): Promise<Blob> {
  return new Promise((resolve, reject) => {
    const objectUrl = URL.createObjectURL(file);
    const image = new Image();

    image.onload = () => {
      const scale = Math.min(1, maxWidth / image.naturalWidth);
      const width = Math.max(1, Math.round(image.naturalWidth * scale));
      const height = Math.max(1, Math.round(image.naturalHeight * scale));
      const canvas = document.createElement('canvas');
      canvas.width = width;
      canvas.height = height;

      const context = canvas.getContext('2d');
      if (!context) {
        URL.revokeObjectURL(objectUrl);
        reject(new Error('Não foi possível criar o contexto do canvas'));
        return;
      }

      context.drawImage(image, 0, 0, width, height);
      canvas.toBlob(
        (blob) => {
          URL.revokeObjectURL(objectUrl);
          if (blob) resolve(blob);
          else reject(new Error('Falha ao gerar a miniatura'));
        },
        'image/jpeg',
        0.8,
      );
    };

    image.onerror = () => {
      URL.revokeObjectURL(objectUrl);
      reject(new Error('Falha ao carregar a imagem'));
    };

    image.src = objectUrl;
  });
}

@Component({
  selector: 'app-campaign-cover-dialog',
  standalone: true,
  imports: [
    CommonModule,
    MatDialogModule,
    MatButtonModule,
    MatIconModule,
    FileUploadComponent,
  ],
  template: `
    <h2 mat-dialog-title>Capa da Campanha</h2>

    <mat-dialog-content class="dialog-content">
      <p class="folder-name">{{ data.folderName }}</p>

      @if (currentCoverUrl(); as coverUrl) {
        <div class="cover-preview">
          <img [src]="coverUrl" alt="Capa atual" />
        </div>
      } @else if (data.currentCoverFileId) {
        <div class="cover-preview cover-preview--empty">
          <mat-icon>image</mat-icon>
          <span>Capa não encontrada</span>
        </div>
      }

      <app-file-upload
        [acceptedTypes]="acceptedTypes"
        [maxSize]="maxSize"
        (fileChange)="onFileSelected($event)"
      />

      <p class="upload-hint">Imagens PNG, JPEG, WebP ou GIF • Máx 5 MB</p>

      @if (errorMessage()) {
        <div class="error-message">
          <mat-icon class="error-icon">error_outline</mat-icon>
          {{ errorMessage() }}
        </div>
      }
    </mat-dialog-content>

    <mat-dialog-actions align="end">
      <button mat-button (click)="onCancel()">Cancelar</button>
      @if (data.currentCoverFileId) {
        <button
          mat-stroked-button
          color="warn"
          (click)="onRemove()"
          [disabled]="isSaving()"
        >
          <mat-icon>delete</mat-icon>
          Remover Capa
        </button>
      }
      <button
        mat-raised-button
        color="primary"
        (click)="onSave()"
        [disabled]="!selectedFile() || isSaving()"
      >
        <mat-icon>cloud_upload</mat-icon>
        Salvar Capa
      </button>
    </mat-dialog-actions>
  `,
  styles: [
    `
      :host {
        display: block;
      }

      .dialog-content {
        min-width: min(520px, 92vw);
        display: flex;
        flex-direction: column;
        gap: 16px;
        padding-top: 4px;
      }

      .folder-name {
        margin: 0;
        font-size: 0.9rem;
        opacity: 0.55;
      }

      .cover-preview {
        width: 100%;
        max-height: 180px;
        border-radius: 12px;
        overflow: hidden;
        border: 1px solid rgba(255, 255, 255, 0.08);
        background: rgba(255, 255, 255, 0.03);
      }

      .cover-preview img {
        display: block;
        width: 100%;
        max-height: 180px;
        object-fit: cover;
      }

      .cover-preview--empty {
        display: flex;
        align-items: center;
        justify-content: center;
        gap: 8px;
        padding: 32px 16px;
        font-size: 0.85rem;
        opacity: 0.5;
      }

      .cover-preview--empty mat-icon {
        font-size: 1.4rem;
        width: 1.4rem;
        height: 1.4rem;
      }

      .upload-hint {
        margin: -8px 0 0;
        font-size: 0.8rem;
        opacity: 0.45;
      }

      .error-message {
        display: flex;
        align-items: center;
        gap: 6px;
        font-size: 0.85rem;
        color: #f44336;
      }

      .error-icon {
        font-size: 1.1rem;
        width: 1.1rem;
        height: 1.1rem;
      }
    `,
  ],
})
export class CampaignCoverDialogComponent implements OnInit {
  private readonly dialogRef = inject(MatDialogRef<CampaignCoverDialogComponent>);
  readonly data = inject<CampaignCoverDialogData>(MAT_DIALOG_DATA);
  private readonly fileRepo = inject(IndexedDbFileRepository);
  private readonly destroyRef = inject(DestroyRef);

  readonly acceptedTypes = [
    'image/png',
    'image/jpeg',
    'image/webp',
    'image/gif',
  ];
  readonly maxSize = 5 * 1024 * 1024;

  readonly selectedFile = signal<File | null>(null);
  readonly isSaving = signal(false);
  readonly errorMessage = signal<string | null>(null);
  readonly currentCoverUrl = signal<string | null>(null);

  constructor() {
    this.destroyRef.onDestroy(() => {
      const url = this.currentCoverUrl();
      if (url) URL.revokeObjectURL(url);
    });
  }

  ngOnInit(): void {
    this.loadCurrentCover();
  }

  private async loadCurrentCover(): Promise<void> {
    const coverFileId = this.data.currentCoverFileId;
    if (!coverFileId) return;
    try {
      const record = await this.fileRepo.get(coverFileId);
      if (record) {
        this.currentCoverUrl.set(URL.createObjectURL(record.data));
      }
    } catch {
      this.currentCoverUrl.set(null);
    }
  }

  onFileSelected(file: File): void {
    this.selectedFile.set(file);
    this.errorMessage.set(null);
  }

  async onSave(): Promise<void> {
    const file = this.selectedFile();
    if (!file || this.isSaving()) return;

    this.isSaving.set(true);
    this.errorMessage.set(null);
    try {
      const fileId = 'cover-' + crypto.randomUUID();

      await this.fileRepo.save({
        id: fileId,
        name: file.name,
        data: file,
        mimeType: file.type,
        size: file.size,
        uploadedAt: new Date(),
      });

      const thumbBlob = await generateThumbnail(file);
      await this.fileRepo.save({
        id: fileId + '-thumb',
        name: file.name + '-thumb.jpg',
        data: thumbBlob,
        mimeType: 'image/jpeg',
        size: thumbBlob.size,
        uploadedAt: new Date(),
      });

      const oldCoverFileId = this.data.currentCoverFileId;
      if (oldCoverFileId) {
        await this.deleteIfPresent(oldCoverFileId);
        await this.deleteIfPresent(oldCoverFileId + '-thumb');
      }

      this.clearCurrentCover();
      this.dialogRef.close({ fileId });
    } catch {
      this.errorMessage.set('Não foi possível salvar a capa. Tente novamente.');
    } finally {
      this.isSaving.set(false);
    }
  }

  async onRemove(): Promise<void> {
    const coverFileId = this.data.currentCoverFileId;
    if (!coverFileId) return;

    await this.deleteIfPresent(coverFileId);
    await this.deleteIfPresent(coverFileId + '-thumb');

    this.clearCurrentCover();
    this.dialogRef.close({ fileId: null });
  }

  onCancel(): void {
    this.dialogRef.close();
  }

  private async deleteIfPresent(id: string): Promise<void> {
    try {
      await this.fileRepo.delete(id);
    } catch {
      // A missing record must not break the flow.
    }
  }

  private clearCurrentCover(): void {
    const url = this.currentCoverUrl();
    if (url) URL.revokeObjectURL(url);
    this.currentCoverUrl.set(null);
  }
}
