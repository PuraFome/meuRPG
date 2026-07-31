import { Component, inject } from '@angular/core';
import { AsyncPipe } from '@angular/common';
import { Router, RouterLink } from '@angular/router';
import { BehaviorSubject, combineLatest, Subject } from 'rxjs';
import {
  debounceTime,
  distinctUntilChanged,
  map,
  shareReplay,
  startWith,
} from 'rxjs/operators';
import { MatCardModule } from '@angular/material/card';
import { MatChipsModule } from '@angular/material/chips';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatIconModule } from '@angular/material/icon';
import { MatButtonModule } from '@angular/material/button';
import { StoreService } from '../../core/store/store.service';
import { EmptyStateComponent } from '../../shared/components/empty-state.component';
import { LoadingSpinnerComponent } from '../../shared/components/loading-spinner.component';
import type { Character } from '../../core/models/character';

@Component({
  selector: 'app-character-list',
  standalone: true,
  imports: [
    AsyncPipe,
    RouterLink,
    MatCardModule,
    MatChipsModule,
    MatFormFieldModule,
    MatInputModule,
    MatIconModule,
    MatButtonModule,
    EmptyStateComponent,
    LoadingSpinnerComponent,
  ],
  template: `
    <div class="characters-page">
      <!-- Header -->
      <div class="page-header">
        <h1 class="page-title">Personagens</h1>
        <button mat-raised-button color="primary" routerLink="novo">
          <mat-icon>add</mat-icon>
          Novo Personagem
        </button>
      </div>

      <!-- Filters -->
      <div class="filters-bar">
        <mat-form-field appearance="outline" class="search-field" subscriptSizing="dynamic">
          <mat-icon matPrefix>search</mat-icon>
          <input
            matInput
            placeholder="Buscar personagem..."
            (input)="onSearchInput($event)"
          />
        </mat-form-field>

        <mat-chip-listbox
          [value]="activeType"
          (change)="setType($event.value)"
          class="type-filters"
          aria-label="Filtro por tipo"
        >
          <mat-chip-option value="all">Todos</mat-chip-option>
          <mat-chip-option value="npc">NPC</mat-chip-option>
          <mat-chip-option value="player">Jogador</mat-chip-option>
          <mat-chip-option value="boss">Boss</mat-chip-option>
        </mat-chip-listbox>
      </div>

      <!-- Content -->
      @if (loading$ | async) {
        <app-loading-spinner [isLoading]="true" message="Carregando personagens..." />
      } @else {
        @if ((filteredCharacters$ | async); as characters) {
          @if (characters.length === 0) {
            <app-empty-state
              icon="person_search"
              message="Nenhum personagem encontrado"
              actionLabel="Criar Personagem"
              (action)="onNewCharacter()"
            />
          } @else {
            <div class="character-grid">
              @for (character of characters; track character.id) {
                <mat-card
                  class="character-card"
                  [routerLink]="[character.id]"
                >
                  <div class="card-avatar" [class]="'card-avatar type-' + character.type">
                    {{ character.name.charAt(0).toUpperCase() }}
                  </div>
                  <mat-card-content>
                    <h3 class="card-name">{{ character.name }}</h3>
                    <span class="type-badge" [class]="'type-badge type-' + character.type">
                      {{ typeLabel(character.type) }}
                    </span>
                    <p class="card-description">{{ character.description }}</p>
                  </mat-card-content>
                </mat-card>
              }
            </div>
          }
        }
      }
    </div>
  `,
  styles: `
    :host {
      display: block;
      max-width: 1200px;
      margin: 0 auto;
    }

    .characters-page {
      padding: 8px 0;
    }

    /* ── Header ────────────────────────────────── */

    .page-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 16px;
      margin-bottom: 24px;
      flex-wrap: wrap;
    }

    .page-title {
      margin: 0;
      font-size: 1.75rem;
      font-weight: 700;
      letter-spacing: 0.5px;
      background: linear-gradient(135deg, #e0e0e0 0%, #b388ff 100%);
      -webkit-background-clip: text;
      -webkit-text-fill-color: transparent;
      background-clip: text;
    }

    /* ── Filters ───────────────────────────────── */

    .filters-bar {
      display: flex;
      align-items: center;
      gap: 16px;
      margin-bottom: 24px;
      flex-wrap: wrap;
    }

    .search-field {
      flex: 1;
      min-width: 220px;
      max-width: 400px;
    }

    .type-filters {
      display: flex;
      flex-wrap: wrap;
    }

    /* ── Grid ──────────────────────────────────── */

    .character-grid {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
      gap: 20px;
    }

    .character-card {
      cursor: pointer;
      transition: transform 0.2s ease, box-shadow 0.2s ease, border-color 0.2s ease;
      border: 1px solid rgba(255, 255, 255, 0.06);
      border-radius: 12px;
      overflow: hidden;
    }

    .character-card:hover {
      transform: translateY(-4px);
      box-shadow: 0 8px 24px rgba(0, 0, 0, 0.35);
      border-color: rgba(255, 255, 255, 0.12);
    }

    /* ── Avatar ────────────────────────────────── */

    .card-avatar {
      width: 100%;
      height: 100px;
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 2.5rem;
      font-weight: 700;
      color: #fff;
      text-shadow: 0 2px 6px rgba(0, 0, 0, 0.3);
      letter-spacing: 1px;
    }

    .card-avatar.type-player {
      background: linear-gradient(135deg, #065f46, #059669);
    }

    .card-avatar.type-npc {
      background: linear-gradient(135deg, #155e75, #0891b2);
    }

    .card-avatar.type-boss {
      background: linear-gradient(135deg, #7f1d1d, #dc2626);
    }

    /* ── Card content ──────────────────────────── */

    .card-name {
      margin: 12px 0 6px;
      font-size: 1.1rem;
      font-weight: 600;
    }

    .type-badge {
      display: inline-block;
      padding: 2px 10px;
      border-radius: 12px;
      font-size: 0.75rem;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.5px;
      color: #fff;
    }

    .type-badge.type-player {
      background: #059669;
    }

    .type-badge.type-npc {
      background: #0891b2;
    }

    .type-badge.type-boss {
      background: #dc2626;
    }

    .card-description {
      margin: 10px 0 0;
      font-size: 0.875rem;
      opacity: 0.6;
      line-height: 1.5;
      display: -webkit-box;
      -webkit-line-clamp: 2;
      -webkit-box-orient: vertical;
      overflow: hidden;
    }
  `,
})
export class CharacterListComponent {
  private readonly store = inject(StoreService<Character>);
  private readonly router = inject(Router);

