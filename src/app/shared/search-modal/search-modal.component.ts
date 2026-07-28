import {
  Component,
  inject,
  ViewChild,
  ElementRef,
  AfterViewInit,
  OnDestroy,
  HostListener,
  signal,
} from '@angular/core';
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
import { SearchService } from '../../core/services/search.service';
import type { SearchResult } from '../../core/models/shared';
import type { EntityType } from '../../core/models/shared';

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
        @if (results().length === 0 && query()) {
          <div class="empty-state">
            <mat-icon class="empty-icon">search_off</mat-icon>
            <p>Nenhum resultado encontrado</p>
          </div>
        } @else if (results().length === 0 && !query()) {
          <div class="empty-state">
            <mat-icon class="empty-icon">search</mat-icon>
            <p>Digite para pesquisar</p>
            <span class="empty-hint">Pressione Escape para fechar</span>
          </div>
        } @else {
          <mat-nav-list>
            @for (result of results(); track result.id + result.type) {
              <a
                mat-list-item
                class="search-result-item"
                (click)="selectResult(result)"
              >
                <mat-icon matListItemIcon>{{ resultIcon(result.type) }}</mat-icon>
                <div matListItemTitle class="result-name">{{ result.name }}</div>
                <div matListItemLine class="result-description">
                  {{ result.description }}
                </div>
                <span matListItemMeta class="result-type-badge">{{
                  resultLabel(result.type)
                }}</span>
              </a>
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
        max-height: 500px;
        display: flex;
        flex-direction: column;
      }

      .search-input-wrapper {
        padding: 16px 16px 0;
      }

      .search-field {
        width: 100%;
      }

      .search-results {
        flex: 1;
        overflow-y: auto;
        min-height: 120px;
      }

      .empty-state {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        padding: 40px 16px;
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

      .search-result-item {
        cursor: pointer;
      }

      .result-name {
        font-weight: 500;
        font-size: 0.95rem;
      }

      .result-description {
        font-size: 0.8rem;
        opacity: 0.65;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
        line-height: 1.4;
      }

      .result-type-badge {
        font-size: 0.7rem;
        opacity: 0.5;
        text-transform: uppercase;
        letter-spacing: 0.5px;
      }
    `,
  ],
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
    character: 'Personagem',
    campaign: 'Campanha',
    gallery: 'Galeria',
    map: 'Mapa',
    session: 'Sessão',
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
    const route = this.typeRouteMap[result.type] ?? '/';
    this.router.navigate([route]);
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
