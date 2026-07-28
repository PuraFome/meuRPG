import {
  Component,
  EventEmitter,
  inject,
  Output,
  signal,
  ViewChild,
} from '@angular/core';
import { FileUploadComponent } from '../../shared/components/file-upload.component';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { StoreService } from '../../core/store/store.service';
import { IndexedDbFileRepository } from '../../core/repositories/indexed-db-file-repository';
import type { GalleryItem } from '../../core/models/gallery';

@Component({
  selector: 'app-gallery-upload',
  standalone: true,
  imports: [
    FileUploadComponent,
    MatProgressBarModule,
    MatButtonModule,
    MatIconModule,
  ],
  template: `
    <div class="upload-section">
      <app-file-upload
        #fileUpload
        [acceptedTypes]="acceptedTypes"
        [maxSize]="maxSize"
        (fileChange)="onFileSelected($event)"
      />

      @if (selectedFile() && !isUploading()) {
        <div class="upload-actions">
          <span class="file-name">
            <mat-icon class="file-icon">
              {{ selectedFile()!.type.startsWith('image/') ? 'image' : 'audiotrack' }}
            </mat-icon>
            {{ selectedFile()!.name }}
            <span class="file-size">({{ formatSize(selectedFile()!.size) }})</span>
          </span>
          <div class="action-buttons">
            <button
              mat-stroked-button
              color="warn"
              (click)="cancelUpload()"
            >
              <mat-icon>close</mat-icon>
              Cancelar
            </button>
            <button
              mat-raised-button
              color="primary"
              (click)="confirmUpload()"
            >
              <mat-icon>cloud_upload</mat-icon>
              Fazer Upload
            </button>
          </div>
        </div>
      }

      @if (isUploading()) {
        <div class="progress-section">
          <div class="progress-header">
            <span class="progress-label">Enviando…</span>
            <span class="progress-value">{{ uploadProgress() }}%</span>
          </div>
          <mat-progress-bar
            mode="determinate"
            [value]="uploadProgress()"
            class="upload-progress"
          />
          @if (uploadProgress() === 100) {
            <p class="progress-done">Upload concluído!</p>
          }
        </div>
      }
    </div>
  `,
  styles: `
    :host {
      display: block;
    }

    .upload-section {
      padding: 24px;
      border-radius: 12px;
      background: rgba(255, 255, 255, 0.03);
      border: 1px solid rgba(255, 255, 255, 0.06);
    }

    .upload-actions {
      display: flex;
      align-items: center;
      justify-content: space-between;
      flex-wrap: wrap;
      gap: 12px;
      margin-top: 16px;
      padding: 12px 16px;
      border-radius: 8px;
      background: rgba(255, 255, 255, 0.04);
    }

    .file-name {
      display: flex;
      align-items: center;
      gap: 8px;
      font-size: 0.9rem;
      opacity: 0.85;
    }

    .file-icon {
      font-size: 1.2rem;
      width: 1.2rem;
      height: 1.2rem;
      opacity: 0.6;
    }

    .file-size {
      opacity: 0.45;
      font-size: 0.8rem;
    }

    .action-buttons {
      display: flex;
      gap: 8px;
    }

    .progress-section {
      margin-top: 16px;
      padding: 16px;
      border-radius: 8px;
      background: rgba(255, 255, 255, 0.04);
    }

    .progress-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 8px;
    }

    .progress-label {
      font-size: 0.85rem;
      opacity: 0.7;
    }

    .progress-value {
      font-size: 0.85rem;
      font-weight: 500;
      opacity: 0.8;
    }

    .upload-progress {
      border-radius: 4px;
    }

    .progress-done {
      color: #66bb6a;
      font-size: 0.85rem;
      margin: 8px 0 0;
    }
  `,
})
export class GalleryUploadComponent {
  private readonly store = inject(StoreService<GalleryItem>);
  private readonly fileRepo = new IndexedDbFileRepository();

  @Output() uploadComplete = new EventEmitter<void>();

  @ViewChild('fileUpload') fileUpload!: FileUploadComponent;

  readonly acceptedTypes = [
    'image/png',
    'image/jpeg',
    'image/gif',
    'image/webp',
    'image/svg+xml',
    'audio/mpeg',
    'audio/ogg',
    'audio/wav',
  ];
  readonly maxSize = 10 * 1024 * 1024; // 10 MB

  readonly selectedFile = signal<File | null>(null);
  readonly isUploading = signal(false);
  readonly uploadProgress = signal(0);
  readonly previewUrl = signal<string | null>(null);

  private simulatedTimer: ReturnType<typeof setInterval> | null = null;

  onFileSelected(file: File): void {
    // Revoke previous preview
    const prev = this.previewUrl();
    if (prev) URL.revokeObjectURL(prev);

    this.selectedFile.set(file);

    if (file.type.startsWith('image/')) {
      this.previewUrl.set(URL.createObjectURL(file));
    } else {
      this.previewUrl.set(null);
    }
  }

  async confirmUpload(): Promise<void> {
    const file = this.selectedFile();
    if (!file) return;

    this.isUploading.set(true);
    this.uploadProgress.set(0);

    // Simulated upload progress (ramps up quickly then slows)
    this.simulatedTimer = setInterval(() => {
      this.uploadProgress.update((v) => {
        if (v >= 95) return 95;
        // Fast at start, slower as it progresses
        const increment = v < 50 ? 15 : v < 80 ? 8 : 3;
        return Math.min(v + increment + Math.random() * 5, 95);
      });
    }, 250);

    const id = crypto.randomUUID();
    const mediaType: GalleryItem['type'] =
      file.type.startsWith('image/') ? 'image' : 'audio';

    // Save binary data to IndexedDb
    await this.fileRepo.save({
      id,
      name: file.name,
      data: file,
      mimeType: file.type,
      size: file.size,
      uploadedAt: new Date(),
    });

    // Create gallery item metadata
    const item: GalleryItem = {
      id,
      name: file.name,
      type: mediaType,
      url: id, // Reference key to fetch blob from IndexedDb
      thumbnailUrl: undefined, // Generated at display time from blob
      folderId: null,
      size: file.size,
      createdAt: new Date(),
    };

    this.store.set('gallery', item);

    // Complete progress
    if (this.simulatedTimer) {
      clearInterval(this.simulatedTimer);
      this.simulatedTimer = null;
    }
    this.uploadProgress.set(100);

    // Brief pause at 100% for user feedback
    await new Promise((r) => setTimeout(r, 600));

    // Reset state
    this.resetUpload();
    this.uploadComplete.emit();
  }

  cancelUpload(): void {
    this.resetUpload();
  }

  private resetUpload(): void {
    // Revoke preview URL
    const preview = this.previewUrl();
    if (preview) URL.revokeObjectURL(preview);

    this.selectedFile.set(null);
    this.previewUrl.set(null);
    this.isUploading.set(false);
    this.uploadProgress.set(0);

    // Reset FileUploadComponent internal state
    this.fileUpload?.clearFile();

    if (this.simulatedTimer) {
      clearInterval(this.simulatedTimer);
      this.simulatedTimer = null;
    }
  }

  protected formatSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  }
}
