import {
  AfterViewInit,
  Component,
  ElementRef,
  inject,
  OnDestroy,
  signal,
  viewChild,
} from '@angular/core';
import { Router, ActivatedRoute } from '@angular/router';
import { Subscription, firstValueFrom } from 'rxjs';
import { HttpErrorResponse } from '@angular/common/http';
import { FormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatSelectModule } from '@angular/material/select';
import { MatSnackBar, MatSnackBarModule } from '@angular/material/snack-bar';
import { MatTooltipModule } from '@angular/material/tooltip';
import { MapService } from './map.service';
import { DungeonService } from './dungeon.service';
import { MapsService } from '../../core/services/maps.service';
import type { DungeonCharacter, DungeonTool } from './dungeon.service';
import { MapConfigPanelComponent } from './map-config-panel.component';
import { PageHeaderComponent, BreadcrumbItem } from '../../shared/components/page-header.component';
import { LoadingSpinnerComponent } from '../../shared/components/loading-spinner.component';
import { EmptyStateComponent } from '../../shared/components/empty-state.component';
import { ConfirmDialogComponent, ConfirmDialogData } from '../../shared';
import { PoiDialogComponent, PoiDialogData, PoiDialogResult } from './poi-dialog.component';
import { MapFormDialogComponent, MapFormDialogData } from './map-form-dialog.component';
import type { MapData, MapMarker } from '../../core/models/map';
import type { Character } from '../../core/models/character';
import { StoreService } from '../../core/store/store.service';

interface DungeonToolOption {
  tool: DungeonTool;
  icon: string;
  label: string;
}

const CHARACTER_COLORS = [
  '#7c4dff', '#e53935', '#00c853', '#2979ff',
  '#ff6d00', '#00bcd4', '#ff4081', '#ffd600',
];

