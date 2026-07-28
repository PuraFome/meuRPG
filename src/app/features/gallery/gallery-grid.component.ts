import {
  Component,
  EventEmitter,
  inject,
  OnDestroy,
  OnInit,
  Output,
  signal,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { MatIconModule } from '@angular/material/icon';
import { MatButtonModule } from '@angular/material/button';
import { MatSelectModule } from '@angular/material/select';
import { MatFormFieldModule } from '@angular/material/form-field';
import { FormsModule } from '@angular/forms';
import { Subscription } from 'rxjs';
import { StoreService } from '../../core/store/store.service';
import { IndexedDbFileRepository } from '../../core/repositories/indexed-db-file-repository';
import type { GalleryItem } from '../../core/models/gallery';
import { EmptyStateComponent } from '../../shared/components/empty-state.component';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';

type SortField = 'name' | 'date' | 'type';

@Component({
  selector: 'app-gallery-grid',
  standalone: true,
  imports: [
    CommonModule,
    MatIconModule,
    MatButtonModule,
    MatSelectModule,
    MatFormFieldModule,
    FormsModule,
    EmptyStateComponent,
    MatProgressSpinnerModule,
  ],
  template: `
    <div class="grid-section">
      <div class="grid-toolbar">
        <h2 class="grid-title">Galeria</h2>

        <mat-form-field appearance="fill" class="sort-select" subscriptSizing="dynamic">
          <mat-label>Ordenar por</mat-label>
          <mat-select [value]="sortField()" (valueChange)="sortField.set($event); onSortChange()">
            <mat-option value="date">Data</mat-option>
            <mat-option value="name">Nome</mat-option>
            <mat-option value="type">Tipo</mat-option>
          </mat-select>
        </mat-form-field>
      </div>

      @if (loading()) {
        <div class="loading-container">
          <mat-progress-spinner mode="indeterminate" diameter="36" />
        </div>
      } @else if (sortedItems().length === 0) {
        <app-empty-state
          icon="collections_bookmark"
          message="Nenhum arquivo na galeria. Arraste ou clique acima para adicionar."
        />
      } @else {
        <div class="thumbnail-grid">
          @for (item of sortedItems(); track item.id) {
            <button
              class="thumbnail-card"
              (click)="openLightbox(item)"
              [class.is-audio]="item.type === 'audio'"
            >
              <div class="card-preview">
                @if (item.type === 'image') {
                  @if (getThumbnail(item.id); as thumbUrl) {
                    <img
                      [src]="thumbUrl"
                      [alt]="item.name"
                      class="thumb-img"
                      loading="lazy"
                    />
                  } @else {
                    <div class="thumb-placeholder">
                      <mat-icon class="placeholder-icon">image</mat-icon>
                    </div>
                  }
                } @else {
                  <div class="audio-icon-wrapper">
                    <mat-icon class="audio-icon">audiotrack</mat-icon>
                    <div class="audio-wave">
                      <span></span><span></span><span></span>
                      <span></span><span></span>
                    </div>
                  </div>
                }
              </div>
              <div class="card-info">
                <span class="card-name" [title]="item.name">{{ item.name }}</span>
                <span class="card-meta">{{ formatSize(item.size) }} • {{ formatDate(item.createdAt) }}</span>
              </div>
            </button>
          }
        </div>
      }
    </div>
  `,
  styles: `
    :host {
      display: block;
    }

    .grid-section {
      padding: 0 24px 24px;
    }

    .loading-container {
      display: flex;
      align-items: center;
      justify-content: center;
      padding: 48px 0;
    }

    .grid-toolbar {
      display: flex;
      align-items: center;
      justify-content: space-between;
      flex-wrap: wrap;
      gap: 12px;
      margin-bottom: 20px;
    }

    .grid-title {
      font-size: 1.1rem;
      font-weight: 500;
      margin: 0;
      opacity: 0.8;
    }

    .sort-select {
      width: 160px;
    }

    .thumbnail-grid {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
      gap: 16px;
    }

    .thumbnail-card {
      all: unset;
      display: flex;
      flex-direction: column;
      border-radius: 10px;
      overflow: hidden;
      background: rgba(255, 255, 255, 0.03);
      border: 1px solid rgba(255, 255, 255, 0.06);
      cursor: pointer;
      transition: transform 0.2s ease, box-shadow 0.2s ease, border-color 0.2s ease;
    }

    .thumbnail-card:hover {
      transform: translateY(-3px);
      box-shadow: 0 8px 24px rgba(0, 0, 0, 0.25);
      border-color: rgba(255, 255, 255, 0.15);
    }

    .thumbnail-card:focus-visible {
      outline: 2px solid rgb(var(--mat-app-primary));
      outline-offset: 2px;
    }

    .card-preview {
      aspect-ratio: 1;
      overflow: hidden;
      display: flex;
      align-items: center;
      justify-content: center;
      background: rgba(255, 255, 255, 0.02);
    }

    .thumb-img {
      width: 100%;
      height: 100%;
      object-fit: cover;
      display: block;
      transition: transform 0.3s ease;
    }

    .thumbnail-card:hover .thumb-img {
      transform: scale(1.08);
    }

    .thumb-placeholder {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 100%;
      height: 100%;
      opacity: 0.3;
    }

    .placeholder-icon {
      font-size: 2.5rem;
      width: 2.5rem;
      height: 2.5rem;
    }

    .audio-icon-wrapper {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      gap: 8px;
      width: 100%;
      height: 100%;
    }

    .audio-icon {
      font-size: 2.5rem;
      width: 2.5rem;
      height: 2.5rem;
      opacity: 0.5;
      transition: opacity 0.2s;
    }

    .thumbnail-card:hover .audio-icon {
      opacity: 0.8;
    }

    .audio-wave {
      display: flex;
      align-items: center;
      gap: 3px;
      height: 24px;
    }

    .audio-wave span {
      display: block;
      width: 3px;
      border-radius: 2px;
      background: rgba(255, 255, 255, 0.4);
      animation: wave 1.2s ease-in-out infinite;
    }

    .audio-wave span:nth-child(1) { height: 8px; animation-delay: 0s; }
    .audio-wave span:nth-child(2) { height: 16px; animation-delay: 0.15s; }
    .audio-wave span:nth-child(3) { height: 22px; animation-delay: 0.3s; }
    .audio-wave span:nth-child(4) { height: 14px; animation-delay: 0.45s; }
    .audio-wave span:nth-child(5) { height: 10px; animation-delay: 0.6s; }

    @keyframes wave {
      0%, 100% { transform: scaleY(0.5); opacity: 0.3; }
      50% { transform: scaleY(1); opacity: 0.7; }
    }

    .card-info {
      display: flex;
      flex-direction: column;
      padding: 10px 12px;
      gap: 2px;
      border-top: 1px solid rgba(255, 255, 255, 0.04);
    }

    .card-name {
      font-size: 0.85rem;
      line-height: 1.3;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      opacity: 0.85;
    }

    .card-meta {
      font-size: 0.75rem;
      opacity: 0.45;
    }
  `,
})
export class GalleryGridComponent implements OnInit, OnDestroy {
  private readonly store = inject(StoreService<GalleryItem>);
  private readonly fileRepo = new IndexedDbFileRepository();

  @Output() lightboxOpen = new EventEmitter<GalleryItem>();

  readonly loading = signal(true);
  readonly sortField = signal<SortField>('date');
  readonly sortedItems = signal<GalleryItem[]>([]);

  private items: GalleryItem[] = [];
  private thumbnailUrls = new Map<string, string>();
  private subscription: Subscription | null = null;

  ngOnInit(): void {
    this.subscription = this.store.getAll('gallery').subscribe(async (items) => {
      this.items = items;
      await this.loadThumbnails(items);
      this.applySort();
      this.loading.set(false);
    });
  }

  ngOnDestroy(): void {
    this.subscription?.unsubscribe();
    // Revoke all object URLs
    for (const url of this.thumbnailUrls.values()) {
      URL.revokeObjectURL(url);
    }
    this.thumbnailUrls.clear();
  }

  getThumbnail(id: string): string | undefined {
    return this.thumbnailUrls.get(id);
  }

  onSortChange(): void {
    this.applySort();
  }

  openLightbox(item: GalleryItem): void {
    this.lightboxOpen.emit(item);
  }

  private async loadThumbnails(items: GalleryItem[]): Promise<void> {
    // Revoke old URLs
    for (const url of this.thumbnailUrls.values()) {
      URL.revokeObjectURL(url);
    }
    this.thumbnailUrls.clear();

    // Load new thumbnails for image items
    const imageItems = items.filter((i) => i.type === 'image');
    const results = await Promise.allSettled(
      imageItems.map(async (item) => {
        const record = await this.fileRepo.get(item.id);
        if (record) {
          const url = URL.createObjectURL(record.data);
          return { id: item.id, url };
        }
        return null;
      }),
    );

    for (const result of results) {
      if (result.status === 'fulfilled' && result.value) {
        this.thumbnailUrls.set(result.value.id, result.value.url);
      }
    }
  }

  private applySort(): void {
    const sorted = [...this.items];

    switch (this.sortField()) {
      case 'name':
        sorted.sort((a, b) => a.name.localeCompare(b.name));
        break;
      case 'date':
        sorted.sort(
          (a, b) =>
            new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime(),
        );
        break;
      case 'type':
        sorted.sort((a, b) => {
          if (a.type !== b.type) return a.type.localeCompare(b.type);
          return a.name.localeCompare(b.name);
        });
        break;
    }

    this.sortedItems.set(sorted);
  }

  protected formatSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  }

  protected formatDate(date: Date | string): string {
    const d = new Date(date);
    return d.toLocaleDateString('pt-BR', {
      day: '2-digit',
      month: 'short',
      year: 'numeric',
    });
  }
}
