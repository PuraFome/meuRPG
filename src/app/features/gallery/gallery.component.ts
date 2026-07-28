import { Component, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { MatIconModule } from '@angular/material/icon';
import { MatButtonModule } from '@angular/material/button';
import { PageHeaderComponent } from '../../shared/components/page-header.component';
import { GalleryUploadComponent } from './gallery-upload.component';
import { GalleryGridComponent } from './gallery-grid.component';

@Component({
  selector: 'app-gallery',
  standalone: true,
  imports: [
    CommonModule,
    MatIconModule,
    MatButtonModule,
    PageHeaderComponent,
    GalleryUploadComponent,
    GalleryGridComponent,
  ],
  template: `
    <div class="gallery-page">
      <app-page-header
        title="Galeria"
        [breadcrumbs]="breadcrumbs"
      />

      <div class="gallery-content">
        <section class="upload-area">
          <div class="upload-header">
            <h3 class="upload-heading">
              <mat-icon class="heading-icon">add_photo_alternate</mat-icon>
              Adicionar Mídia
            </h3>
            <button
              mat-icon-button
              class="collapse-btn"
              (click)="showUpload.set(!showUpload())"
              [attr.aria-expanded]="showUpload()"
              aria-label="Alternar upload"
            >
              <mat-icon>{{ showUpload() ? 'expand_less' : 'expand_more' }}</mat-icon>
            </button>
          </div>

          @if (showUpload()) {
            <app-gallery-upload (uploadComplete)="onUploadComplete()" />
          }
        </section>

        <app-gallery-grid />
      </div>
    </div>
  `,
  styles: `
    :host {
      display: block;
    }

    .gallery-page {
      max-width: 1200px;
      margin: 0 auto;
    }

    .gallery-content {
      display: flex;
      flex-direction: column;
      gap: 24px;
      margin-top: 8px;
    }

    .upload-area {
      border-radius: 12px;
      background: rgba(255, 255, 255, 0.02);
      border: 1px solid rgba(255, 255, 255, 0.04);
      overflow: hidden;
    }

    .upload-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 12px 24px;
    }

    .upload-heading {
      display: flex;
      align-items: center;
      gap: 8px;
      margin: 0;
      font-size: 0.95rem;
      font-weight: 500;
      opacity: 0.7;
    }

    .heading-icon {
      font-size: 1.2rem;
      width: 1.2rem;
      height: 1.2rem;
      opacity: 0.6;
    }

    .collapse-btn {
      opacity: 0.4;
      transition: opacity 0.15s;
    }

    .collapse-btn:hover {
      opacity: 0.8;
    }
  `,
})
export class GalleryComponent {
  readonly breadcrumbs = [{ label: 'Galeria' }];
  readonly showUpload = signal(true);

  onUploadComplete(): void {
    // Auto-collapse upload section after successful upload
    this.showUpload.set(false);
  }
}