  /** Current type filter value. */
  activeType = 'all';
  private readonly typeFilterSubject = new BehaviorSubject<string>('all');
  private readonly typeFilter$ = this.typeFilterSubject.asObservable();

  /** Search input stream with debounce. */
  private readonly searchSubject = new Subject<string>();
  private readonly search$ = this.searchSubject.pipe(
    debounceTime(300),
    distinctUntilChanged(),
    startWith(''),
  );

  /** Raw character stream from store (shared to avoid deadlocking loading / filtered). */
  private readonly allCharacters$ = this.store.getAll('characters').pipe(
    shareReplay({ bufferSize: 1, refCount: true }),
  );

  /** Loading is true until first emission from the store. */
  readonly loading$ = this.allCharacters$.pipe(
    map(() => false),
    startWith(true),
  );

  /** Characters filtered by type + search. */
  readonly filteredCharacters$ = combineLatest([
    this.allCharacters$,
    this.typeFilter$,
    this.search$,
  ]).pipe(
    map(([characters, type, search]) =>
      characters.filter((c) => {
        const matchesType = type === 'all' || c.type === type;
        const matchesSearch =
          !search ||
          c.name.toLowerCase().includes((search as string).toLowerCase());
        return matchesType && matchesSearch;
      }),
    ),
  );

  /** Human-readable label for each character type. */
  typeLabel(type: 'npc' | 'player' | 'boss'): string {
    switch (type) {
      case 'player':
        return 'Jogador';
      case 'npc':
        return 'NPC';
      case 'boss':
        return 'Boss';
    }
  }

  /** Update the active type filter. */
  setType(type: string): void {
    this.activeType = type;
    this.typeFilterSubject.next(type);
  }

  /** Emit the raw search input value (debounced internally). */
  onSearchInput(event: Event): void {
    this.searchSubject.next((event.target as HTMLInputElement).value);
  }

  /** Navigate to the new-character screen. */
  onNewCharacter(): void {
    this.router.navigate(['/personagens', 'novo']);
  }
}
