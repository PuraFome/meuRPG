import {
  Component,
  inject,
  input,
  signal,
  computed,
  effect,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink } from '@angular/router';
import {
  CdkDrag,
  CdkDragHandle,
  CdkDropList,
  CdkDropListGroup,
} from '@angular/cdk/drag-drop';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatCardModule } from '@angular/material/card';
import { MatChipsModule } from '@angular/material/chips';
import { MatDialog } from '@angular/material/dialog';
import { StoreService } from '../../core';
import type { CampaignFolder, Character, MapData, SessionState } from '../../core';
import {
  ConfirmDialogComponent,
  ConfirmDialogData,
} from '../../shared';
import { EntitySelectorDialogComponent, EntitySelectorData } from './entity-selector-dialog.component';

// ─── Entity display model ────────────────────────────────────
export interface EntityRef {
  id: string;
  name: string;
  type: 'character' | 'map' | 'session';
  icon: string;
  route: string;
}

let nextEntitySourceId = 0;
function uniqueEntitySourceId(): string {
  return `entity-source-${nextEntitySourceId++}`;
}

// ─── Component ───────────────────────────────────────────────
@Component({
  selector: 'app-folder-content',
  standalone: true,
  imports: [
    CommonModule,
    RouterLink,
    CdkDrag,
    CdkDragHandle,
    CdkDropList,
    CdkDropListGroup,
    MatButtonModule,
    MatIconModule,
    MatCardModule,
    MatChipsModule,
  ],
  template: `
    @if (folder(); as folder) {
      <div class="folder-content" cdkDropListGroup>
        <!-- Header -->
        <div class="content-header">
          <div class="header-left">
            <mat-icon class="header-folder-icon">folder_open</mat-icon>
            <h2 class="folder-name">{{ folder.name }}</h2>
          </div>
          <div class="header-actions">
            <button
              mat-raised-button
              color="primary"
              (click)="openAssociateDialog()"
              class="assoc-btn"
            >
              <mat-icon>add_link</mat-icon>
              Associar
            </button>
          </div>
        </div>

        <!-- Summary chips -->
        <div class="summary-chips">
          <span class="summary-chip" [class.active]="activeFilter() === 'all'" (click)="setFilter('all')">
            Todos ({{ allEntities().length }})
          </span>
          <span class="summary-chip" [class.active]="activeFilter() === 'character'" (click)="setFilter('character')">
            <mat-icon class="chip-icon">person</mat-icon>
            Personagens ({{ characters().length }})
          </span>
          <span class="summary-chip" [class.active]="activeFilter() === 'map'" (click)="setFilter('map')">
            <mat-icon class="chip-icon">map</mat-icon>
            Mapas ({{ maps().length }})
          </span>
          <span class="summary-chip" [class.active]="activeFilter() === 'session'" (click)="setFilter('session')">
            <mat-icon class="chip-icon">event</mat-icon>
            Sessões ({{ sessions().length }})
          </span>
        </div>

        <!-- Entity list -->
        <div class="entity-list-area">
          @if (filteredEntities().length === 0) {
            <div class="empty-state">
              <mat-icon class="empty-icon">link_off</mat-icon>
              <p class="empty-message">Nenhuma entidade associada</p>
              <p class="empty-hint">
                Clique em "Associar" para vincular personagens, mapas ou sessões a esta pasta.
              </p>
            </div>
          } @else {
            <div
              cdkDropList
              [id]="entitySourceListId"
              [cdkDropListData]="filteredEntities()"
              class="entity-grid"
            >
              @for (entity of filteredEntities(); track entity.type + ':' + entity.id) {
                <mat-card
                  class="entity-card"
                  [class]="'entity-card type-' + entity.type"
                  cdkDrag
                  [cdkDragData]="{ type: entity.type, id: entity.id, name: entity.name }"
                >
                  <div class="drag-indicator" cdkDragHandle>
                    <mat-icon>drag_indicator</mat-icon>
                  </div>

                  <div class="card-icon" [class]="'icon-' + entity.type">
                    <mat-icon>{{ entity.icon }}</mat-icon>
                  </div>

                  <div class="card-body">
                    <a
                      class="entity-name"
                      [routerLink]="entity.route"
                    >
                      {{ entity.name }}
                    </a>
                    <span class="entity-type-label">{{ typeLabel(entity.type) }}</span>
                  </div>

                  <button
                    mat-icon-button
                    class="disassoc-btn"
                    matTooltip="Desassociar"
                    (click)="disassociate(entity)"
                  >
                    <mat-icon>link_off</mat-icon>
                  </button>
                </mat-card>
              }
            </div>
          }
        </div>
      </div>
    } @else {
      <div class="no-folder-selected">
        <mat-icon class="no-folder-icon">folder</mat-icon>
        <p class="no-folder-message">Selecione uma pasta na árvore</p>
        <p class="no-folder-hint">
          Escolha uma pasta à esquerda para ver seu conteúdo ou criar associações.
        </p>
      </div>
    }
  `,
  styles: [
    `
      :host {
        display: block;
        height: 100%;
      }

      /* ── Content layout ────────────────── */
      .folder-content {
        display: flex;
        flex-direction: column;
        gap: 20px;
        height: 100%;
        padding: 4px 0;
      }

      .no-folder-selected {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        height: 100%;
        min-height: 300px;
        opacity: 0.45;
        text-align: center;
        padding: 48px 24px;
      }
      .no-folder-icon {
        font-size: 3.5rem;
        width: 3.5rem;
        height: 3.5rem;
        margin-bottom: 16px;
      }
      .no-folder-message {
        margin: 0 0 6px;
        font-size: 1.1rem;
        font-weight: 500;
      }
      .no-folder-hint {
        margin: 0;
        font-size: 0.85rem;
        opacity: 0.6;
        max-width: 280px;
        line-height: 1.5;
      }

      /* ── Header ────────────────────────── */
      .content-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 16px;
        flex-wrap: wrap;
      }
      .header-left {
        display: flex;
        align-items: center;
        gap: 10px;
      }
      .header-folder-icon {
        font-size: 1.75rem;
        width: 1.75rem;
        height: 1.75rem;
        opacity: 0.7;
      }
      .folder-name {
        margin: 0;
        font-size: 1.35rem;
        font-weight: 600;
        letter-spacing: -0.01em;
      }
      .assoc-btn mat-icon {
        margin-right: 4px;
      }

      /* ── Summary chips ─────────────────── */
      .summary-chips {
        display: flex;
        gap: 8px;
        flex-wrap: wrap;
      }
      .summary-chip {
        display: inline-flex;
        align-items: center;
        gap: 4px;
        padding: 6px 14px;
        border-radius: 20px;
        font-size: 0.82rem;
        font-weight: 500;
        cursor: pointer;
        background: rgba(255, 255, 255, 0.04);
        border: 1px solid rgba(255, 255, 255, 0.08);
        transition: background 0.2s, border-color 0.2s;
        user-select: none;
      }
      .summary-chip:hover {
        background: rgba(255, 255, 255, 0.08);
      }
      .summary-chip.active {
        background: rgba(var(--mat-app-primary, 63, 81, 181), 0.15);
        border-color: rgba(var(--mat-app-primary, 63, 81, 181), 0.35);
      }
      .chip-icon {
        font-size: 14px;
        width: 14px;
        height: 14px;
      }

      /* ── Entity grid ───────────────────── */
      .entity-list-area {
        flex: 1;
        overflow-y: auto;
      }
      .entity-grid {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
        gap: 10px;
      }

      .entity-card {
        display: flex;
        align-items: center;
        gap: 10px;
        padding: 10px 12px;
        border-radius: 10px;
        border: 1px solid rgba(255, 255, 255, 0.06);
        background: rgba(255, 255, 255, 0.02);
        transition: transform 0.15s, box-shadow 0.15s, border-color 0.15s;
        position: relative;
        overflow: visible;
        cursor: default;
      }
      .entity-card:hover {
        border-color: rgba(255, 255, 255, 0.12);
        box-shadow: 0 2px 10px rgba(0, 0, 0, 0.2);
      }
      .entity-card.type-character {
        border-left: 3px solid #059669;
      }
      .entity-card.type-map {
        border-left: 3px solid #0891b2;
      }
      .entity-card.type-session {
        border-left: 3px solid #7c3aed;
      }

      .entity-card.cdk-drag-placeholder {
        opacity: 0.3;
      }
      .entity-card.cdk-drag-preview {
        background: rgba(30, 30, 40, 0.95);
        backdrop-filter: blur(4px);
        border-radius: 10px;
        box-shadow: 0 8px 24px rgba(0, 0, 0, 0.35);
      }

      .drag-indicator {
        display: flex;
        align-items: center;
        cursor: grab;
        color: rgba(255, 255, 255, 0.15);
        transition: color 0.15s;
        flex-shrink: 0;
      }
      .drag-indicator:hover {
        color: rgba(255, 255, 255, 0.45);
      }
      .drag-indicator mat-icon {
        font-size: 16px;
        width: 16px;
        height: 16px;
      }

      .card-icon {
        display: flex;
        align-items: center;
        justify-content: center;
        width: 36px;
        height: 36px;
        border-radius: 8px;
        flex-shrink: 0;
      }
      .card-icon mat-icon {
        font-size: 20px;
        width: 20px;
        height: 20px;
      }
      .icon-character {
        background: rgba(5, 150, 105, 0.15);
        color: #34d399;
      }
      .icon-map {
        background: rgba(8, 145, 178, 0.15);
        color: #22d3ee;
      }
      .icon-session {
        background: rgba(124, 58, 237, 0.15);
        color: #a78bfa;
      }

      .card-body {
        flex: 1;
        min-width: 0;
        display: flex;
        flex-direction: column;
        gap: 2px;
      }
      .entity-name {
        font-size: 0.9rem;
        font-weight: 500;
        color: inherit;
        text-decoration: none;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
        transition: color 0.15s;
      }
      .entity-name:hover {
        color: rgb(var(--mat-app-primary, 63, 81, 181));
        text-decoration: underline;
      }
      .entity-type-label {
        font-size: 0.7rem;
        opacity: 0.5;
        text-transform: uppercase;
        letter-spacing: 0.5px;
      }

      .disassoc-btn {
        width: 28px;
        height: 28px;
        line-height: 28px;
        opacity: 0;
        transition: opacity 0.15s;
        flex-shrink: 0;
      }
      .entity-card:hover .disassoc-btn {
        opacity: 1;
      }
      .disassoc-btn mat-icon {
        font-size: 16px;
        width: 16px;
        height: 16px;
        line-height: 16px;
      }

      /* ── Empty state ───────────────────── */
      .empty-state {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        padding: 64px 16px;
        text-align: center;
      }
      .empty-icon {
        font-size: 3rem;
        width: 3rem;
        height: 3rem;
        margin-bottom: 12px;
        opacity: 0.35;
      }
      .empty-message {
        margin: 0 0 4px;
        font-size: 1rem;
        opacity: 0.6;
      }
      .empty-hint {
        margin: 0;
        font-size: 0.85rem;
        opacity: 0.4;
        max-width: 320px;
        line-height: 1.4;
      }
    `,
  ],
})
export class FolderContentComponent {
  // ── Inputs ──────────────────────────────────────────────────
  readonly folder = input<CampaignFolder | null>(null);

