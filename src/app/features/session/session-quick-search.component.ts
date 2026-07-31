import {
  Component,
  inject,
  signal,
  computed,
  ViewChild,
  ElementRef,
  AfterViewInit,
  OnDestroy,
  HostListener,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { Subject, Subscription } from 'rxjs';
import { debounceTime, distinctUntilChanged } from 'rxjs/operators';
import {
  MatDialogModule,
  MatDialogRef,
} from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatListModule } from '@angular/material/list';
import { MatChipsModule } from '@angular/material/chips';
import { StoreService } from '../../core';
import type { Character } from '../../core';
import type { RuleBook } from '../../core';

// ─── Types ─────────────────────────────────────────────────────

interface CharacterResult {
  kind: 'character';
  character: Character;
  matchType: 'name' | 'quote';
  quoteText?: string;
  quoteIndex?: number;
}

interface RuleResult {
  kind: 'rule';
  rule: RuleBook;
}

type SearchEntry = CharacterResult | RuleResult;

interface SearchGroup {
  category: 'personagens' | 'regras' | 'falas';
  icon: string;
  label: string;
  entries: SearchEntry[];
}

// ─── Component ─────────────────────────────────────────────────

@Component({
  selector: 'app-session-quick-search',
  standalone: true,
  imports: [
    CommonModule,
    FormsModule,
    MatDialogModule,
    MatFormFieldModule,
    MatInputModule,
    MatButtonModule,
    MatIconModule,
    MatListModule,
    MatChipsModule,
  ],
  template: `
    <div class="qs-container">
      <!-- ── Header with title + close ── -->
      <div class="qs-header">
        <span class="qs-title">Pesquisa Rápida</span>
        <button
          mat-icon-button
          class="qs-close-btn"
          (click)="close()"
          aria-label="Fechar"
        >
          <mat-icon>close</mat-icon>
        </button>
      </div>

      @if (!selectedEntry()) {
        <!-- ══════════ SEARCH VIEW ══════════ -->
        <div class="qs-body">
          <!-- Search input -->
          <div class="qs-search-field-wrapper">
            <mat-form-field appearance="outline" class="qs-search-field" subscriptSizing="dynamic">
              <mat-icon matPrefix>search</mat-icon>
              <input
                #searchInput
                matInput
                [ngModel]="query()"
                (ngModelChange)="onQueryChange($event)"
                placeholder="Buscar personagens, regras, falas..."
                autocomplete="off"
              />
              @if (query()) {
                <button
                  matSuffix
                  mat-icon-button
                  aria-label="Limpar"
                  (click)="clearQuery()"
                  class="qs-clear-btn"
                >
                  <mat-icon>close</mat-icon>
                </button>
              }
            </mat-form-field>
          </div>

          <!-- Shortcut hint -->
          <div class="qs-hint">
            <kbd>Ctrl+F</kbd> &middot; <kbd>Esc</kbd> para fechar
          </div>

          <!-- Results -->
          <div class="qs-results">
            @if (loading()) {
              <div class="qs-state">
                <mat-icon class="qs-state-icon">hourglass_top</mat-icon>
                <span>Buscando...</span>
              </div>
            } @else if (query() && totalResults() === 0) {
              <div class="qs-state">
                <mat-icon class="qs-state-icon">search_off</mat-icon>
                <span>Nenhum resultado para <strong>&ldquo;{{ query() }}&rdquo;</strong></span>
              </div>
            } @else if (!query()) {
              <div class="qs-state">
                <mat-icon class="qs-state-icon">search</mat-icon>
                <span>Digite para pesquisar</span>
              </div>
            } @else {
              @for (group of groups(); track group.category) {
                @if (group.entries.length > 0) {
                  <!-- Section header -->
                  <div class="qs-section-header">
                    <mat-icon class="qs-section-icon">{{ group.icon }}</mat-icon>
                    <span class="qs-section-label">{{ group.label }}</span>
                    <span class="qs-section-count">{{ group.entries.length }}</span>
                  </div>

                  <!-- Results list -->
                  <mat-nav-list class="qs-entry-list" dense>
                    @for (entry of group.entries; track entryId(entry)) {
                      <button
                        mat-list-item
                        class="qs-entry-row"
                        (click)="selectEntry(entry)"
                      >
                        <mat-icon matListItemIcon class="qs-entry-icon">
                          {{ group.icon }}
                        </mat-icon>
                        <div matListItemTitle class="qs-entry-title">
                          {{ entryTitle(entry) }}
                        </div>
                        <div matListItemLine class="qs-entry-line">
                          {{ entrySubtitle(entry) }}
                        </div>
                        <span matListItemMeta>
                          <mat-chip-row
                            class="qs-cat-chip"
                            [class.chip-personagens]="group.category === 'personagens'"
                            [class.chip-regras]="group.category === 'regras'"
                            [class.chip-falas]="group.category === 'falas'"
                            disableRipple
                          >
                            {{ group.label }}
                          </mat-chip-row>
                        </span>
                      </button>
                    }
                  </mat-nav-list>
                }
              }
            }
          </div>
        </div>
      } @else {
        <!-- ══════════ DETAIL VIEW ══════════ -->
        <div class="qs-body qs-detail-body">
          <button mat-button class="qs-back-btn" (click)="backToResults()">
            <mat-icon>arrow_back</mat-icon>
            Voltar para resultados
          </button>

          @let entry = selectedEntry()!;

          @switch (entry.kind) {
            @case ('character') {
              @let char = entry.character;
              <div class="qs-detail-card">
                <div class="qs-detail-card-header">
                  <div class="qs-detail-avatar">
                    <mat-icon class="qs-detail-avatar-icon">person</mat-icon>
                  </div>
                  <div class="qs-detail-title-group">
                    <h2 class="qs-detail-name">{{ char.name }}</h2>
                    <span
                      class="qs-type-badge"
                      [class.badge-npc]="char.type === 'npc'"
                      [class.badge-player]="char.type === 'player'"
                      [class.badge-boss]="char.type === 'boss'"
                    >
                      {{ char.type | uppercase }}
                    </span>
                  </div>
                </div>

                @if (char.description) {
                  <p class="qs-detail-description">{{ char.description }}</p>
                }

                @if (char.history) {
                  <div class="qs-detail-section">
                    <h3 class="qs-detail-section-title">História</h3>
                    <p class="qs-detail-text">{{ char.history }}</p>
                  </div>
                }

                @if (char.masterNotes) {
                  <div class="qs-detail-section qs-detail-notes">
                    <h3 class="qs-detail-section-title">Notas do Mestre</h3>
                    <p class="qs-detail-text">{{ char.masterNotes }}</p>
                  </div>
                }

                @if (hasKeys(char.attributes)) {
                  <div class="qs-detail-section">
                    <h3 class="qs-detail-section-title">Atributos</h3>
                    <div class="qs-attr-grid">
                      @for (entry of keyValues(char.attributes); track entry.key) {
                        <div class="qs-attr-item">
                          <span class="qs-attr-label">{{ entry.key }}</span>
                          <span class="qs-attr-value">{{ entry.value }}</span>
                        </div>
                      }
                    </div>
                  </div>
                }

                @if (char.skills.length > 0) {
                  <div class="qs-detail-section">
                    <h3 class="qs-detail-section-title">Habilidades</h3>
                    <div class="qs-chip-list">
                      @for (skill of char.skills; track skill) {
                        <span class="qs-skill-chip">{{ skill }}</span>
                      }
                    </div>
                  </div>
                }

                @if (char.inventory.length > 0) {
                  <div class="qs-detail-section">
                    <h3 class="qs-detail-section-title">Inventário</h3>
                    <ul class="qs-inv-list">
                      @for (item of char.inventory; track item) {
                        <li>{{ item }}</li>
                      }
                    </ul>
                  </div>
                }

                @if (char.quotes.length > 0) {
                  <div class="qs-detail-section">
                    <h3 class="qs-detail-section-title">Falas</h3>
                    <div class="qs-quotes-list">
                      @for (quote of char.quotes; track quote; let i = $index) {
                        <div
                          class="qs-quote-item"
                          [class.qs-quote-highlight]="entry.quoteIndex === i"
                        >
                          <mat-icon class="qs-quote-icon">format_quote</mat-icon>
                          <span>{{ quote }}</span>
                        </div>
                      }
                    </div>
                  </div>
                }
              </div>
            }

            @case ('rule') {
              @let rule = entry.rule;
              <div class="qs-detail-card">
                <div class="qs-detail-card-header">
                  <div class="qs-detail-avatar qs-detail-avatar-rule">
                    <mat-icon class="qs-detail-avatar-icon">menu_book</mat-icon>
                  </div>
                  <div class="qs-detail-title-group">
                    <h2 class="qs-detail-name">{{ rule.name }}</h2>
                  </div>
                </div>

                @if (rule.description) {
                  <p class="qs-detail-description">{{ rule.description }}</p>
                }

                <div class="qs-detail-section">
                  <h3 class="qs-detail-section-title">Arquivo</h3>
                  <p class="qs-detail-text">
                    <mat-icon class="qs-detail-inline-icon">description</mat-icon>
                    {{ rule.pdfUrl || 'Nenhum arquivo vinculado' }}
                  </p>
                </div>
              </div>
            }
          }
        </div>
      }
    </div>
  `,
  styles: [
    `
      /* ─── Container ─────────────────────────────────── */
      .qs-container {
        width: min(560px, 95vw);
        max-height: min(620px, 90vh);
        display: flex;
        flex-direction: column;
        overflow: hidden;
        background: #1a1a2e;
        border-radius: 12px;
      }

      /* ─── Header ────────────────────────────────────── */
      .qs-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 12px 12px 0 20px;
        flex-shrink: 0;
      }

      .qs-title {
        font-size: 1rem;
        font-weight: 600;
        letter-spacing: 0.3px;
        color: #e0e0e0;
      }

      .qs-close-btn {
        width: 32px;
        height: 32px;
        line-height: 32px;
      }
      .qs-close-btn .mat-icon {
        font-size: 18px;
        width: 18px;
        height: 18px;
        line-height: 18px;
      }

      /* ─── Body ──────────────────────────────────────── */
      .qs-body {
        flex: 1;
        display: flex;
        flex-direction: column;
        overflow: hidden;
        padding: 8px 12px 12px;
      }

      .qs-detail-body {
        overflow-y: auto;
      }

      /* ─── Search field ──────────────────────────────── */
      .qs-search-field-wrapper {
        padding: 0 4px;
        flex-shrink: 0;
      }

      .qs-search-field {
        width: 100%;
      }

      .qs-clear-btn .mat-icon {
        font-size: 18px;
        width: 18px;
        height: 18px;
        line-height: 18px;
      }

      /* ─── Hint ──────────────────────────────────────── */
      .qs-hint {
        padding: 4px 8px 8px;
        font-size: 0.72rem;
        color: rgba(255, 255, 255, 0.35);
        flex-shrink: 0;
      }
      .qs-hint kbd {
        display: inline-block;
        padding: 1px 5px;
        font-size: 0.68rem;
        font-family: inherit;
        background: rgba(255, 255, 255, 0.08);
        border: 1px solid rgba(255, 255, 255, 0.12);
        border-radius: 4px;
        color: rgba(255, 255, 255, 0.5);
      }

      /* ─── Empty / loading states ────────────────────── */
      .qs-state {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        padding: 40px 16px;
        opacity: 0.5;
        text-align: center;
        gap: 8px;
      }
      .qs-state-icon {
        font-size: 2.2rem;
        width: 2.2rem;
        height: 2.2rem;
      }
      .qs-state span {
        font-size: 0.9rem;
      }
      .qs-state strong {
        color: rgba(255, 255, 255, 0.7);
      }

      /* ─── Results area ──────────────────────────────── */
      .qs-results {
        flex: 1;
        overflow-y: auto;
        margin: 0 -4px;
        padding: 0 4px;
      }

      /* Section header */
      .qs-section-header {
        display: flex;
        align-items: center;
        gap: 8px;
        padding: 12px 8px 6px;
        font-size: 0.8rem;
        text-transform: uppercase;
        letter-spacing: 1px;
        color: rgba(255, 255, 255, 0.5);
        border-bottom: 1px solid rgba(255, 255, 255, 0.06);
        margin-bottom: 4px;
      }
      .qs-section-header:first-child {
        padding-top: 4px;
      }

      .qs-section-icon {
        font-size: 16px;
        width: 16px;
        height: 16px;
        line-height: 16px;
        opacity: 0.6;
      }

      .qs-section-label {
        flex: 1;
        font-weight: 600;
      }

      .qs-section-count {
        font-size: 0.7rem;
        background: rgba(255, 255, 255, 0.08);
        padding: 0 8px;
        border-radius: 10px;
        line-height: 20px;
      }

      /* Entry list */
      .qs-entry-list {
        padding: 0;
      }

      .qs-entry-row {
        cursor: pointer;
        border-radius: 6px;
        margin: 1px 0;
        --mat-list-active-indicator-shape: 6px;
      }

      .qs-entry-icon {
        opacity: 0.6;
        font-size: 20px;
      }

      .qs-entry-title {
        font-size: 0.9rem;
        font-weight: 500;
        color: #e8e8e8;
      }

      .qs-entry-line {
        font-size: 0.78rem !important;
        opacity: 0.55;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
        padding-right: 8px;
      }

      /* Category chips */
      .qs-cat-chip {
        --mdc-chip-elevated-container-color: transparent;
        --mdc-chip-label-text-size: 0.65rem;
        --mdc-chip-label-text-color: rgba(255, 255, 255, 0.7);
        font-size: 0.65rem;
        padding: 0 6px;
        min-height: 22px;
        border: 1px solid rgba(255, 255, 255, 0.15);
      }
      .qs-cat-chip.chip-personagens {
        border-color: #7c4dff;
        background: rgba(124, 77, 255, 0.12);
      }
      .qs-cat-chip.chip-regras {
        border-color: #448aff;
        background: rgba(68, 138, 255, 0.12);
      }
      .qs-cat-chip.chip-falas {
        border-color: #ff6d00;
        background: rgba(255, 109, 0, 0.12);
      }

      /* ─── Back button ──────────────────────────────── */
      .qs-back-btn {
        margin-bottom: 16px;
        align-self: flex-start;
        font-size: 0.85rem;
      }
      .qs-back-btn .mat-icon {
        font-size: 18px;
        width: 18px;
        height: 18px;
        line-height: 18px;
      }

      /* ─── Detail card ──────────────────────────────── */
      .qs-detail-card {
        padding: 4px 0;
      }

      .qs-detail-card-header {
        display: flex;
        align-items: center;
        gap: 16px;
        margin-bottom: 16px;
      }

      .qs-detail-avatar {
        width: 52px;
        height: 52px;
        border-radius: 50%;
        background: linear-gradient(135deg, #7c4dff, #b388ff);
        display: flex;
        align-items: center;
        justify-content: center;
        flex-shrink: 0;
      }

      .qs-detail-avatar-rule {
        background: linear-gradient(135deg, #448aff, #82b1ff);
      }

      .qs-detail-avatar-icon {
        font-size: 26px;
        width: 26px;
        height: 26px;
        line-height: 26px;
        color: #fff;
      }

      .qs-detail-title-group {
        display: flex;
        flex-direction: column;
        gap: 4px;
        min-width: 0;
      }

      .qs-detail-name {
        margin: 0;
        font-size: 1.25rem;
        font-weight: 600;
        color: #f0f0f0;
        line-height: 1.3;
      }

      .qs-type-badge {
        display: inline-block;
        font-size: 0.62rem;
        font-weight: 700;
        letter-spacing: 1.2px;
        padding: 2px 10px;
        border-radius: 4px;
        text-transform: uppercase;
        width: fit-content;
      }

      .badge-npc {
        background: rgba(124, 77, 255, 0.2);
        color: #b388ff;
      }
      .badge-player {
        background: rgba(0, 200, 83, 0.2);
        color: #69f0ae;
      }
      .badge-boss {
        background: rgba(255, 23, 68, 0.2);
        color: #ff5252;
      }

      .qs-detail-description {
        font-size: 0.9rem;
        line-height: 1.6;
        color: rgba(255, 255, 255, 0.7);
        margin: 0 0 16px;
        padding: 12px;
        background: rgba(255, 255, 255, 0.04);
        border-radius: 8px;
        border-left: 3px solid rgba(124, 77, 255, 0.4);
      }

      /* Sections */
      .qs-detail-section {
        margin-bottom: 16px;
      }

      .qs-detail-section-title {
        font-size: 0.78rem;
        font-weight: 600;
        text-transform: uppercase;
        letter-spacing: 0.8px;
        color: rgba(255, 255, 255, 0.4);
        margin: 0 0 8px;
      }

      .qs-detail-text {
        font-size: 0.88rem;
        line-height: 1.6;
        color: rgba(255, 255, 255, 0.7);
        margin: 0;
        white-space: pre-wrap;
      }

      .qs-detail-notes {
        padding: 12px;
        background: rgba(255, 215, 0, 0.05);
        border-radius: 8px;
        border: 1px solid rgba(255, 215, 0, 0.1);
      }

      .qs-detail-inline-icon {
        font-size: 16px;
        width: 16px;
        height: 16px;
        line-height: 16px;
        vertical-align: middle;
        margin-right: 4px;
        opacity: 0.6;
      }

      /* Attributes grid */
      .qs-attr-grid {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(100px, 1fr));
        gap: 6px;
      }

      .qs-attr-item {
        display: flex;
        flex-direction: column;
        padding: 8px 10px;
        background: rgba(255, 255, 255, 0.04);
        border-radius: 6px;
        border: 1px solid rgba(255, 255, 255, 0.06);
      }

      .qs-attr-label {
        font-size: 0.65rem;
        text-transform: uppercase;
        letter-spacing: 0.6px;
        color: rgba(255, 255, 255, 0.35);
        margin-bottom: 2px;
      }

      .qs-attr-value {
        font-size: 1rem;
        font-weight: 600;
        color: #e0e0e0;
      }

      /* Skill chips */
      .qs-chip-list {
        display: flex;
        flex-wrap: wrap;
        gap: 6px;
      }

      .qs-skill-chip {
        display: inline-block;
        padding: 4px 12px;
        font-size: 0.78rem;
        background: rgba(124, 77, 255, 0.15);
        color: #c9b1ff;
        border-radius: 16px;
        border: 1px solid rgba(124, 77, 255, 0.2);
      }

      /* Inventory */
      .qs-inv-list {
        margin: 0;
        padding: 0 0 0 20px;
        font-size: 0.88rem;
        color: rgba(255, 255, 255, 0.7);
        line-height: 1.7;
      }

      /* Quotes in detail */
      .qs-quotes-list {
        display: flex;
        flex-direction: column;
        gap: 8px;
      }

      .qs-quote-item {
        display: flex;
        align-items: flex-start;
        gap: 8px;
        padding: 10px 12px;
        background: rgba(255, 255, 255, 0.03);
        border-radius: 8px;
        border: 1px solid rgba(255, 255, 255, 0.06);
        font-size: 0.88rem;
        font-style: italic;
        color: rgba(255, 255, 255, 0.75);
        line-height: 1.5;
        transition: background 0.2s;
      }

      .qs-quote-highlight {
        background: rgba(255, 109, 0, 0.12);
        border-color: rgba(255, 109, 0, 0.3);
      }

      .qs-quote-icon {
        font-size: 18px;
        width: 18px;
        height: 18px;
        line-height: 18px;
        flex-shrink: 0;
        opacity: 0.4;
        margin-top: 2px;
      }
    `,
  ],
})
export class SessionQuickSearchComponent implements AfterViewInit, OnDestroy {
  // ── DI ──────────────────────────────────────────────────────
  private readonly dialogRef = inject(MatDialogRef<SessionQuickSearchComponent>);
  private readonly store = inject(StoreService);

