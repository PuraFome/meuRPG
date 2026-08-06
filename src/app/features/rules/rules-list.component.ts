import { Component, inject, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { AsyncPipe, DatePipe } from '@angular/common';
import { Observable } from 'rxjs';
import { MatCardModule } from '@angular/material/card';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { PageHeaderComponent } from '../../shared/components/page-header.component';
import { FileUploadComponent } from '../../shared/components/file-upload.component';
import { EmptyStateComponent } from '../../shared/components/empty-state.component';
import { LoadingSpinnerComponent } from '../../shared/components/loading-spinner.component';
import { StoreService, IndexedDbFileRepository } from '../../core';
import type { RuleBook } from '../../core';

@Component({
  selector: 'app-rules-list',
  standalone: true,
  imports: [
    AsyncPipe,
    DatePipe,
    RouterLink,
    MatCardModule,
    MatButtonModule,
    MatIconModule,
    PageHeaderComponent,
    FileUploadComponent,
    EmptyStateComponent,
    LoadingSpinnerComponent,
  ],
  template: `
    <app-page-header
      title="Regras"
      icon="menu_book"
      [breadcrumbs]="[{ label: 'Regras' }]"
    />

    <div class="actions-bar">
      <button
        mat-raised-button
        color="primary"
        (click)="showUpload.set(true)"
        [disabled]="isUploading()"
      >
        <mat-icon>add</mat-icon>
        Adicionar Livro
      </button>
    </div>

    @if (showUpload()) {
      <div class="upload-section">
        <app-file-upload
          [acceptedTypes]="['application/pdf']"
          [maxSize]="50 * 1024 * 1024"
          (fileChange)="onFileSelected($event)"
        />
        <button
          mat-button
          class="cancel-btn"
          (click)="showUpload.set(false)"
        >
          Cancelar
        </button>
      </div>
    }

    @if (isUploading()) {
      <app-loading-spinner
        [isLoading]="true"
        message="Enviando PDF..."
      />
    }

    <div class="rules-grid">
      @for (rule of rules$ | async; track rule.id) {
        <mat-card
          class="rule-card"
          [routerLink]="[rule.id]"
        >
          <mat-card-header>
            <mat-icon mat-card-avatar>menu_book</mat-icon>
            <mat-card-title>{{ rule.name }}</mat-card-title>
            <mat-card-subtitle>
              {{ rule.createdAt | date:'shortDate' }}
              @if (rule.description) {
                &nbsp;— {{ rule.description }}
              }
            </mat-card-subtitle>
          </mat-card-header>
        </mat-card>
      } @empty {
        @if (!isUploading()) {
          <app-empty-state
            icon="menu_book"
            message="Nenhum livro de regras encontrado"
            actionLabel="Adicionar Livro"
            (action)="showUpload.set(true)"
          />
        }
      }
    </div>
  `,
  styles: [
    `
      .actions-bar {
        display: flex;
        justify-content: flex-end;
        margin-bottom: 16px;
      }
      .upload-section {
        margin-bottom: 16px;
      }
      .cancel-btn {
        margin-top: 8px;
      }
      .rules-grid {
        display: flex;
        flex-direction: column;
        gap: 12px;
      }
      .rule-card {
        cursor: pointer;
        transition: opacity 0.2s;
      }
      .rule-card:hover {
        opacity: 0.8;
      }
    `,
  ],
})
export class RulesListComponent {
  private readonly store = inject(StoreService<RuleBook>);
  private readonly fileRepo = inject(IndexedDbFileRepository);

  readonly rules$: Observable<RuleBook[]> = this.store.getAll('rules');
  readonly showUpload = signal(false);
  readonly isUploading = signal(false);

  async onFileSelected(file: File): Promise<void> {
    this.isUploading.set(true);

    try {
      const fileId = crypto.randomUUID();
      await this.fileRepo.save({
        id: fileId,
        name: file.name,
        data: file,
        mimeType: file.type,
        size: file.size,
        uploadedAt: new Date(),
      });

      const name = file.name.replace(/\.pdf$/i, '');
      this.store.set('rules', {
        id: crypto.randomUUID(),
        name,
        pdfUrl: fileId,
        description: '',
        createdAt: new Date(),
      });

      this.showUpload.set(false);
    } catch (err) {
      console.error('Falha ao enviar PDF:', err);
    } finally {
      this.isUploading.set(false);
    }
  }
}