  readonly entitySourceListId = uniqueEntitySourceId();

  // ── Dependencies ────────────────────────────────────────────
  private readonly store = inject(StoreService);
  private readonly dialog = inject(MatDialog);

  // ── State ───────────────────────────────────────────────────
  readonly activeFilter = signal<'all' | 'character' | 'map' | 'session'>('all');

  readonly entities = signal<EntityRef[]>([]);

  private readonly storeSub = signal<boolean>(false);

  constructor() {
    // Reload entities when folder changes
    effect(() => {
      const f = this.folder();
      if (f) {
        this.loadEntities(f);
      } else {
        this.entities.set([]);
      }
    });
  }

  private loadEntities(folder: CampaignFolder) {
    const ids = folder.entityIds;
    const refs: EntityRef[] = [];

    // Characters
    if (ids.characterIds?.length) {
      const allChars = this.store.snapshot('characters') as Character[];
      for (const id of ids.characterIds) {
        const char = allChars.find((c) => c.id === id);
        if (char) {
          refs.push({
            id: char.id,
            name: char.name,
            type: 'character',
            icon: 'person',
            route: `/personagens/${char.id}`,
          });
        }
      }
    }

    // Maps
    if (ids.mapIds?.length) {
      const allMaps = this.store.snapshot('maps') as MapData[];
      for (const id of ids.mapIds) {
        const map = allMaps.find((m) => m.id === id);
        if (map) {
          refs.push({
            id: map.id,
            name: map.name,
            type: 'map',
            icon: 'map',
            route: `/mapa/${map.id}`,
          });
        }
      }
    }

    // Sessions
    if (ids.sessionIds?.length) {
      const allSessions = this.store.snapshot('sessions') as SessionState[];
      for (const id of ids.sessionIds) {
        const session = allSessions.find((s) => s.id === id);
        if (session) {
          refs.push({
            id: session.id,
            name: `Sessão ${session.id.slice(0, 8)}`,
            type: 'session',
            icon: 'event',
            route: `/sessao/${session.id}`,
          });
        }
      }
    }

    this.entities.set(refs);
  }