  @ViewChild('searchInput') searchInput!: ElementRef<HTMLInputElement>;

  // ── State ───────────────────────────────────────────────────
  readonly query = signal('');
  readonly loading = signal(false);
  readonly groups = signal<SearchGroup[]>([]);
  readonly selectedEntry = signal<SearchEntry | null>(null);

  readonly totalResults = computed(() =>
    this.groups().reduce((sum, g) => sum + g.entries.length, 0),
  );

  private readonly searchSubject = new Subject<string>();
  private readonly subscriptions = new Subscription();

  // ── Lifecycle ───────────────────────────────────────────────
  ngAfterViewInit(): void {
    setTimeout(() => this.searchInput?.nativeElement?.focus(), 80);

    const sub = this.searchSubject
      .pipe(debounceTime(300), distinctUntilChanged())
      .subscribe((q) => this.performSearch(q));

    this.subscriptions.add(sub);
  }

  ngOnDestroy(): void {
    this.subscriptions.unsubscribe();
  }

  // ── Search ──────────────────────────────────────────────────
  private performSearch(q: string): void {
    const trimmed = q.trim().toLowerCase();

    if (!trimmed) {
      this.groups.set([]);
      this.loading.set(false);
      return;
    }

    this.loading.set(true);

    // Search in parallel via synchronous snapshot
    const characters = this.store.snapshot('characters') as Character[];
    const rules = this.store.snapshot('rules') as RuleBook[];

    const personagens: CharacterResult[] = [];
    const falas: CharacterResult[] = [];
    const regras: RuleResult[] = [];

    // Character name match
    for (const char of characters) {
      if (char.name.toLowerCase().includes(trimmed)) {
        personagens.push({ kind: 'character', character: char, matchType: 'name' });
      }
    }

    // Character quote match
    for (const char of characters) {
      for (let i = 0; i < char.quotes.length; i++) {
        if (char.quotes[i].toLowerCase().includes(trimmed)) {
          falas.push({
            kind: 'character',
            character: char,
            matchType: 'quote',
            quoteText: char.quotes[i],
            quoteIndex: i,
          });
        }
      }
    }

    // Rule name match
    for (const rule of rules) {
      if (rule.name.toLowerCase().includes(trimmed)) {
        regras.push({ kind: 'rule', rule });
      }
    }

    const result: SearchGroup[] = [];

    if (personagens.length > 0) {
      result.push({ category: 'personagens', icon: 'person', label: 'Personagens', entries: personagens });
    }
    if (regras.length > 0) {
      result.push({ category: 'regras', icon: 'menu_book', label: 'Regras', entries: regras });
    }
    if (falas.length > 0) {
      result.push({ category: 'falas', icon: 'chat', label: 'Falas', entries: falas });
    }

    this.groups.set(result);
    this.loading.set(false);
  }

