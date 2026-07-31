import { Component, OnInit, OnDestroy, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { Router, RouterModule } from '@angular/router';
import { Subject } from 'rxjs';
import { takeUntil } from 'rxjs/operators';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatListModule } from '@angular/material/list';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatDialog } from '@angular/material/dialog';
import { PageHeaderComponent } from '../../shared/components/page-header.component';
import { EmptyStateComponent } from '../../shared/components/empty-state.component';
import { StoreService } from '../../core/store/store.service';
import { SessionToolbarComponent } from './session-toolbar.component';
import { DiceRollerDialogComponent } from './dice-roller-dialog.component';
import type { SessionState, QuickReference } from '../../core/models/session';
import type { BreadcrumbItem } from '../../shared/components/page-header.component';

const SESSION_ID = 'current-session';

@Component({
  selector: 'app-session-cockpit',
  standalone: true,
  imports: [
    CommonModule,
    FormsModule,
    RouterModule,
    MatButtonModule,
    MatIconModule,
    MatListModule,
    MatFormFieldModule,
    MatInputModule,
    PageHeaderComponent,
    EmptyStateComponent,
    SessionToolbarComponent,
  ],
  template: `
    <div class="cockpit">
      <!-- Page Header -->
      <app-page-header
        [breadcrumbs]="breadcrumbs"
        title="Painel da Sessão"
      />

      <!-- Session Toolbar -->
      <app-session-toolbar
        (rollDice)="openDiceRoller()"
        (openMap)="openMap()"
        (toggleMusic)="toggleMusic()"
        (togglePlayPause)="togglePlayPause()"
      />

      <!-- Split Panels -->
      <div class="split-container" #splitContainer>
        <!-- Left Panel: Active Entity -->
        <div
          class="panel panel-left"
          [style.width.%]="leftPanelWidth"
        >
          <div class="panel-header">
            <mat-icon class="panel-icon">person</mat-icon>
            <span>Entidade Ativa</span>
          </div>

          <div class="panel-body">
            @if (activeEntity) {
              <div class="entity-card">
                <div class="entity-type-badge" [class]="'badge-' + activeEntity.type">
                  {{ typeLabel(activeEntity.type) }}
                </div>
                <p class="entity-id">ID: {{ activeEntity.id }}</p>
                <p class="entity-placeholder">
                  Conteúdo detalhado da entidade será exibido aqui.
                </p>
              </div>
            } @else {
              <app-empty-state
                icon="touch_app"
                message="Selecione uma entidade na campanha para visualizar aqui."
              />
            }
          </div>
        </div>

        <!-- Draggable Divider -->
        <div
          class="split-divider"
          (mousedown)="onDividerMouseDown($event)"
          [class.dragging]="isDragging"
        >
          <div class="divider-grip">
            <mat-icon class="grip-icon">drag_indicator</mat-icon>
          </div>
        </div>

        <!-- Right Panel: Quick References -->
        <div
          class="panel panel-right"
          [style.width.%]="rightPanelWidth"
        >
          <div class="panel-header">
            <mat-icon class="panel-icon">book</mat-icon>
            <span>Referências Rápidas</span>
          </div>

          <div class="panel-body">
            <!-- Search -->
            <mat-form-field appearance="outline" class="ref-search" subscriptSizing="dynamic">
              <mat-icon matPrefix>search</mat-icon>
              <input
                matInput
                [(ngModel)]="searchQuery"
                placeholder="Filtrar referências..."
                (ngModelChange)="onSearchChange()"
              />
              @if (searchQuery) {
                <button matSuffix mat-icon-button aria-label="Limpar" (click)="clearSearch()">
                  <mat-icon>close</mat-icon>
                </button>
              }
            </mat-form-field>

            <!-- Reference List -->
            @if (filteredReferences.length > 0) {
              <mat-list class="ref-list">
                @for (ref of filteredReferences; track ref.id) {
                  <mat-list-item class="ref-item" (click)="focusReference(ref)">
                    <mat-icon matListItemIcon>article</mat-icon>
                    <span matListItemTitle>{{ ref.title }}</span>
                    <span matListItemLine class="ref-excerpt">{{ ref.content | slice:0:80 }}{{ ref.content.length > 80 ? '...' : '' }}</span>
                  </mat-list-item>
                }
              </mat-list>
            } @else {
              <app-empty-state
                icon="menu_book"
                [message]="searchQuery ? 'Nenhuma referência encontrada.' : 'Nenhuma referência ainda. Adicione na campanha.'"
              />
            }
          </div>
        </div>
      </div>
    </div>
  `,
  styles: `
    :host {
      display: block;
      height: 100%;
    }

    .cockpit {
      display: flex;
      flex-direction: column;
      height: 100%;
    }

    /* ── Split Container ─────────────────────── */

    .split-container {
      display: flex;
      flex: 1;
      min-height: 0;
      overflow: hidden;
      position: relative;
      border-top: 1px solid rgba(255, 255, 255, 0.06);
    }

    /* ── Panels ──────────────────────────────── */

    .panel {
      display: flex;
      flex-direction: column;
      overflow: hidden;
      min-width: 0;
    }

    .panel-header {
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 12px 16px;
      font-size: 0.85rem;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.5px;
      color: rgba(255, 255, 255, 0.6);
      border-bottom: 1px solid rgba(255, 255, 255, 0.06);
      background: rgba(255, 255, 255, 0.02);
      flex-shrink: 0;
    }

    .panel-icon {
      font-size: 1.1rem;
      width: 1.1rem;
      height: 1.1rem;
      opacity: 0.8;
    }

    .panel-body {
      flex: 1;
      overflow-y: auto;
      padding: 16px;
    }

    .panel-left {
      background: rgba(255, 255, 255, 0.01);
    }

    .panel-right {
      background: rgba(255, 255, 255, 0.02);
    }

    /* ── Draggable Divider ───────────────────── */

    .split-divider {
      width: 8px;
      cursor: col-resize;
      display: flex;
      align-items: center;
      justify-content: center;
      background: rgba(255, 255, 255, 0.03);
      transition: background 0.15s ease;
      flex-shrink: 0;
      position: relative;
      z-index: 2;
    }

    .split-divider:hover,
    .split-divider.dragging {
      background: rgba(255, 255, 255, 0.08);
    }

    .divider-grip {
      width: 2px;
      height: 40px;
      border-radius: 2px;
      background: rgba(255, 255, 255, 0.15);
      display: flex;
      align-items: center;
      justify-content: center;
      transition: background 0.15s ease;
    }

    .split-divider:hover .divider-grip,
    .split-divider.dragging .divider-grip {
      background: rgba(255, 255, 255, 0.35);
    }

    .grip-icon {
      font-size: 0.5rem;
      width: 0.5rem;
      height: 0.5rem;
      opacity: 0;
      transition: opacity 0.15s ease;
    }

    .split-divider:hover .grip-icon,
    .split-divider.dragging .grip-icon {
      opacity: 1;
    }

    /* ── Entity Card ─────────────────────────── */

    .entity-card {
      background: rgba(255, 255, 255, 0.03);
      border: 1px solid rgba(255, 255, 255, 0.06);
      border-radius: 12px;
      padding: 20px;
    }

    .entity-type-badge {
      display: inline-block;
      padding: 2px 10px;
      border-radius: 8px;
      font-size: 0.7rem;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.5px;
      color: #fff;
      margin-bottom: 12px;
    }

    .badge-character {
      background: linear-gradient(135deg, #065f46, #059669);
    }
    .badge-map {
      background: linear-gradient(135deg, #155e75, #0891b2);
    }
    .badge-rules {
      background: linear-gradient(135deg, #5b21b6, #8b5cf6);
    }
    .badge-note {
      background: linear-gradient(135deg, #7f1d1d, #dc2626);
    }

    .entity-id {
      margin: 0 0 8px;
      font-size: 0.8rem;
      opacity: 0.5;
      font-family: monospace;
    }

    .entity-placeholder {
      margin: 0;
      font-size: 0.9rem;
      opacity: 0.6;
      line-height: 1.5;
    }

    /* ── Empty State handled by EmptyStateComponent ── */

    /* ── Quick References ────────────────────── */

    .ref-search {
      width: 100%;
      margin-bottom: 8px;
    }

    .ref-list {
      padding-top: 0;
    }

    .ref-item {
      cursor: pointer;
      border-radius: 8px;
      transition: background 0.15s ease;
    }

    .ref-item:hover {
      background: rgba(255, 255, 255, 0.04);
    }

    .ref-excerpt {
      font-size: 0.8rem;
      opacity: 0.55;
    }

    /* ── Mobile: ≤768px ─────────────────────── */

    @media (max-width: 768px) {
      .split-container {
        flex-direction: column;
      }
      .panel {
        width: 100% !important;
        overflow: auto;
        flex: none;
        min-height: 200px;
      }
    }
  `,
})
export class SessionCockpitComponent implements OnInit, OnDestroy {
  private readonly store = inject(StoreService<SessionState>);
  private readonly dialog = inject(MatDialog);
  private readonly router = inject(Router);
  private readonly destroy$ = new Subject<void>();