@Component({
  selector: 'app-map-view',
  standalone: true,
  imports: [
    FormsModule,
    MatButtonModule,
    MatFormFieldModule,
    MatIconModule,
    MatSelectModule,
    MatSnackBarModule,
    MatTooltipModule,
    MapConfigPanelComponent,
    PageHeaderComponent,
    LoadingSpinnerComponent,
    EmptyStateComponent,
  ],
  template: `
    <app-page-header
      [title]="mapTitle"
      icon="map"
      [breadcrumbs]="breadcrumbs"
    />

    <div class="map-stage">
      <div #mapContainer class="map-container"></div>

      @if (loading()) {
        <div class="overlay">
          <app-loading-spinner [isLoading]="true" message="Carregando mapa..." />
        </div>
      } @else if (error()) {
        <div class="overlay">
          <app-empty-state
            icon="map"
            [message]="error()!"
            actionLabel="Voltar"
            (action)="goBack()"
          />
        </div>
      } @else {
        <div class="toolbar-row">
          @if (parentMapId) {
            <button mat-stroked-button (click)="goToParent()">
              <mat-icon>arrow_back</mat-icon>
              Voltar ao mapa pai
            </button>
          }

          <button
            mat-stroked-button
            [class.active]="pinPlacementMode()"
            [disabled]="drawMode()"
            (click)="togglePinPlacement()"
          >
            <mat-icon>add_location</mat-icon>
            Adicionar Ponto
          </button>

          <button
            mat-stroked-button
            [class.active]="drawMode()"
            [disabled]="!hasImage()"
            [matTooltip]="hasImage() ? 'Desenhar a masmorra sobre a imagem' : 'Adicione uma imagem ao mapa para desenhar'"
            (click)="toggleDrawMode()"
          >
            <mat-icon>draw</mat-icon>
            Desenhar Masmorra
          </button>

          <button
            mat-icon-button
            [class.active]="gridVisible()"
            matTooltip="Mostrar/ocultar grade"
            (click)="toggleGrid()"
          >
            <mat-icon>grid_4x4</mat-icon>
          </button>

          @if (pinPlacementMode()) {
            <span class="hint">Clique no mapa para posicionar o ponto</span>
          }
          @if (drawMode()) {
            <span class="hint">Clique e arraste para pintar as células</span>
          }

          @if (currentMapData) {
            <span class="toolbar-spacer"></span>
            <button
              mat-raised-button
              color="primary"
              [disabled]="saving()"
              (click)="saveCurrentMap()"
            >
              <mat-icon>{{ saving() ? 'hourglass_top' : 'save' }}</mat-icon>
              {{ saving() ? 'Salvando...' : 'Salvar' }}
            </button>
            <button mat-stroked-button (click)="openEditMapDialog()">
              <mat-icon>edit</mat-icon>
              Editar Mapa
            </button>
            <button mat-stroked-button color="warn" (click)="deleteMap()">
              <mat-icon>delete</mat-icon>
              Excluir
            </button>
          }
        </div>

        @if (drawMode()) {
          <div class="tool-row">
            @for (option of dungeonTools; track option.tool) {
              <button
                mat-stroked-button
                class="tool-btn"
                [class.active]="dungeonTool() === option.tool"
                [matTooltip]="option.label"
                (click)="selectTool(option.tool)"
              >
                <mat-icon>{{ option.icon }}</mat-icon>
                {{ option.label }}
              </button>
            }

            @if (dungeonTool() === 'character') {
              <mat-form-field
                appearance="outline"
                class="char-select"
                subscriptSizing="dynamic"
              >
                <mat-label>Personagem</mat-label>
                <mat-select
                  [ngModel]="selectedCharacterId()"
                  (ngModelChange)="onCharacterSelected($event)"
                >
                  @for (character of characters(); track character.id) {
                    <mat-option [value]="character.id">
                      {{ character.name }}
                    </mat-option>
                  }
                </mat-select>
              </mat-form-field>
              @if (!selectedCharacterId()) {
                <span class="hint">Escolha um personagem para carimbar</span>
              }
            }
          </div>
        }

        @if (showConfigPanel) {
          <app-map-config-panel />
        }

        <button
          mat-fab
          class="fab-btn config-btn"
          (click)="showConfigPanel = !showConfigPanel"
          aria-label="Configurações do mapa"
        >
          <mat-icon>layers</mat-icon>
        </button>

        <button
          mat-fab
          class="fab-btn fullscreen-btn"
          (click)="toggleFullscreen()"
          aria-label="Alternar tela cheia"
        >
          <mat-icon>fullscreen</mat-icon>
        </button>
      }
    </div>
  `,
  styles: `
    :host {
      display: flex;
      flex-direction: column;
      width: 100%;
      height: calc(100vh - 64px - 48px);
      position: relative;
      overflow: hidden;
    }

    .map-stage {
      position: relative;
      flex: 1;
      min-height: 0;
    }

    .toolbar-row,
    .tool-row {
      position: relative;
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: 8px;
      row-gap: 8px;
      padding: 8px 16px;
      background: rgba(0,0,0,0.3);
      z-index: 10;
      flex-shrink: 0;
    }

    .tool-row {
      padding-top: 0;
      background: rgba(0,0,0,0.22);
    }

    .toolbar-row button.active,
    .tool-row button.active,
    button.active {
      background: rgba(124,77,255,0.25);
      border-color: #7c4dff;
    }

    .hint {
      font-size: 0.8125rem;
      color: rgba(255,255,255,0.6);
      font-style: italic;
    }

    .toolbar-spacer {
      flex: 1;
    }

    .tool-btn {
      min-width: 0;
    }

    .char-select {
      width: 180px;
      font-size: 0.85rem;
    }

    .map-container {
      position: absolute;
      inset: 0;
      width: 100%;
      height: 100%;
    }

    .overlay {
      position: absolute;
      inset: 0;
      z-index: 20;
      display: flex;
      align-items: center;
      justify-content: center;
      background: rgba(10, 10, 18, 0.6);
    }

    .fab-btn {
      position: absolute;
      z-index: 10;
    }

    .fullscreen-btn {
      bottom: 24px;
      right: 24px;
    }

    .config-btn {
      top: 16px;
      right: 16px;
    }

    @media (max-width: 480px) {
      .config-btn {
        top: 8px;
        right: 8px;
      }
      .fullscreen-btn {
        bottom: 16px;
        right: 16px;
      }
    }

    :host ::ng-deep app-page-header {
      flex-shrink: 0;
    }
  `,
})
export class MapViewComponent implements AfterViewInit, OnDestroy {
  private readonly router = inject(Router);
  private readonly route = inject(ActivatedRoute);
  private readonly dialog = inject(MatDialog);
  private readonly mapService = inject(MapService);
  private readonly dungeonService = inject(DungeonService);
  private readonly mapsService = inject(MapsService);
  private readonly snackBar = inject(MatSnackBar);
  private readonly store = inject(StoreService<MapData>);
  private readonly characterStore = inject<StoreService<Character>>(StoreService);

