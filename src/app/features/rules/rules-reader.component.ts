import {
  Component,
  inject,
  Input,
  OnInit,
  OnDestroy,
  signal,
  HostListener,
} from '@angular/core';
import { Router } from '@angular/router';

import { firstValueFrom } from 'rxjs';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { PdfViewerModule } from 'ng2-pdf-viewer';
import { PageHeaderComponent } from '../../shared/components/page-header.component';
import { LoadingSpinnerComponent } from '../../shared/components/loading-spinner.component';
import { EmptyStateComponent } from '../../shared/components/empty-state.component';
import { StoreService, IndexedDbFileRepository } from '../../core';
import type { RuleBook } from '../../core';
import { GlobalWorkerOptions } from 'pdfjs-dist';

interface FlatBookmark {
  title: string;
  dest: string | any[] | null;
  url: string | null;
  level: number;
  bold: boolean;
  italic: boolean;
}

interface PdfOutlineItem {
  title: string;
  dest: string | any[] | null;
  url: string | null;
  bold: boolean;
  italic: boolean;
  items?: PdfOutlineItem[];
}

interface PdfDocument {
  numPages: number;
  getPage(pageNum: number): Promise<any>;
  getOutline(): Promise<PdfOutlineItem[] | null>;
  getDestination(dest: string): Promise<any>;
  getPageIndex(ref: any): Promise<number>;
}

function flattenOutline(items: PdfOutlineItem[], level: number = 0): FlatBookmark[] {
  const result: FlatBookmark[] = [];
  for (const item of items) {
    result.push({
      title: item.title,
      dest: item.dest,
      url: item.url,
      level,
      bold: item.bold,
      italic: item.italic,
    });
    if (item.items?.length) {
      result.push(...flattenOutline(item.items, level + 1));
    }
  }
  return result;
}

