import { Component, inject, OnInit, signal, computed } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import {
  MatDialogModule,
  MatDialogRef,
  MAT_DIALOG_DATA,
} from '@angular/material/dialog';
import { MatButtonModule } from '@angular/material/button';
import { MatChipsModule } from '@angular/material/chips';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatIconModule } from '@angular/material/icon';
import { MatListModule } from '@angular/material/list';
import { StoreService } from '../../core';
import type { CampaignFolder } from '../../core';
import type { Character } from '../../core';
import type { MapData } from '../../core';
import type { SessionState } from '../../core';

// ─── Dialog data ─────────────────────────────────────────────
export interface EntitySelectorData {
  folderId: string;
  currentEntityIds: CampaignFolder['entityIds'];
}

// ─── Internal item model ─────────────────────────────────────
interface SelectableEntity {
  id: string;
  name: string;
  selected: boolean;
}

// ─── Component ────────────────────────────────────────────────
@Component({
  selector: 'app-entity-selector-dialog',
  standalone: true,
  imports: [
    CommonModule,
    FormsModule,
    MatDialogModule,
    MatButtonModule,
    MatChipsModule,
    MatCheckboxModule,
    MatIconModule,
    MatListModule,
  ],
  template: `
    <h2 mat-dialog-title>Associar Entidades</h2>

    <mat-dialog-content class="dialog-content">
      <!-- Type chips -->
      <mat-chip-listbox
        [value]="activeType()"
        (change)="setType($event.value)"
        class="type-chips"
        aria-label="Filtrar por tipo"
      >
        <mat-chip-option value="characters" [class.active-chip]="activeType() === 'characters'">
          <mat-icon matChipAvatar>person</mat-icon>
          Personagens
        </mat-chip-option>
        <mat-chip-option value="maps" [class.active-chip]="activeType() === 'maps'">
          <mat-icon matChipAvatar>map</mat-icon>
          Mapas
        </mat-chip-option>
        <mat-chip-option value="sessions" [class.active-chip]="activeType() === 'sessions'">
          <mat-icon matChipAvatar>event</mat-icon>
          Sessões
        </mat-chip-option>
      </mat-chip-listbox>

      <!-- Entity list -->
      <div class="entity-list-wrapper">
        @if (loading()) {
          <div class="loading-state">
            <mat-icon class="loading-icon">hourglass_top</mat-icon>
            <span>Carregando...</span>
          </div>
        } @else if (filteredEntities().length === 0) {
          <div class="empty-state">
            <mat-icon class="empty-icon">search_off</mat-icon>
            <span>Nenhum {{ typeLabel() }} disponível</span>
          </div>
        } @else {
          <mat-selection-list class="entity-list" [multiple]="true" #entityList>
            @for (entity of filteredEntities(); track entity.id) {
              <mat-list-option
                [value]="entity.id"
                [selected]="entity.selected"
                (selectionChange)="onOptionChange(entity.id, $any($event))"
                class="entity-option"
              >
                <mat-icon matListItemIcon>
                  {{ activeType() === 'characters' ? 'person' : activeType() === 'maps' ? 'map' : 'event' }}
                </mat-icon>
                <span matListItemTitle>{{ entity.name }}</span>
              </mat-list-option>
            }
          </mat-selection-list>
        }
      </div>
    </mat-dialog-content>

    <mat-dialog-actions align="end">
      <button mat-button (click)="onCancel()">Cancelar</button>
      <button
        mat-raised-button
        color="primary"
        (click)="onConfirm()"
        [disabled]="totalSelected() === 0"
      >
        <mat-icon>check</mat-icon>
        Confirmar ({{ totalSelected() }})
      </button>
    </mat-dialog-actions>
  `,
  styles: [
    `
      :host {
        display: block;
      }

      .dialog-content {
        min-width: 420px;
        max-width: 520px;
        min-height: 320px;
        display: flex;
        flex-direction: column;
        gap: 16px;
      }

      /* ── Chips ─────────────────────────── */
      .type-chips {
        display: flex;
        gap: 8px;
        flex-wrap: wrap;
      }
      .type-chips .active-chip {
        --mdc-chip-selected-label-text-color: inherit;
      }

      /* ── Entity list ───────────────────── */
      .entity-list-wrapper {
        flex: 1;
        min-height: 180px;
        max-height: 340px;
        overflow-y: auto;
        border: 1px solid rgba(255, 255, 255, 0.08);
        border-radius: 8px;
        background: rgba(0, 0, 0, 0.15);
      }

      .entity-list {
        padding: 0;
      }

      .entity-option {
        border-bottom: 1px solid rgba(255, 255, 255, 0.04);
      }
      .entity-option:last-child {
        border-bottom: none;
      }

      /* ── States ────────────────────────── */
      .loading-state,
      .empty-state {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        padding: 48px 16px;
        gap: 8px;
        opacity: 0.55;
        text-align: center;
      }

      .loading-icon,
      .empty-icon {
        font-size: 2rem;
        width: 2rem;
        height: 2rem;
      }
    `,
  ],
})
export class EntitySelectorDialogComponent implements OnInit {
  private readonly dialogRef = inject(
    MatDialogRef<EntitySelectorDialogComponent>,
  );
  readonly data = inject<EntitySelectorData>(MAT_DIALOG_DATA);
  private readonly store = inject(StoreService);