  // ── Derived ─────────────────────────────────────────────────
  readonly characters = computed(() =>
    this.entities().filter((e) => e.type === 'character'),
  );
  readonly maps = computed(() =>
    this.entities().filter((e) => e.type === 'map'),
  );
  readonly sessions = computed(() =>
    this.entities().filter((e) => e.type === 'session'),
  );
  readonly allEntities = computed(() => this.entities());

  readonly filteredEntities = computed(() => {
    const filter = this.activeFilter();
    if (filter === 'all') return this.entities();
    return this.entities().filter((e) => e.type === filter);
  });

  // ── Actions ─────────────────────────────────────────────────
  setFilter(filter: 'all' | 'character' | 'map' | 'session') {
    this.activeFilter.set(filter);
  }

  typeLabel(type: string): string {
    switch (type) {
      case 'character':
        return 'Personagem';
      case 'map':
        return 'Mapa';
      case 'session':
        return 'Sessão';
      default:
        return type;
    }
  }

  openAssociateDialog() {
    const f = this.folder();
    if (!f) return;

    const dialogRef = this.dialog.open(EntitySelectorDialogComponent, {
      width: '520px',
      maxWidth: '95vw',
      data: {
        folderId: f.id,
        currentEntityIds: f.entityIds,
      } as EntitySelectorData,
    });

    dialogRef.afterClosed().subscribe((result) => {
      if (result) {
        // Update the folder's entityIds
        this.store.patch('campaigns', f.id, {
          entityIds: result,
        } as Partial<CampaignFolder>);
      }
    });
  }

  disassociate(entity: EntityRef) {
    const f = this.folder();
    if (!f) return;

    const typeLabel = this.typeLabel(entity.type);
    const dialogRef = this.dialog.open(ConfirmDialogComponent, {
      width: '420px',
      data: {
        title: 'Desassociar Entidade',
        message: `Tem certeza que deseja remover "${entity.name}" (${typeLabel}) desta pasta? A entidade original não será afetada.`,
        confirmText: 'Desassociar',
        cancelText: 'Cancelar',
      } as ConfirmDialogData,
    });

    dialogRef.afterClosed().subscribe((confirmed) => {
      if (!confirmed) return;

      const ids = { ...f.entityIds };
      switch (entity.type) {
        case 'character':
          ids.characterIds = ids.characterIds.filter((id) => id !== entity.id);
          break;
        case 'map':
          ids.mapIds = ids.mapIds.filter((id) => id !== entity.id);
          break;
        case 'session':
          ids.sessionIds = ids.sessionIds.filter((id) => id !== entity.id);
          break;
      }

      this.store.patch('campaigns', f.id, {
        entityIds: ids,
      } as Partial<CampaignFolder>);
    });
  }
}