  private readonly mapContainer =
    viewChild.required<ElementRef<HTMLElement>>('mapContainer');

  protected showConfigPanel = false;
  protected readonly pinPlacementMode = signal(false);
  protected readonly drawMode = signal(false);
  protected readonly hasImage = signal(false);
  protected readonly gridVisible = signal(false);
  protected readonly saving = signal(false);
  protected readonly dungeonTool = signal<DungeonTool>('floor');
  protected readonly characters = signal<Character[]>([]);
  protected readonly selectedCharacterId = signal<string | null>(null);
  protected mapTitle = 'Mapa';
  protected breadcrumbs: BreadcrumbItem[] = [];
  protected parentMapId: string | null = null;
  protected currentMapData: MapData | null = null;
  readonly loading = signal(true);
  readonly error = signal<string | null>(null);

  protected readonly dungeonTools: DungeonToolOption[] = [
    { tool: 'floor', icon: 'crop_square', label: 'Piso' },
    { tool: 'wall', icon: 'border_outer', label: 'Parede' },
    { tool: 'false_wall', icon: 'border_style', label: 'Parede falsa' },
    { tool: 'water', icon: 'water_drop', label: 'Água' },
    { tool: 'difficult', icon: 'grass', label: 'Terreno difícil' },
    { tool: 'rubble', icon: 'scatter_plot', label: 'Escombros' },
    { tool: 'door', icon: 'door_front', label: 'Porta' },
    { tool: 'secret_door', icon: 'key', label: 'Porta secreta' },
    { tool: 'trap', icon: 'warning', label: 'Armadilha' },
    { tool: 'chest', icon: 'redeem', label: 'Baú' },
    { tool: 'mimic', icon: 'pest_control', label: 'Mímico' },
    { tool: 'character', icon: 'person_add', label: 'Personagem' },
    { tool: 'erase', icon: 'cleaning_services', label: 'Apagar' },
  ];

  private mapId: string | null = null;
  private routeSub: Subscription | null = null;
  private characterSub: { unsubscribe: () => void } | null = null;
  private clickUnregister: (() => void) | null = null;
  private featureUnregister: (() => void) | null = null;

  async ngAfterViewInit() {
    this.loading.set(true);
    this.error.set(null);

    try {
      await this.mapService.initialize(this.mapContainer().nativeElement, {
        zoom: 10,
        center: [-46.6333, -23.5505],
      });
    } catch (err) {
      console.error('Falha ao inicializar mapa:', err);
      this.error.set('Erro ao carregar mapa');
      this.loading.set(false);
      return;
    }

    this.setupClickHandler();

    this.characterSub = this.characterStore.subscribe('characters', (items) => {
      this.characters.set(items as Character[]);
      this.applyCharacter();
    });

    this.routeSub = this.route.paramMap.subscribe((params) => {
      this.mapId = params.get('id');
      void this.loadMapData();
    });
  }

  ngOnDestroy() {
    this.clickUnregister?.();
    this.featureUnregister?.();
    this.routeSub?.unsubscribe();
    this.characterSub?.unsubscribe();
    this.dungeonService.destroy();
    this.mapService.destroy();
  }

  toggleFullscreen(): void {
    this.mapService.toggleFullscreen(this.mapContainer().nativeElement);
  }

  protected toggleGrid(): void {
    this.mapService.toggleGrid().then(() => {
      this.gridVisible.set(this.mapService.isGridVisible());
    });
  }

  protected togglePinPlacement(): void {
    this.pinPlacementMode.update((v) => !v);
  }

  protected async toggleDrawMode(): Promise<void> {
    if (this.drawMode()) {
      this.dungeonService.disable();
      this.drawMode.set(false);
      return;
    }

    if (!this.hasImage()) return;

    this.pinPlacementMode.set(false);
    this.dungeonService.setTool(this.dungeonTool());
    await this.dungeonService.enable();
    this.drawMode.set(true);
  }

