import { Component, inject, OnDestroy, OnInit, signal } from '@angular/core';
import { Router } from '@angular/router';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { MatTooltipModule } from '@angular/material/tooltip';
import { StoreService } from '../../core/store/store.service';
import type { MapData } from '../../core/models/map';
import { PageHeaderComponent } from '../../shared/components/page-header.component';
import { EmptyStateComponent } from '../../shared/components/empty-state.component';
import { ConfirmDialogComponent, ConfirmDialogData } from '../../shared';
import { MapFormDialogComponent, MapFormDialogData } from './map-form-dialog.component';

@Component({
  selector: 'app-map-list',
  standalone: true,
  imports: [
    MatButtonModule,
    MatCardModule,
    MatIconModule,
    MatTooltipModule,
    PageHeaderComponent,
    EmptyStateComponent,
  ],
  template: `
    <app-page-header title="Mapas" icon="map" [breadcrumbs]="[]" />

    <div class="list-container">
      <div class="toolbar-row">
        <button mat-raised-button color="primary" (click)="openCreateDialog()">
          <mat-icon>add</mat-icon>
          Novo Mapa
        </button>
      </div>

      @if (maps().length === 0) {
        <div class="empty-wrapper">
          <app-empty-state
            icon="map"
            message="Nenhum mapa criado ainda. Crie um mapa de mundo, de cidade ou de masmorra e adicione pontos de interesse."
            actionLabel="Criar primeiro mapa"
            (action)="openCreateDialog()"
          />
        </div>
      } @else {
        <div class="map-grid">
          @for (map of maps(); track map.id) {
            <mat-card class="map-card" (click)="openMap(map)">
              <div class="thumb">
                @if (map.backgroundImage) {
                  <img [src]="map.backgroundImage" [alt]="map.name" />
                } @else {
                  <div class="thumb-placeholder">
                    <mat-icon>map</mat-icon>
                    <span>Sem imagem</span>
                  </div>
                }
                <div class="poi-badge">
                  <mat-icon>place</mat-icon>
                  {{ map.markers.length }}
                </div>
              </div>
              <mat-card-content>
                <h3 class="map-name">{{ map.name }}</h3>
                @if (map.description) {
                  <p class="map-desc">{{ map.description }}</p>
                }
              </mat-card-content>
              <mat-card-actions align="end">
                <button
                  mat-icon-button
                  matTooltip="Editar"
                  (click)="openEditDialog(map); $event.stopPropagation()"
                  aria-label="Editar mapa"
                >
                  <mat-icon>edit</mat-icon>
                </button>
                <button
                  mat-icon-button
                  color="warn"
                  matTooltip="Excluir"
                  (click)="deleteMap(map); $event.stopPropagation()"
                  aria-label="Excluir mapa"
                >
                  <mat-icon>delete</mat-icon>
                </button>
              </mat-card-actions>
            </mat-card>
          }
        </div>
      }
    </div>
  `,
  styles: [
    `
      :host {
        display: block;
        height: 100%;
        overflow-y: auto;
      }

      .list-container {
        display: flex;
        flex-direction: column;
        gap: 16px;
        padding: 16px;
      }

      .toolbar-row {
        display: flex;
        align-items: center;
      }

      .empty-wrapper {
        display: flex;
        justify-content: center;
        align-items: center;
        min-height: 320px;
      }

      .map-grid {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
        gap: 16px;
      }

      .map-card {
        cursor: pointer;
        overflow: hidden;
        transition: transform 0.15s, box-shadow 0.15s, border-color 0.15s;
        border: 1px solid rgba(255, 255, 255, 0.08);
      }
      .map-card:hover {
        transform: translateY(-4px);
        box-shadow: 0 8px 24px rgba(0, 0, 0, 0.35);
        border-color: rgba(124, 77, 255, 0.45);
      }

      .thumb {
        position: relative;
        height: 150px;
        background: rgba(0, 0, 0, 0.35);
      }
      .thumb img {
        width: 100%;
        height: 100%;
        object-fit: cover;
        display: block;
      }
      .thumb-placeholder {
        height: 100%;
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        gap: 8px;
        color: rgba(255, 255, 255, 0.4);
      }
      .thumb-placeholder mat-icon {
        font-size: 3rem;
        width: 3rem;
        height: 3rem;
      }
      .thumb-placeholder span {
        font-size: 0.8rem;
      }

      .poi-badge {
        position: absolute;
        top: 8px;
        right: 8px;
        display: flex;
        align-items: center;
        gap: 4px;
        background: rgba(0, 0, 0, 0.65);
        color: #fff;
        padding: 4px 10px;
        border-radius: 14px;
        font-size: 0.8rem;
      }
      .poi-badge mat-icon {
        font-size: 15px;
        width: 15px;
        height: 15px;
      }

      .map-name {
        margin: 0 0 4px;
        font-size: 1.05rem;
        font-weight: 600;
      }
      .map-desc {
        margin: 0;
        font-size: 0.85rem;
        opacity: 0.6;
        display: -webkit-box;
        -webkit-line-clamp: 2;
        -webkit-box-orient: vertical;
        overflow: hidden;
      }

      mat-card-actions {
        padding: 4px 8px 8px;
      }
    `,
  ],
})
export class MapListComponent implements OnInit, OnDestroy {
  private readonly router = inject(Router);
  private readonly dialog = inject(MatDialog);
  private readonly store = inject(StoreService<MapData>);

  readonly maps = signal<MapData[]>([]);
  private unsubscribeHandle: { unsubscribe: () => void } | null = null;

  ngOnInit(): void {
    this.unsubscribeHandle = this.store.subscribe('maps', (items) => {
      this.maps.set(items as MapData[]);
    });
  }

  ngOnDestroy(): void {
    this.unsubscribeHandle?.unsubscribe();
  }

  openMap(map: MapData): void {
    this.router.navigate(['/mapa', map.id]);
  }

  openCreateDialog(): void {
    const ref = this.dialog.open(MapFormDialogComponent, {
      width: '520px',
      maxWidth: '95vw',
      data: {} as MapFormDialogData,
    });
    ref.afterClosed().subscribe((created) => {
      if (created) {
        this.router.navigate(['/mapa', created.id]);
      }
    });
  }

  openEditDialog(map: MapData): void {
    this.dialog.open(MapFormDialogComponent, {
      width: '520px',
      maxWidth: '95vw',
      data: { map } as MapFormDialogData,
    });
  }

  deleteMap(map: MapData): void {
    const ref = this.dialog.open(ConfirmDialogComponent, {
      width: '420px',
      data: {
        title: 'Excluir Mapa',
        message: `Tem certeza que deseja excluir "${map.name}"? Os pontos de interesse e submaps deste mapa serão removidos.`,
        confirmText: 'Excluir',
        cancelText: 'Cancelar',
      } as ConfirmDialogData,
    });
    ref.afterClosed().subscribe((confirmed) => {
      if (confirmed) {
        this.store.delete('maps', map.id);
      }
    });
  }
}