  // ── Event handlers ─────────────────────────────────────────
  onQueryChange(value: string): void {
    this.query.set(value);
    this.searchSubject.next(value);
  }

  clearQuery(): void {
    this.query.set('');
    this.groups.set([]);
    this.searchSubject.next('');
    this.searchInput?.nativeElement?.focus();
  }

  selectEntry(entry: SearchEntry): void {
    this.selectedEntry.set(entry);
  }

  backToResults(): void {
    this.selectedEntry.set(null);
  }

  close(): void {
    this.dialogRef.close();
  }

  // ── Host listener ──────────────────────────────────────────
  @HostListener('keydown', ['$event'])
  onKeydown(event: KeyboardEvent): void {
    if (event.key === 'Escape') {
      if (this.selectedEntry()) {
        this.backToResults();
        event.stopPropagation();
      } else {
        this.close();
      }
    }
  }

  // ── Template helpers ───────────────────────────────────────
  entryId(entry: SearchEntry): string {
    switch (entry.kind) {
      case 'character':
        return `char-${entry.character.id}-${entry.matchType}${entry.quoteIndex ?? ''}`;
      case 'rule':
        return `rule-${entry.rule.id}`;
    }
  }

  entryTitle(entry: SearchEntry): string {
    switch (entry.kind) {
      case 'character':
        return entry.character.name;
      case 'rule':
        return entry.rule.name;
    }
  }

  entrySubtitle(entry: SearchEntry): string {
    switch (entry.kind) {
      case 'character':
        if (entry.matchType === 'quote' && entry.quoteText) {
          return entry.quoteText.length > 80
            ? entry.quoteText.slice(0, 80) + '…'
            : entry.quoteText;
        }
        return entry.character.description || entry.character.type;
      case 'rule':
        return entry.rule.description || 'Regra';
    }
  }

  hasKeys(obj: Record<string, number>): boolean {
    return obj != null && Object.keys(obj).length > 0;
  }

  keyValues(obj: Record<string, number>): { key: string; value: number }[] {
    return Object.entries(obj).map(([key, value]) => ({ key, value }));
  }
}