  protected selectTool(tool: DungeonTool): void {
    this.dungeonTool.set(tool);
    this.dungeonService.setTool(tool);
    if (tool === 'character') {
      this.applyCharacter();
    }
  }

  protected onCharacterSelected(id: string): void {
    this.selectedCharacterId.set(id);
    this.applyCharacter();
  }

  private applyCharacter(): void {
    const id = this.selectedCharacterId();
    const characters = this.characters();
    const character = characters.find((c) => c.id === id);
    if (!character) {
      this.dungeonService.setCharacter(null);
      return;
    }
    const color =
      CHARACTER_COLORS[characters.indexOf(character) % CHARACTER_COLORS.length];
    const dungeonCharacter: DungeonCharacter = {
      id: character.id,
      name: character.name,
      color,
    };
    this.dungeonService.setCharacter(dungeonCharacter);
  }

  protected goToParent(): void {
    if (this.parentMapId) {
      this.router.navigate(['/mapa', this.parentMapId]);
    }
  }

  protected goBack(): void {
    this.router.navigate(['/mapa']);
  }

  protected openEditMapDialog(): void {
    if (!this.currentMapData) return;

    const ref = this.dialog.open(MapFormDialogComponent, {
      width: '520px',
      maxWidth: '95vw',
      data: { map: this.currentMapData } as MapFormDialogData,
    });
    ref.afterClosed().subscribe((saved) => {
      if (saved) {
        void this.loadMapData();
      }
    });
  }

  protected deleteMap(): void {
    if (!this.currentMapData) return;

    const ref = this.dialog.open(ConfirmDialogComponent, {
      width: '420px',
      data: {
        title: 'Excluir Mapa',
        message: `Tem certeza que deseja excluir "${this.currentMapData.name}"?`,
        confirmText: 'Excluir',
        cancelText: 'Cancelar',
      } as ConfirmDialogData,
    });
    ref.afterClosed().subscribe((confirmed) => {
      if (confirmed && this.currentMapData) {
        this.store.delete('maps', this.currentMapData.id);
        this.router.navigate(['/mapa']);
      }
    });
  }

  protected async saveCurrentMap(): Promise<void> {
    if (!this.mapId) return;
    const map = this.store.snapshot('maps').find((m) => m.id === this.mapId);
    if (!map) {
      this.snackBar.open('Mapa não encontrado para salvar.', 'Fechar', { duration: 4000 });
      return;
    }

    this.saving.set(true);
    try {
      await firstValueFrom(this.mapsService.update(map.id, map));
      this.snackBar.open('Mapa salvo com sucesso!', 'OK', { duration: 3000 });
    } catch (error) {
      if (error instanceof HttpErrorResponse && error.status === 404) {
        try {
          await firstValueFrom(this.mapsService.create(map));
          this.snackBar.open('Mapa salvo com sucesso!', 'OK', { duration: 3000 });
        } catch (createError) {
          this.showSaveError(createError);
        }
      } else {
        this.showSaveError(error);
      }
    } finally {
      this.saving.set(false);
    }
  }

  private showSaveError(error: unknown): void {
    this.snackBar.open(`Falha ao salvar: ${describeHttpError(error)}`, 'Fechar', {
      duration: 6000,
    });
  }