@Component({
  selector: 'app-rules-reader',
  standalone: true,
  imports: [
    MatButtonModule,
    MatIconModule,
    PdfViewerModule,
    PageHeaderComponent,
    LoadingSpinnerComponent,
    EmptyStateComponent,
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
        <app-empty-state
          icon="menu_book"
          [message]="error()!"
          actionLabel="Tentar Novamente"
          (action)="retry()"
        />
      </div>
    } @else {
      <div class="reader-toolbar">
        <div class="toolbar-section toolbar-left">
          <button
            mat-icon-button
            (click)="toggleSidebar()"
            [class.active]="sidebarOpen()"
            aria-label="Alternar sumário"
          >
            <mat-icon>bookmark</mat-icon>
          </button>
        </div>

        <div class="toolbar-section toolbar-center">
          <button
            mat-icon-button
            (click)="firstPage()"
            [disabled]="page() <= 1"
            aria-label="Primeira página"
          >
            <mat-icon>first_page</mat-icon>
          </button>
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
          <button
            mat-icon-button
            (click)="lastPage()"
            [disabled]="page() >= totalPages()"
            aria-label="Última página"
          >
            <mat-icon>last_page</mat-icon>
          </button>
        </div>

        <div class="toolbar-section toolbar-right">
          <button
            mat-icon-button
            (click)="zoomOut()"
            [disabled]="zoom() <= 0.25"
            aria-label="Reduzir zoom"
          >
            <mat-icon>zoom_out</mat-icon>
          </button>
          <span class="zoom-info">{{ zoomPercent() }}</span>
          <button
            mat-icon-button
            (click)="zoomIn()"
            [disabled]="zoom() >= 5"
            aria-label="Aumentar zoom"
          >
            <mat-icon>zoom_in</mat-icon>
          </button>
          <button
            mat-icon-button
            (click)="zoomToFit()"
            aria-label="Zoom para ajustar"
          >
            <mat-icon>aspect_ratio</mat-icon>
          </button>
        </div>
      </div>

      <div class="reader-layout">
        @if (sidebarOpen()) {
          <aside class="bookmarks-sidebar">
            <div class="bookmarks-header">Sumário</div>
            <div class="bookmarks-list">
              @for (bookmark of bookmarks(); track $index) {
                <button
                  class="bookmark-item"
                  [style.paddingLeft.px]="12 + bookmark.level * 16"
                  (click)="navigateToBookmark(bookmark.dest, bookmark.url)"
                  [class.bold]="bookmark.bold"
                  [class.italic]="bookmark.italic"
                >
                  <span class="bookmark-title">{{ bookmark.title }}</span>
                </button>
              } @empty {
                <div class="no-bookmarks">Nenhum sumário disponível</div>
              }
            </div>
          </aside>
        }

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
            [zoom]="zoom()"
            [autoresize]="true"
          >
          </pdf-viewer>
        </div>
      </div>
    }
  `,
  styles: [
    `
      .reader-toolbar {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 8px;
        padding: 4px 0;
        margin-bottom: 4px;
        flex-wrap: wrap;
      }
      .toolbar-section {
        display: flex;
        align-items: center;
        gap: 2px;
      }
      .toolbar-left {
        flex: 0 0 auto;
      }
      .toolbar-center {
        flex: 1 1 auto;
        justify-content: center;
      }
      .toolbar-right {
        flex: 0 0 auto;
      }
      .toolbar-right .mat-icon {
        font-size: 20px;
      }
      .page-info {
        display: flex;
        align-items: center;
        gap: 4px;
        font-size: 0.85rem;
        opacity: 0.8;
        white-space: nowrap;
      }
      .page-input {
        width: 44px;
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
      .zoom-info {
        font-size: 0.8rem;
        min-width: 36px;
        text-align: center;
        opacity: 0.8;
        font-variant-numeric: tabular-nums;
      }

      .reader-layout {
        display: flex;
        gap: 0;
        height: calc(100vh - 280px);
      }

      .bookmarks-sidebar {
        width: 240px;
        min-width: 240px;
        overflow-y: auto;
        border-right: 1px solid rgba(255, 255, 255, 0.12);
        background: rgba(0, 0, 0, 0.12);
        display: flex;
        flex-direction: column;
      }
      .bookmarks-header {
        padding: 10px 12px;
        font-weight: 500;
        font-size: 0.85rem;
        text-transform: uppercase;
        letter-spacing: 0.5px;
        opacity: 0.7;
        border-bottom: 1px solid rgba(255, 255, 255, 0.08);
        flex-shrink: 0;
      }
      .bookmarks-list {
        padding: 4px 0;
        flex: 1;
        overflow-y: auto;
      }
      .bookmark-item {
        display: flex;
        align-items: center;
        width: 100%;
        border: none;
        background: transparent;
        color: inherit;
        cursor: pointer;
        padding: 6px 12px;
        font-size: 0.82rem;
        text-align: left;
        transition: background 0.15s;
        line-height: 1.3;
      }
      .bookmark-item:hover {
        background: rgba(255, 255, 255, 0.06);
      }
      .bookmark-item.bold .bookmark-title {
        font-weight: 600;
      }
      .bookmark-item.italic .bookmark-title {
        font-style: italic;
      }
      .bookmark-title {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
      .no-bookmarks {
        padding: 24px 16px;
        text-align: center;
        opacity: 0.5;
        font-size: 0.85rem;
      }

      .pdf-container {
        flex: 1;
        overflow: auto;
        height: 100%;
      }

      @media (max-width: 768px) {
        .reader-layout {
          flex-direction: column;
        }
        .bookmarks-sidebar {
          width: 100%;
          min-width: unset;
          max-height: 200px;
          border-right: none;
          border-bottom: 1px solid rgba(255, 255, 255, 0.12);
        }
        .pdf-container {
          width: 100%;
        }
      }

      .error-state {
        display: flex;
        justify-content: center;
        align-items: center;
        min-height: 300px;
      }
    `,
  ],
})
export class RulesReaderComponent implements OnInit, OnDestroy {
  private readonly store = inject(StoreService<RuleBook>);
  private readonly fileRepo = inject(IndexedDbFileRepository);
  private readonly router = inject(Router);

  @Input() id!: string;

  readonly ruleName = signal('');
  readonly pdfSrc = signal<string | Uint8Array | undefined>(undefined);
  readonly page = signal(1);
  readonly totalPages = signal(0);
  readonly isLoading = signal(true);
  readonly error = signal<string | null>(null);

  readonly sidebarOpen = signal(false);
  readonly bookmarks = signal<FlatBookmark[]>([]);
  readonly zoom = signal(1);
  readonly zoomPercent = signal('100%');

  private objectUrl: string | null = null;
  private pdfDocument: PdfDocument | null = null;

  constructor() {
    // Configure pdf.js worker before pdf-viewer component initializes
    if (typeof window !== 'undefined') {
      GlobalWorkerOptions.workerSrc =
        'pdfjs-dist/build/pdf.worker.min.mjs';
    }
  }

  async ngOnInit(): Promise<void> {
    await this.loadRule();
  }

  async retry(): Promise<void> {
    // Revoke old object URL before retrying
    if (this.objectUrl) {
      URL.revokeObjectURL(this.objectUrl);
      this.objectUrl = null;
    }
    this.pdfSrc.set(undefined);
    await this.loadRule();
  }

  private async loadRule(): Promise<void> {
    try {
      this.isLoading.set(true);
      this.error.set(null);

      const rule = await firstValueFrom(this.store.get('rules', this.id));

      if (!rule) {
        this.error.set('Regra não encontrada');
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

  onLoadComplete(pdf: PdfDocument): void {
    this.totalPages.set(pdf.numPages);
    this.page.set(1);
    this.pdfDocument = pdf;

    // Extract bookmarks/outline
    pdf.getOutline().then((outline: PdfOutlineItem[] | null) => {
      if (outline && outline.length > 0) {
        this.bookmarks.set(flattenOutline(outline));
      } else {
        this.bookmarks.set([]);
      }
    });
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

  firstPage(): void {
    this.page.set(1);
  }

  lastPage(): void {
    this.page.set(this.totalPages());
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

  toggleSidebar(): void {
    this.sidebarOpen.update((v) => !v);
  }

  zoomIn(): void {
    const step = this.zoom() < 1 ? 0.1 : 0.25;
    const next = Math.min(this.zoom() + step, 5);
    this.zoom.set(Math.round(next * 100) / 100);
    this.updateZoomPercent();
  }

  zoomOut(): void {
    const step = this.zoom() <= 1 ? 0.1 : 0.25;
    const next = Math.max(this.zoom() - step, 0.25);
    this.zoom.set(Math.round(next * 100) / 100);
    this.updateZoomPercent();
  }

  zoomToFit(): void {
    this.zoom.set(1);
    this.updateZoomPercent();
  }

  private updateZoomPercent(): void {
    this.zoomPercent.set(Math.round(this.zoom() * 100) + '%');
  }

  async navigateToBookmark(
    dest: string | any[] | null,
    url: string | null,
  ): Promise<void> {
    if (url) {
      window.open(url, '_blank');
      return;
    }
    if (!dest || !this.pdfDocument) return;

    try {
      if (typeof dest === 'string') {
        // Named destination — resolve it via pdfDocument
        const destArray = await this.pdfDocument.getDestination(dest);
        if (destArray && destArray.length > 0) {
          const ref = destArray[0];
          const pageIndex =
            typeof ref === 'number'
              ? ref
              : await this.pdfDocument.getPageIndex(ref);
          this.page.set(pageIndex + 1);
        }
      } else if (Array.isArray(dest) && dest.length > 0) {
        // Explicit destination — first element is the page reference
        const ref = dest[0];
        if (typeof ref === 'number') {
          this.page.set(ref + 1);
        } else if (ref && typeof ref === 'object') {
          const pageIndex = await this.pdfDocument.getPageIndex(ref);
          this.page.set(pageIndex + 1);
        }
      }
    } catch (err) {
      console.warn('Falha ao navegar para destino do marcador:', err);
    }
  }

  @HostListener('window:keydown', ['$event'])
  onKeydown(event: KeyboardEvent): void {
    // Ignore if user is typing in an input
    const target = event.target as HTMLElement;
    if (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA') return;

    switch (event.key) {
      case 'PageDown':
        event.preventDefault();
        this.nextPage();
        break;
      case 'PageUp':
        event.preventDefault();
        this.previousPage();
        break;
      case 'Home':
        event.preventDefault();
        this.firstPage();
        break;
      case 'End':
        event.preventDefault();
        this.lastPage();
        break;
    }
  }
}