  readonly breadcrumbs: BreadcrumbItem[] = [
    { label: 'Sessão', route: '/sessao' },
  ];

  /** Active entity from session state. */
  activeEntity: SessionState['activeEntity'] | null = null;

  /** Quick references from session state. */
  quickReferences: QuickReference[] = [];

  /** Search query for filtering references. */
  searchQuery = '';

  /** Filtered references for display. */
  filteredReferences: QuickReference[] = [];

  /** Left panel width in percentage. */
  leftPanelWidth = 60;

  /** Right panel width in percentage. */
  get rightPanelWidth(): number {
    return 100 - this.leftPanelWidth;
  }

  /** Whether the divider is being dragged. */
  isDragging = false;

  /** Music toggle placeholder state. */
  musicPlaying = false;

  /** Play/pause placeholder state. */
  isPaused = false;

  private splitStartX = 0;
  private splitStartLeft = 60;

  ngOnInit(): void {
    // Load or create session state
    this.store.get('sessions', SESSION_ID)
      .pipe(takeUntil(this.destroy$))
      .subscribe((session) => {
        if (session) {
          this.activeEntity = session.activeEntity ?? null;
          this.quickReferences = session.quickReferences;
          this.applyFilter();
        } else {
          // Create a new session state
          const newSession: SessionState = {
            id: SESSION_ID,
            quickReferences: [
              { id: 'ref-1', title: 'Regras Básicas', content: 'Sistema d20: role 1d20 e adicione modificadores. Resultados acima da CD são sucessos.' },
              { id: 'ref-2', title: 'Condições', content: 'Atordoado: não pode agir. Caído: desvantagem em ataques corpo a corpo. Invisível: vantagem em ataques.' },
              { id: 'ref-3', title: 'Economia de Ações', content: 'Ação Padrão, Ação de Movimento, Ação Bonus. Uma ação livre por turno.' },
            ],
            toolbarShortcuts: [
              { id: 'shortcut-1', icon: 'casino', label: 'Rolar Dados', action: 'rollDice' },
              { id: 'shortcut-2', icon: 'map', label: 'Abrir Mapa', action: 'openMap' },
              { id: 'shortcut-3', icon: 'music_note', label: 'Tocar Música', action: 'toggleMusic' },
              { id: 'shortcut-4', icon: 'play_arrow', label: 'Play/Pause', action: 'togglePlayPause' },
            ],
            notes: '',
            createdAt: new Date(),
            updatedAt: new Date(),
          };
          this.store.set('sessions', newSession);
          this.activeEntity = null;
          this.quickReferences = newSession.quickReferences;
          this.applyFilter();
        }
      });
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

  /* ── Divider Drag Logic ─────────────────── */

  onDividerMouseDown(event: MouseEvent): void {
    event.preventDefault();
    this.isDragging = true;
    this.splitStartX = event.clientX;
    this.splitStartLeft = this.leftPanelWidth;

    const onMouseMove = (e: MouseEvent) => {
      const container = (e.target as HTMLElement).closest('.split-container') as HTMLElement;
      if (!container) return;
      const rect = container.getBoundingClientRect();
      const deltaX = e.clientX - this.splitStartX;
      const deltaPercent = (deltaX / rect.width) * 100;
      let newLeft = this.splitStartLeft + deltaPercent;
      newLeft = Math.max(25, Math.min(75, newLeft));
      this.leftPanelWidth = newLeft;
    };

    const onMouseUp = () => {
      this.isDragging = false;
      document.removeEventListener('mousemove', onMouseMove);
      document.removeEventListener('mouseup', onMouseUp);
    };

    document.addEventListener('mousemove', onMouseMove);
    document.addEventListener('mouseup', onMouseUp);
  }

  /* ── Toolbar Actions ────────────────────── */

  openDiceRoller(): void {
    const dialogRef = this.dialog.open(DiceRollerDialogComponent, {
      width: '400px',
      maxWidth: '90vw',
      panelClass: 'dice-roller-dialog',
    });
    dialogRef.afterClosed().pipe(takeUntil(this.destroy$)).subscribe();
  }

  openMap(): void {
    this.router.navigate(['/mapa']);
  }

  toggleMusic(): void {
    this.musicPlaying = !this.musicPlaying;
  }

  togglePlayPause(): void {
    this.isPaused = !this.isPaused;
  }

  /* ── Quick References ───────────────────── */

  onSearchChange(): void {
    this.applyFilter();
  }

  clearSearch(): void {
    this.searchQuery = '';
    this.applyFilter();
  }

  private applyFilter(): void {
    if (!this.searchQuery.trim()) {
      this.filteredReferences = [...this.quickReferences];
      return;
    }
    const q = this.searchQuery.toLowerCase();
    this.filteredReferences = this.quickReferences.filter(
      (ref) =>
        ref.title.toLowerCase().includes(q) ||
        ref.content.toLowerCase().includes(q),
    );
  }

  focusReference(ref: QuickReference): void {
    // For MVP: could be expanded or navigated; for now just a visual focus placeholder
  }

  /* ── Helpers ────────────────────────────── */

  typeLabel(type: string): string {
    switch (type) {
      case 'character': return 'Personagem';
      case 'map': return 'Mapa';
      case 'rules': return 'Regras';
      case 'note': return 'Nota';
      default: return type;
    }
  }
}
