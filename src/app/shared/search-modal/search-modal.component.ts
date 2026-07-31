import {
  Component,
  inject,
  ViewChild,
  ElementRef,
  AfterViewInit,
  OnDestroy,
  HostListener,
  signal,
  computed,
} from '@angular/core';
import { trigger, transition, style, animate } from '@angular/animations';
import { Router } from '@angular/router';
import { Subject, Subscription } from 'rxjs';
import { debounceTime, distinctUntilChanged, switchMap } from 'rxjs/operators';
import { MatDialogModule, MatDialogRef } from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatListModule } from '@angular/material/list';
import { MatIconModule } from '@angular/material/icon';
import { FormsModule } from '@angular/forms';
import { MatDividerModule } from '@angular/material/divider';
import { SearchService, SearchResultGroup } from '../../core/services/search.service';
import type { SearchResult } from '../../core/models/shared';
import type { EntityType } from '../../core/models/shared';
import { SearchHighlightPipe } from './search-highlight.pipe';

@Component({
  selector: 'app-search-modal',
  standalone: true,
  imports: [
    FormsModule,
    MatDialogModule,
    MatFormFieldModule,
    MatInputModule,
    MatListModule,
    MatIconModule,
    MatDividerModule,
    SearchHighlightPipe,
  ],
  template: `
    <div class="search-modal" (keydown)="onKeydown($event)">
      <div class="search-input-wrapper">
        <mat-form-field appearance="outline" class="search-field">
          <mat-icon matPrefix>search</mat-icon>
          <input
            #searchInput
            matInput
            [ngModel]="query()"
            (ngModelChange)="onSearchInput($event)"
            placeholder="Pesquisar personagens, campanhas, regras..."
            autocomplete="off"
          />
          @if (query()) {
            <button matSuffix mat-icon-button aria-label="Limpar" (click)="clearSearch()">
              <mat-icon>close</mat-icon>
            </button>
          }
        </mat-form-field>
      </div>

      <mat-divider />

      <div class="search-results">
        @if (query() && groupedResults().length === 0) {
          <div class="empty-state">
            <mat-icon class="empty-icon">search_off</mat-icon>
            <p>Nenhum resultado encontrado</p>
          </div>
        } @else if (!query()) {
          <div class="empty-state">
            <mat-icon class="empty-icon">search</mat-icon>
            <p>Digite para pesquisar</p>
            <span class="empty-hint">Pressione Escape para fechar</span>
          </div>
        } @else {
          <mat-nav-list class="results-list">
            @for (group of groupedResults(); track group.type) {
              <!-- Section header -->
              <div class="group-header">
                <mat-icon class="group-icon">{{ group.icon }}</mat-icon>
                <span class="group-label">{{ group.label }}</span>
                <span class="group-count">{{ group.items.length }}</span>
              </div>
              <mat-divider class="group-divider" />

              @for (result of group.items; track result.id + result.type) {
                <a
                  mat-list-item
                  class="search-result-item"
                  (click)="selectResult(result)"
                >
                  <mat-icon
                    matListItemIcon
                    class="result-item-icon icon-{{ result.type }}"
                  >
                    {{ group.icon }}
                  </mat-icon>
                  <div
                    matListItemTitle
                    class="result-name"
                    [innerHTML]="result.name | searchHighlight: query()"
                  ></div>
                  @if (result.description) {
                    <div
                      matListItemLine
                      class="result-description"
                      [innerHTML]="result.description | searchHighlight: query()"
                    ></div>
                  }
                  <span
                    matListItemMeta
                    class="result-type-badge badge-{{ result.type }}"
                  >
                    {{ resultLabel(result.type) }}
                  </span>
                </a>
              }
            }
          </mat-nav-list>
        }
      </div>
    </div>
  `,
  styles: [
    `
      .search-modal {
        min-height: 200px;
        max-height: 520px;
        display: flex;
        flex-direction: column;
      }

      .search-input-wrapper {
        padding: 16px 16px 0;
        flex-shrink: 0;
      }

      .search-field {
        width: 100%;
      }

      .search-results {
        flex: 1;
        overflow-y: auto;
        min-height: 120px;
      }

      /* ── Empty states ─────────────────────────────── */
      .empty-state {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        padding: 48px 16px;
        opacity: 0.55;
        text-align: center;
      }

      .empty-icon {
        font-size: 2.5rem;
        width: 2.5rem;
        height: 2.5rem;
        margin-bottom: 8px;
      }

      .empty-state p {
        margin: 0 0 4px;
        font-size: 0.95rem;
      }

      .empty-hint {
        font-size: 0.8rem;
        opacity: 0.6;
      }

      /* ── Group headers ────────────────────────────── */
      .group-header {
        display: flex;
        align-items: center;
        gap: 8px;
        padding: 14px 16px 6px;
        font-size: 0.75rem;
        text-transform: uppercase;
        letter-spacing: 1px;
        color: rgba(255, 255, 255, 0.45);
      }

      .group-header:first-child {
        padding-top: 8px;
      }

      .group-icon {
        font-size: 16px;
        width: 16px;
        height: 16px;
        line-height: 16px;
        opacity: 0.6;
      }

      .group-label {
        flex: 1;
        font-weight: 700;
      }

      .group-count {
        font-size: 0.65rem;
        background: rgba(255, 255, 255, 0.08);
        padding: 1px 10px;
        border-radius: 10px;
        line-height: 20px;
        font-weight: 600;
      }

      .group-divider {
        margin: 4px 16px 2px;
      }

      /* ── Results list ─────────────────────────────── */
      .results-list {
        padding: 0 0 8px;
      }

      .search-result-item {
        cursor: pointer;
        border-radius: 8px;
        margin: 1px 8px;
        padding-block: 4px;
        --mat-list-active-indicator-shape: 8px;
        transition: background 0.15s ease;
      }

      .search-result-item:hover {
        background: rgba(255, 255, 255, 0.04);
      }

      .result-item-icon {
        font-size: 20px;
        width: 20px;
        height: 20px;
        line-height: 20px;
        opacity: 0.7;
      }

      .result-name {
        font-weight: 500;
        font-size: 0.92rem;
        line-height: 1.4;
      }

      .result-description {
        font-size: 0.78rem !important;
        opacity: 0.55;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
        line-height: 1.4;
        padding-right: 8px;
      }

      /* ── Search highlight ─────────────────────────── */
      :host ::ng-deep .search-highlight {
        background: rgba(255, 215, 0, 0.25);
        color: #ffd740;
        border-radius: 2px;
        padding: 0 1px;
        font-weight: 600;
      }

      /* ── Type badge chips ─────────────────────────── */
      .result-type-badge {
        font-size: 0.62rem;
        font-weight: 700;
        letter-spacing: 0.6px;
        padding: 2px 10px;
        border-radius: 4px;
        text-transform: uppercase;
        white-space: nowrap;
      }

      .badge-character {
        background: rgba(124, 77, 255, 0.18);
        color: #b388ff;
        border: 1px solid rgba(124, 77, 255, 0.25);
      }

      .badge-campaign {
        background: rgba(0, 191, 165, 0.18);
        color: #64ffda;
        border: 1px solid rgba(0, 191, 165, 0.25);
      }

      .badge-gallery {
        background: rgba(255, 109, 0, 0.18);
        color: #ffab40;
        border: 1px solid rgba(255, 109, 0, 0.25);
      }

      .badge-rules {
        background: rgba(255, 171, 0, 0.18);
        color: #ffd740;
        border: 1px solid rgba(255, 171, 0, 0.25);
      }

      .badge-map {
        background: rgba(68, 138, 255, 0.18);
        color: #82b1ff;
        border: 1px solid rgba(68, 138, 255, 0.25);
      }

      .badge-session {
        background: rgba(255, 64, 129, 0.18);
        color: #ff80ab;
        border: 1px solid rgba(255, 64, 129, 0.25);
      }

      /* ── Result icon colors ───────────────────────── */
      .icon-character {
        color: #b388ff;
      }

      .icon-campaign {
        color: #64ffda;
      }

      .icon-gallery {
        color: #ffab40;
      }

      .icon-rules {
        color: #ffd740;
      }

      .icon-map {
        color: #82b1ff;
      }

      .icon-session {
        color: #ff80ab;
      }
    `,
  ],
  animations: [
    trigger('fadeSlide', [
      transition(':enter', [
        style({ opacity: 0, transform: 'translateY(-12px)' }),
        animate('250ms ease-out', style({ opacity: 1, transform: 'translateY(0)' })),
      ]),
    ]),
  ],
  host: { '[@fadeSlide]': '' },
})
export class SearchModalComponent implements AfterViewInit, OnDestroy {
  private readonly dialogRef = inject(MatDialogRef<SearchModalComponent>);
  private readonly router = inject(Router);
  private readonly searchService = inject(SearchService);

