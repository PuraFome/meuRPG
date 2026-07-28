import {
  Component,
  inject,
  Input,
  OnInit,
  OnDestroy,
  signal,
} from '@angular/core';
import { RouterLink } from '@angular/router';

import { firstValueFrom } from 'rxjs';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { PdfViewerModule } from 'ng2-pdf-viewer';
import { PageHeaderComponent } from '../../shared/components/page-header.component';
import { LoadingSpinnerComponent } from '../../shared/components/loading-spinner.component';
import { StoreService, IndexedDbFileRepository } from '../../core';
import type { RuleBook } from '../../core';

@Component({
  selector: 'app-rules-reader',
  standalone: true,
  imports: [
    RouterLink,
    MatButtonModule,
    MatIconModule,
    PdfViewerModule,
    PageHeaderComponent,
    LoadingSpinnerComponent,
  ],
  template: `
    <app-page-header
      [title]="ruleName()"
      [breadcrumbs]="[
        { label: 'Regras', route: '/regras' },
        { label: ruleName() }
      ]"
    />

    @if (isLoading()) {
      <app-loading-spinner
        [isLoading]="true"
        message="Carregando PDF..."
      />
    } @else if (error()) {
      <div class="error-state">
        <p>{{ error() }}</p>
        <button mat-button routerLink="/regras">Voltar</button>
      </div>
    } @else {
      <div class="reader-toolbar">
        <button
          mat-icon-button
          (click)="previousPage()"
          [disabled]="page() <= 1"
          aria-label="Página anterior"
        >
          <mat-icon>chevron_left</mat-icon>
        </button>

        <span class="page-info">
          Página
          <input
            class="page-input"
            type="number"
            [value]="page()"
            (change)="goToPage($event)"
            (keydown.enter)="goToPage($event)"
            [min]="1"
            [max]="totalPages()"
          />
          de {{ totalPages() }}
        </span>

        <button
          mat-icon-button
          (click)="nextPage()"
          [disabled]="page() >= totalPages()"
          aria-label="Próxima página"
        >
          <mat-icon>chevron_right</mat-icon>
        </button>
      </div>

      <div class="pdf-container">
        <pdf-viewer
          [src]="pdfSrc()"
          [page]="page()"
          (pageChange)="page.set($event)"
          (after-load-complete)="onLoadComplete($event)"
          [render-text]="true"
          [show-all]="false"
          [original-size]="false"
          [fit-to-page]="true"
          [autoresize]="true"
        >
        </pdf-viewer>
      </div>
    }
  `,
  styles: [
    `
      .reader-toolbar {
        display: flex;
        align-items: center;
        justify-content: center;
        gap: 16px;
        padding: 8px 0;
        margin-bottom: 8px;
      }
      .page-info {
        display: flex;
        align-items: center;
        gap: 4px;
        font-size: 0.9rem;
        opacity: 0.8;
      }
      .page-input {
        width: 48px;
        text-align: center;
        padding: 4px;
        border: 1px solid rgba(255, 255, 255, 0.23);
        border-radius: 4px;
        background: transparent;
        color: inherit;
        font-size: inherit;
        -moz-appearance: textfield;
      }
      .page-input::-webkit-outer-spin-button,
      .page-input::-webkit-inner-spin-button {
        -webkit-appearance: none;
        margin: 0;
      }
      .page-input:focus {
        outline: none;
        border-color: #ce93d8;
      }
      .pdf-container {
        height: calc(100vh - 280px);
        overflow: auto;
      }
      .error-state {
        display: flex;
        flex-direction: column;
        align-items: center;
        gap: 16px;
        padding: 48px 16px;
        text-align: center;
        opacity: 0.7;
      }
    `,
  ],
})
export class RulesReaderComponent implements OnInit, OnDestroy {
  private readonly store = inject(StoreService<RuleBook>);
  private readonly fileRepo = inject(IndexedDbFileRepository);

  @Input() id!: string;

  readonly ruleName = signal('');
  readonly pdfSrc = signal<string | Uint8Array | undefined>(undefined);
  readonly page = signal(1);
  readonly totalPages = signal(0);
  readonly isLoading = signal(true);
  readonly error = signal<string | null>(null);

  private objectUrl: string | null = null;

  constructor() {
    // Configure pdf.js worker before pdf-viewer component initializes
    if (typeof window !== 'undefined') {
      (window as any)['pdfWorkerSrc'] =
        'pdfjs-dist/build/pdf.worker.min.mjs';
    }
  }

  async ngOnInit(): Promise<void> {
    try {
      this.isLoading.set(true);

      const rule = await firstValueFrom(this.store.get('rules', this.id));

      if (!rule) {
        this.error.set('Livro de regras não encontrado');
        return;
      }

      this.ruleName.set(rule.name);

      const fileRecord = await this.fileRepo.get(rule.pdfUrl);
      if (!fileRecord) {
        this.error.set('Arquivo PDF não encontrado');
        return;
      }

      this.objectUrl = URL.createObjectURL(fileRecord.data);
      this.pdfSrc.set(this.objectUrl);
    } catch (err) {
      console.error('Falha ao carregar PDF:', err);
      this.error.set('Erro ao carregar PDF');
    } finally {
      this.isLoading.set(false);
    }
  }

  ngOnDestroy(): void {
    if (this.objectUrl) {
      URL.revokeObjectURL(this.objectUrl);
    }
  }

  onLoadComplete(pdf: { numPages: number }): void {
    this.totalPages.set(pdf.numPages);
    // Reset to page 1 when a new PDF loads
    this.page.set(1);
  }

  previousPage(): void {
    if (this.page() > 1) {
      this.page.update((p) => p - 1);
    }
  }

  nextPage(): void {
    if (this.page() < this.totalPages()) {
      this.page.update((p) => p + 1);
    }
  }

  goToPage(event: Event): void {
    const input = event.target as HTMLInputElement;
    const pageNum = parseInt(input.value, 10);
    if (pageNum >= 1 && pageNum <= this.totalPages()) {
      this.page.set(pageNum);
    } else {
      input.value = String(this.page());
    }
  }
}