  // ── State ─────────────────────────────────────────────────
  readonly activeType = signal<'characters' | 'maps' | 'sessions'>('characters');
  readonly loading = signal(true);

  // Entities grouped by type
  private characters = signal<SelectableEntity[]>([]);
  private maps = signal<SelectableEntity[]>([]);
  private sessions = signal<SelectableEntity[]>([]);

  // Selected counts
  private selectedChars = signal<Set<string>>(new Set());
  private selectedMaps = signal<Set<string>>(new Set());
  private selectedSessions = signal<Set<string>>(new Set());

  readonly totalSelected = computed(
    () =>
      this.selectedChars().size +
      this.selectedMaps().size +
      this.selectedSessions().size,
  );

  readonly filteredEntities = computed(() => {
    switch (this.activeType()) {
      case 'characters':
        return this.characters();
      case 'maps':
        return this.maps();
      case 'sessions':
        return this.sessions();
    }
  });

  readonly typeLabel = computed(() => {
    switch (this.activeType()) {
      case 'characters':
        return 'personagem';
      case 'maps':
        return 'mapa';
      case 'sessions':
        return 'sessão';
    }
  });

  // ── Lifecycle ─────────────────────────────────────────────
  ngOnInit() {
    this.loadEntities();
  }

  private loadEntities() {
    this.loading.set(true);

    // Load all entity types in parallel
    const chars = this.store.snapshot('characters') as Character[];
    const maps = this.store.snapshot('maps') as MapData[];
    const sessions = this.store.snapshot('sessions') as SessionState[];

    const current = this.data.currentEntityIds;
    const currentCharSet = new Set(current.characterIds ?? []);
    const currentMapSet = new Set(current.mapIds ?? []);
    const currentSessionSet = new Set(current.sessionIds ?? []);

    this.characters.set(
      chars.map((c) => ({
        id: c.id,
        name: c.name,
        selected: currentCharSet.has(c.id),
      })),
    );
    this.selectedChars.set(new Set(currentCharSet));

    this.maps.set(
      maps.map((m) => ({
        id: m.id,
        name: m.name,
        selected: currentMapSet.has(m.id),
      })),
    );
    this.selectedMaps.set(new Set(currentMapSet));

    this.sessions.set(
      sessions.map((s) => ({
        id: s.id,
        name: `Sessão ${s.id.slice(0, 8)}`,
        selected: currentSessionSet.has(s.id),
      })),
    );
    this.selectedSessions.set(new Set(currentSessionSet));

    this.loading.set(false);
  }

  // ── Actions ───────────────────────────────────────────────
  setType(type: string) {
    this.activeType.set(type as 'characters' | 'maps' | 'sessions');
  }

  onOptionChange(id: string, event: { selected: boolean }) {
    const selected = event.selected;
    const type = this.activeType();
    switch (type) {
      case 'characters': {
        const next = new Set(this.selectedChars());
        if (selected) next.add(id);
        else next.delete(id);
        this.selectedChars.set(next);
        this.characters.update((list) =>
          list.map((e) => (e.id === id ? { ...e, selected } : e)),
        );
        break;
      }
      case 'maps': {
        const next = new Set(this.selectedMaps());
        if (selected) next.add(id);
        else next.delete(id);
        this.selectedMaps.set(next);
        this.maps.update((list) =>
          list.map((e) => (e.id === id ? { ...e, selected } : e)),
        );
        break;
      }
      case 'sessions': {
        const next = new Set(this.selectedSessions());
        if (selected) next.add(id);
        else next.delete(id);
        this.selectedSessions.set(next);
        this.sessions.update((list) =>
          list.map((e) => (e.id === id ? { ...e, selected } : e)),
        );
        break;
      }
    }
  }

  onConfirm() {
    this.dialogRef.close({
      characterIds: Array.from(this.selectedChars()),
      mapIds: Array.from(this.selectedMaps()),
      sessionIds: Array.from(this.selectedSessions()),
    } as CampaignFolder['entityIds']);
  }

  onCancel() {
    this.dialogRef.close();
  }
}