  @ViewChild('searchInput') searchInput!: ElementRef<HTMLInputElement>;

  private readonly searchSubject = new Subject<string>();
  private readonly subscription = new Subscription();

  readonly query = signal('');
  readonly results = signal<SearchResult[]>([]);

  readonly groupedResults = computed(() => {
    const items = this.results();
    const typeOrder: EntityType[] = [
      'character',
      'campaign',
      'gallery',
      'rules',
      'map',
      'session',
    ];
    const grouped = new Map<EntityType, SearchResult[]>();

    for (const item of items) {
      const group = grouped.get(item.type);
      if (group) {
        group.push(item);
      } else {
        grouped.set(item.type, [item]);
      }
    }

    return typeOrder
      .filter((type) => grouped.has(type))
      .map((type) => ({
        type,
        label: this.typeLabelMap[type],
        icon: this.typeIconMap[type],
        items: grouped.get(type)!,
      }));
  });

  private readonly typeRouteMap: Record<EntityType, string> = {
    character: '/personagens',
    campaign: '/campanha',
    gallery: '/galeria',
    map: '/mapa',
    session: '/sessao',
    rules: '/regras',
  };

  private readonly typeIconMap: Record<EntityType, string> = {
    character: 'person',
    campaign: 'folder',
    gallery: 'collections_bookmark',
    map: 'map',
    session: 'event',
    rules: 'menu_book',
  };

