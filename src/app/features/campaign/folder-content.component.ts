import {
  Component,
  inject,
  input,
  signal,
  computed,
  effect,
  OnDestroy,
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
import { CampaignCoverService } from './campaign-cover.service';
import {
  CampaignCoverDialogComponent,
  CampaignCoverDialogData,
} from './campaign-cover-dialog.component';
import {
  CampaignGuideDialogComponent,
  CampaignGuideDialogData,
} from './campaign-guide-dialog.component';

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

function isValidHttpUrl(value: string): boolean {
  try {
    const url = new URL(value);
    return url.protocol === 'http:' || url.protocol === 'https:';
  } catch {
    return false;
  }
}

function extractGoogleDocId(value: string): string | null {
  const match = value.match(/\/document\/d\/([a-zA-Z0-9_-]+)/);
  return match?.[1] ?? null;
}

function extractDocsUrlFromHtml(html: string): string | null {
  const match = html.match(/https?:\/\/docs\.google\.com\/document\/d\/[a-zA-Z0-9_/?=&.-]+/);
  return match?.[0] ?? null;
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
        <!-- Cover -->
        @if (coverUrl(); as url) {
          <div class="cover-wrapper">
            <img
              [src]="url"
              class="cover-banner"
              [alt]="'Capa de ' + folder.name"
            />
            <button
              mat-icon-button
              class="cover-edit-btn"
              aria-label="Alterar capa"
              (click)="openCoverDialog()"
            >
              <mat-icon>photo_camera</mat-icon>
            </button>
          </div>
        } @else {
          <button
            type="button"
            class="cover-placeholder"
            (click)="openCoverDialog()"
          >
            <mat-icon>add_photo_alternate</mat-icon>
            Adicionar capa
          </button>
        }

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

        <!-- Campaign guide (Google Docs) -->
        <div class="guide-section">
          @if (editingGuideUrl()) {
            <div class="guide-edit">
              <input
                [value]="guideDraft()"
                (input)="guideDraft.set($any($event.target).value)"
                (blur)="saveGuideUrl()"
                (keydown.enter)="saveGuideUrl()"
                (keydown.escape)="cancelGuideEdit()"
                placeholder="https://docs.google.com/document/d/..."
                aria-label="Link do guia da campanha (Google Docs)"
                class="guide-input"
                autofocus
              />
              @if (guideDraftError()) {
                <span class="guide-error">
                  URL inválida. Informe um endereço completo iniciando com http:// ou https://
                </span>
              }
            </div>
          } @else if (guideDoc(); as guide) {
            <div
              class="guide-card"
              role="button"
              tabindex="0"
              (click)="openGuideDialog(guide)"
              (keydown.enter)="openGuideDialog(guide)"
              aria-label="Abrir guia da campanha"
            >
              <div class="guide-card-icon">
                <mat-icon>description</mat-icon>
              </div>
              <div class="guide-card-body">
                <span class="guide-card-name">Guia da campanha</span>
                <span class="guide-card-sub">Documento Google Docs</span>
              </div>
              <button
                mat-icon-button
                type="button"
                class="guide-card-action"
                (click)="startGuideEdit(); $event.stopPropagation()"
                aria-label="Editar link do guia"
              >
                <mat-icon>edit</mat-icon>
              </button>
              <button
                mat-icon-button
                type="button"
                class="guide-card-action"
                (click)="removeGuide(); $event.stopPropagation()"
                aria-label="Remover guia"
              >
                <mat-icon>link_off</mat-icon>
              </button>
            </div>
          } @else {
            <button type="button" class="guide-add" (click)="startGuideEdit()">
              <mat-icon>add_link</mat-icon>
              Adicionar guia da campanha (Google Docs)
            </button>
          }
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

      /* ── Cover ──────────────────────────── */
      .cover-wrapper {
        position: relative;
        border-radius: 12px;
        overflow: hidden;
      }
      .cover-banner {
        display: block;
        width: 100%;
        height: 180px;
        object-fit: cover;
        border: 1px solid rgba(255, 255, 255, 0.08);
        border-radius: 12px;
      }
      .cover-edit-btn {
        position: absolute;
        top: 8px;
        right: 8px;
        background: rgba(0, 0, 0, 0.55);
      }
      .cover-edit-btn:hover {
        background: rgba(0, 0, 0, 0.75);
      }
      .cover-placeholder {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        gap: 8px;
        width: 100%;
        height: 120px;
        border: 2px dashed rgba(255, 255, 255, 0.2);
        border-radius: 12px;
        background: rgba(255, 255, 255, 0.02);
        color: rgba(255, 255, 255, 0.55);
        cursor: pointer;
        font-size: 0.9rem;
        transition: border-color 0.2s, background 0.2s, color 0.2s;
      }
      .cover-placeholder:hover {
        border-color: rgba(255, 255, 255, 0.4);
        background: rgba(255, 255, 255, 0.05);
        color: rgba(255, 255, 255, 0.85);
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

      /* ── Campaign guide (Google Docs) ─── */
      .guide-section {
        display: flex;
      }
      .guide-card {
        display: flex;
        align-items: center;
        gap: 10px;
        padding: 10px 12px;
        border-radius: 10px;
        border: 1px solid rgba(255, 255, 255, 0.12);
        background: rgba(255, 255, 255, 0.03);
        cursor: pointer;
        flex: 1;
        max-width: 420px;
        transition: background 0.15s, border-color 0.15s, box-shadow 0.15s;
      }
      .guide-card:hover,
      .guide-card:focus-visible {
        background: rgba(255, 255, 255, 0.07);
        border-color: rgba(255, 255, 255, 0.25);
        box-shadow: 0 2px 10px rgba(0, 0, 0, 0.2);
        outline: none;
      }
      .guide-card-icon {
        display: flex;
        align-items: center;
        justify-content: center;
        width: 36px;
        height: 36px;
        border-radius: 8px;
        flex-shrink: 0;
        background: rgba(49, 130, 206, 0.15);
        color: #63b3ed;
      }
      .guide-card-icon mat-icon {
        font-size: 20px;
        width: 20px;
        height: 20px;
      }
      .guide-card-body {
        flex: 1;
        min-width: 0;
        display: flex;
        flex-direction: column;
        gap: 2px;
      }
      .guide-card-name {
        font-size: 0.9rem;
        font-weight: 500;
      }
      .guide-card-sub {
        font-size: 0.7rem;
        opacity: 0.5;
        text-transform: uppercase;
        letter-spacing: 0.5px;
      }
      .guide-card-action {
        width: 28px;
        height: 28px;
        line-height: 28px;
        opacity: 0;
        transition: opacity 0.15s;
        flex-shrink: 0;
      }
      .guide-card:hover .guide-card-action,
      .guide-card:focus-within .guide-card-action {
        opacity: 1;
      }
      .guide-card-action mat-icon {
        font-size: 16px;
        width: 16px;
        height: 16px;
        line-height: 16px;
      }
      .guide-add {
        display: inline-flex;
        align-items: center;
        gap: 6px;
        padding: 6px 14px;
        border-radius: 8px;
        border: 2px dashed rgba(255, 255, 255, 0.2);
        background: transparent;
        color: rgba(255, 255, 255, 0.55);
        cursor: pointer;
        font-size: 0.85rem;
        transition: border-color 0.2s, background 0.2s, color 0.2s;
      }
      .guide-add:hover {
        border-color: rgba(255, 255, 255, 0.4);
        background: rgba(255, 255, 255, 0.05);
        color: rgba(255, 255, 255, 0.85);
      }
      .guide-add mat-icon {
        font-size: 18px;
        width: 18px;
        height: 18px;
      }
      .guide-edit {
        display: flex;
        flex-direction: column;
        gap: 4px;
        width: 100%;
        max-width: 520px;
      }
      .guide-input {
        width: 100%;
        font-size: 0.85rem;
        border: 1px solid rgba(var(--mat-app-primary, 63, 81, 181), 0.5);
        border-radius: 6px;
        padding: 8px 10px;
        background: rgba(0, 0, 0, 0.2);
        color: inherit;
        outline: none;
      }
      .guide-error {
        font-size: 0.75rem;
        color: #f87171;
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
export class FolderContentComponent implements OnDestroy {
  // ── Inputs ──────────────────────────────────────────────────
  readonly folder = input<CampaignFolder | null>(null);

  readonly entitySourceListId = uniqueEntitySourceId();

  // ── Dependencies ────────────────────────────────────────────
  private readonly store = inject(StoreService);
  private readonly dialog = inject(MatDialog);
  private readonly coverService = inject(CampaignCoverService);

  // ── State ───────────────────────────────────────────────────
  readonly activeFilter = signal<'all' | 'character' | 'map' | 'session'>('all');

  readonly entities = signal<EntityRef[]>([]);

  readonly coverUrl = signal<string | null>(null);
  private loadedCoverFileId: string | null = null;

  private readonly storeSub = signal<boolean>(false);

  // ── Campaign guide (Google Docs) ────────────────────────────
  readonly editingGuideUrl = signal(false);
  readonly guideDraft = signal('');
  readonly guideDraftError = signal(false);

  /** Validated Google Doc info, or null when absent/invalid. */
  readonly guideDoc = computed(() => {
    const raw = this.folder()?.googleDocUrl?.trim();
    if (!raw || !isValidHttpUrl(raw)) return null;
    return {
      url: raw,
      docId: extractGoogleDocId(raw),
    };
  });

  constructor() {
    // Reload entities when folder changes
    effect(() => {
      const f = this.folder();
      if (f) {
        this.loadEntities(f);
        this.migrateLegacyGuide(f);
      } else {
        this.entities.set([]);
      }
    });

    // Load the folder cover when it changes
    effect(() => {
      const f = this.folder();
      const coverFileId = f?.coverFileId ?? null;
      if (coverFileId !== this.loadedCoverFileId) {
        if (this.loadedCoverFileId) this.coverService.revokeFull(this.loadedCoverFileId);
        this.loadedCoverFileId = coverFileId;
        if (coverFileId) {
          this.coverService.loadFull(coverFileId).then((url) => {
            if (this.folder()?.coverFileId === coverFileId) this.coverUrl.set(url);
          });
        } else {
          this.coverUrl.set(null);
        }
      }
    });
  }

  ngOnDestroy() {
    if (this.loadedCoverFileId) this.coverService.revokeFull(this.loadedCoverFileId);
  }

  /**
   * Migrate the guide link pasted into the legacy in-app editor (guideHtml)
   * into googleDocUrl, so it becomes a folder item. Runs once per folder.
   */
  private migrateLegacyGuide(f: CampaignFolder) {
    const html = f.guideHtml ?? '';
    if (f.googleDocUrl || !html) return;
    const url = extractDocsUrlFromHtml(html);
    if (!url) return;
    this.store.patch('campaigns', f.id, {
      googleDocUrl: url,
      guideHtml: undefined,
    } as Partial<CampaignFolder>);
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

  // ── Campaign guide actions ─────────────────────────────────
  startGuideEdit() {
    this.guideDraft.set(this.folder()?.googleDocUrl ?? '');
    this.guideDraftError.set(false);
    this.editingGuideUrl.set(true);
  }

  saveGuideUrl() {
    const f = this.folder();
    if (!f) {
      this.editingGuideUrl.set(false);
      return;
    }
    const draft = this.guideDraft().trim();

    if (draft === '') {
      if (f.googleDocUrl) {
        this.store.patch('campaigns', f.id, {
          googleDocUrl: undefined,
        } as Partial<CampaignFolder>);
      }
      this.editingGuideUrl.set(false);
      return;
    }

    if (!isValidHttpUrl(draft)) {
      this.guideDraftError.set(true);
      return;
    }

    this.store.patch('campaigns', f.id, {
      googleDocUrl: draft,
    } as Partial<CampaignFolder>);
    this.editingGuideUrl.set(false);
  }

  cancelGuideEdit() {
    this.editingGuideUrl.set(false);
    this.guideDraftError.set(false);
  }

  openGuideDialog(guide: { url: string; docId: string | null }) {
    const f = this.folder();
    if (!f) return;
    if (!guide.docId) {
      window.open(guide.url, '_blank', 'noopener,noreferrer');
      return;
    }
    this.dialog.open(CampaignGuideDialogComponent, {
      width: 'min(860px, 94vw)',
      maxWidth: '96vw',
      data: {
        docId: guide.docId,
        editUrl: guide.url,
        folderName: f.name,
      } as CampaignGuideDialogData,
    });
  }

  removeGuide() {
    const f = this.folder();
    if (!f) return;

    const dialogRef = this.dialog.open(ConfirmDialogComponent, {
      width: '420px',
      data: {
        title: 'Remover Guia',
        message: 'Tem certeza que deseja remover o link do guia da campanha? O documento no Google Docs não será afetado.',
        confirmText: 'Remover',
        cancelText: 'Cancelar',
      } as ConfirmDialogData,
    });

    dialogRef.afterClosed().subscribe((confirmed) => {
      if (!confirmed) return;
      this.store.patch('campaigns', f.id, {
        googleDocUrl: undefined,
      } as Partial<CampaignFolder>);
    });
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

  openCoverDialog() {
    const f = this.folder();
    if (!f) return;

    const dialogRef = this.dialog.open(CampaignCoverDialogComponent, {
      width: 'min(520px, 92vw)',
      maxWidth: '95vw',
      data: {
        folderName: f.name,
        currentCoverFileId: f.coverFileId ?? null,
      } as CampaignCoverDialogData,
    });

    dialogRef.afterClosed().subscribe((result?: { fileId: string | null }) => {
      if (result === undefined) return;
      this.store.patch('campaigns', f.id, {
        coverFileId: result.fileId,
      } as Partial<CampaignFolder>);
    });
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