  private async loadMapData(): Promise<void> {
    this.loading.set(true);

    this.dungeonService.disable();
    this.drawMode.set(false);

    if (this.mapId) {
      const mapData = this.store.snapshot('maps').find((m) => m.id === this.mapId);
      if (mapData) {
        this.currentMapData = mapData;
        this.mapService.setCurrentMapId(this.mapId);
        this.mapTitle = mapData.name;

        const hierarchy = this.mapService.getMapHierarchy(this.mapId);
        this.breadcrumbs = hierarchy.map((m, i) => ({
          label: m.name,
          route: i < hierarchy.length - 1 ? `/mapa/${m.id}` : undefined,
        }));

        this.parentMapId = this.mapService.getParentMapId(this.mapId);

        if (mapData.backgroundImage) {
          await this.mapService.setImageBackground(
            mapData.backgroundImage,
            mapData.width || 1024,
            mapData.height || 768,
          );
          this.hasImage.set(true);
        } else {
          await this.mapService.clearImageBackground();
          this.hasImage.set(false);
        }

        await this.mapService.setGridConfig(mapData.grid);
        await this.mapService.setGridVisible(false);
        this.gridVisible.set(this.mapService.isGridVisible());

        await this.dungeonService.configure({
          mapId: this.mapId,
          extent: [0, 0, mapData.width || 1024, mapData.height || 768],
          cellSize: mapData.grid?.cellSize ?? 50,
          columns: mapData.grid?.columns ?? 24,
          rows: mapData.grid?.rows ?? 18,
        });
        await this.dungeonService.load(mapData.dungeon);

        await this.mapService.renderPois(mapData.markers ?? []);
        await this.mapService.showPoisLayer();
        await this.mapService.renderSubmapPins(mapData.submaps ?? []);
        await this.mapService.showSubmapPinsLayer();

        this.loading.set(false);
        return;
      }

      this.currentMapData = null;
      this.mapService.setCurrentMapId(null);
      this.error.set('Mapa não encontrado');
      this.loading.set(false);
      return;
    }

    this.currentMapData = null;
    this.mapService.setCurrentMapId(null);
    this.mapTitle = 'Mapa';
    this.breadcrumbs = [];
    this.parentMapId = null;
    this.loading.set(false);
  }

  private setupClickHandler(): void {
    this.featureUnregister = this.mapService.onFeatureClick((result) => {
      if (this.pinPlacementMode() || this.drawMode()) return;

      if (result.type === 'poi' && result.marker) {
        this.openPoiDialog(result.marker);
      } else if (result.type === 'submap' && result.targetMapId) {
        this.router.navigate(['/mapa', result.targetMapId]);
      }
    });

    this.clickUnregister = this.mapService.onMapClick((coords) => {
      if (!this.pinPlacementMode() || this.drawMode()) return;

      this.pinPlacementMode.set(false);

      const ref = this.dialog.open(PoiDialogComponent, {
        data: {
          x: coords[0],
          y: coords[1],
          availableMaps: this.mapService
            .getAllMaps()
            .filter((m) => m.id !== this.mapId),
          currentMapId: this.mapId,
        } as PoiDialogData,
        width: '460px',
        maxWidth: '95vw',
      });

      ref.afterClosed().subscribe((result: PoiDialogResult) => {
        if (result?.action === 'save' && result.marker) {
          this.applyNewSubmap(result);
          void this.mapService.addPoi(result.marker);
        }
      });
    });
  }

  private openPoiDialog(marker: MapMarker): void {
    const ref = this.dialog.open(PoiDialogComponent, {
      data: {
        x: marker.x,
        y: marker.y,
        existing: marker,
        availableMaps: this.mapService
          .getAllMaps()
          .filter((m) => m.id !== this.mapId),
        currentMapId: this.mapId,
      } as PoiDialogData,
      width: '460px',
      maxWidth: '95vw',
    });

    ref.afterClosed().subscribe((result: PoiDialogResult) => {
      if (result?.action === 'save' && result.marker) {
        this.applyNewSubmap(result);
        void this.mapService.updatePoi(result.marker);
      } else if (result?.action === 'delete') {
        void this.mapService.deletePoi(marker.id);
      } else if (result?.action === 'open' && marker.targetMapId) {
        this.router.navigate(['/mapa', marker.targetMapId]);
      }
    });
  }

  private applyNewSubmap(result: PoiDialogResult): void {
    if (!result || result.action !== 'save' || !result.newSubmap) return;
    const submap = this.mapService.createSubmap(
      result.newSubmap.name,
      result.newSubmap.kind,
      result.newSubmap.image,
    );
    result.marker.targetMapId = submap.id;
  }
}

function describeHttpError(error: unknown): string {
  if (error instanceof HttpErrorResponse) {
    const detail = (error.error as { message?: unknown } | null)?.message;
    if (detail) {
      return `${error.status} ${Array.isArray(detail) ? detail.join(', ') : String(detail)}`;
    }
    return `${error.status} ${error.statusText || error.message}`;
  }
  return error instanceof Error ? error.message : 'erro desconhecido';
}