  private readonly typeLabelMap: Record<EntityType, string> = {
    character: 'Personagens',
    campaign: 'Campanha',
    gallery: 'Galeria',
    map: 'Mapas',
    session: 'Sessões',
    rules: 'Regras',
  };

  ngAfterViewInit(): void {
    // Auto-focus the search input
    setTimeout(() => this.searchInput?.nativeElement?.focus(), 80);

    const sub = this.searchSubject
      .pipe(
        debounceTime(300),
        distinctUntilChanged(),
        switchMap((query) => this.searchService.search(query)),
      )
      .subscribe((items) => {
        this.results.set(items);
      });

    this.subscription.add(sub);

    // Load initial results (all indexed items)
    this.searchSubject.next('');
  }

  ngOnDestroy(): void {
    this.subscription.unsubscribe();
  }

  @HostListener('keydown', ['$event'])
  onKeydown(event: KeyboardEvent): void {
    if (event.key === 'Escape') {
      this.dialogRef.close();
    }
  }

  onSearchInput(value: string): void {
    this.query.set(value);
    this.searchSubject.next(value);
  }

  selectResult(result: SearchResult): void {
    this.dialogRef.close();
    const baseRoute = this.typeRouteMap[result.type] ?? '/';

    if (result.type === 'character' || result.type === 'map' || result.type === 'rules') {
      const entityId = (result.entity as { id: string }).id;
      this.router.navigate([baseRoute, entityId]);
    } else {
      this.router.navigate([baseRoute]);
    }
  }

  clearSearch(): void {
    this.query.set('');
    this.results.set([]);
    this.searchSubject.next('');
    this.searchInput?.nativeElement?.focus();
  }

  resultIcon(type: EntityType): string {
    return this.typeIconMap[type] ?? 'search';
  }

  resultLabel(type: EntityType): string {
    return this.typeLabelMap[type] ?? type;
  }
}
