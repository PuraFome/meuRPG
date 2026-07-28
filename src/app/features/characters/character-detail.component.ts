import { Component, Input, OnDestroy, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { Router } from '@angular/router';
import { Subject } from 'rxjs';
import { debounceTime, distinctUntilChanged, takeUntil } from 'rxjs/operators';
import { MatTabsModule } from '@angular/material/tabs';
import { MatIconModule } from '@angular/material/icon';
import { MatButtonModule } from '@angular/material/button';
import { QuillModule } from 'ngx-quill';
import { StoreService } from '../../core/store/store.service';
import { PageHeaderComponent } from '../../shared/components/page-header.component';
import { CharacterSheetComponent } from './character-sheet.component';
import type { Character } from '../../core/models/character';
import type { BreadcrumbItem } from '../../shared/components/page-header.component';

@Component({
  selector: 'app-character-detail',
  standalone: true,
  imports: [
    CommonModule,
    FormsModule,
    MatTabsModule,
    MatIconModule,
    MatButtonModule,
    QuillModule,
    PageHeaderComponent,
    CharacterSheetComponent,
  ],
  template: `
    <!-- Breadcrumbs + Back -->
    <app-page-header
      [breadcrumbs]="breadcrumbs"
      [title]="character?.name ?? 'Personagens'"
    >
      <button mat-icon-button actions (click)="goBack()" aria-label="Voltar">
        <mat-icon>arrow_back</mat-icon>
      </button>
    </app-page-header>

    @if (character) {
      <div class="detail-page">
        <!-- Header: avatar + name + type -->
        <div class="detail-header">
          <div class="avatar-placeholder" [class]="'type-' + character.type">
            {{ character.name.charAt(0).toUpperCase() }}
          </div>
          <div class="header-info">
            <h1 class="character-name">{{ character.name }}</h1>
            <span class="type-badge" [class]="'type-badge type-' + character.type">
              {{ typeLabel(character.type) }}
            </span>
          </div>
        </div>

        <!-- 4 Tabs -->
        <mat-tab-group>
          <mat-tab label="História">
            <div class="tab-content">
              <quill-editor
                [(ngModel)]="character.history"
                (onContentChanged)="onHistoryChange($event.html ?? '')"
                [modules]="quillModules"
                placeholder="Escreva a história do personagem..."
              />
            </div>
          </mat-tab>
          <mat-tab label="Ficha">
            <div class="tab-content">
              <app-character-sheet [character]="character" />
            </div>
          </mat-tab>
          <mat-tab label="Notas do Mestre">
            <div class="tab-content placeholder-content">
              <mat-icon>lock</mat-icon>
              <p>Em construção</p>
            </div>
          </mat-tab>
          <mat-tab label="Falas">
            <div class="tab-content placeholder-content">
              <mat-icon>chat</mat-icon>
              <p>Em construção</p>
            </div>
          </mat-tab>
        </mat-tab-group>
      </div>
    } @else {
      <div class="loading-state">
        <p>Carregando personagem...</p>
      </div>
    }
  `,
  styles: `
    :host {
      display: block;
      max-width: 960px;
      margin: 0 auto;
    }

    .detail-page {
      padding: 8px 0;
    }

    /* ── Header ──────────────────────────────── */

    .detail-header {
      display: flex;
      align-items: center;
      gap: 16px;
      margin-bottom: 24px;
      flex-wrap: wrap;
    }

    .avatar-placeholder {
      width: 64px;
      height: 64px;
      border-radius: 50%;
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 1.75rem;
      font-weight: 700;
      color: #fff;
      text-shadow: 0 2px 6px rgba(0, 0, 0, 0.3);
      flex-shrink: 0;
    }

    .avatar-placeholder.type-player {
      background: linear-gradient(135deg, #065f46, #059669);
    }

    .avatar-placeholder.type-npc {
      background: linear-gradient(135deg, #155e75, #0891b2);
    }

    .avatar-placeholder.type-boss {
      background: linear-gradient(135deg, #7f1d1d, #dc2626);
    }

    .header-info {
      display: flex;
      align-items: center;
      gap: 12px;
      flex-wrap: wrap;
    }

    .character-name {
      margin: 0;
      font-size: 1.75rem;
      font-weight: 700;
      background: linear-gradient(135deg, #e0e0e0 0%, #b388ff 100%);
      -webkit-background-clip: text;
      -webkit-text-fill-color: transparent;
      background-clip: text;
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

    /* ── Tabs ────────────────────────────────── */

    .tab-content {
      padding-top: 16px;
    }

    .placeholder-content {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      min-height: 160px;
      opacity: 0.5;
      gap: 8px;
    }

    .placeholder-content mat-icon {
      font-size: 2rem;
      width: 2rem;
      height: 2rem;
    }

    .placeholder-content p {
      margin: 0;
      font-size: 1rem;
    }

    /* ── Loading ─────────────────────────────── */

    .loading-state {
      display: flex;
      justify-content: center;
      align-items: center;
      min-height: 200px;
      opacity: 0.6;
    }
  `,
})
export class CharacterDetailComponent implements OnInit, OnDestroy {
  @Input() id = '';

  private readonly store = inject(StoreService<Character>);
  private readonly router = inject(Router);

  character: Character | null = null;

  private readonly saveSubject = new Subject<string>();
  private readonly destroy$ = new Subject<void>();

  readonly quillModules = {
    toolbar: [
      ['bold', 'italic'],
      [{ list: 'ordered' }, { list: 'bullet' }],
      [{ header: [1, 2, 3, false] }],
    ],
  };

  get breadcrumbs(): BreadcrumbItem[] {
    return [{ label: 'Personagens', route: '/personagens' }];
  }

  constructor() {
    this.saveSubject.pipe(
      debounceTime(1000),
      distinctUntilChanged(),
      takeUntil(this.destroy$),
    ).subscribe((html) => {
      if (this.character?.id) {
        this.store.patch('characters', this.character.id, {
          history: html,
          updatedAt: new Date(),
        });
      }
    });
  }

  ngOnInit(): void {
    if (this.id) {
      this.store.get('characters', this.id).pipe(
        takeUntil(this.destroy$),
      ).subscribe((c) => {
        this.character = c ?? null;
      });
    }
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

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

  goBack(): void {
    this.router.navigate(['/personagens']);
  }

  onHistoryChange(html: string): void {
    this.saveSubject.next(html);
  }
}
