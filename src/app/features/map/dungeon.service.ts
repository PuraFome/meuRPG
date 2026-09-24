import { Injectable, inject, NgZone } from '@angular/core';
import { MapService } from './map.service';
import { StoreService } from '../../core/store/store.service';
import type {
  DungeonTileType,
  DungeonTiles,
  MapData,
} from '../../core/models/map';

type OlMap = import('ol/Map').default;
type OlVectorLayer = import('ol/layer/Vector').default;
type OlVectorSource = import('ol/source/Vector').default;
type OlFeature = import('ol/Feature').default;

/** Ferramenta ativa no editor de masmorra. `erase` remove a célula. */
export type DungeonTool = DungeonTileType | 'erase';

export interface DungeonConfig {
  mapId: string;
  /** Extensão em pixels da imagem: [minX, minY, maxX, maxY]. */
  extent: [number, number, number, number];
  cellSize: number;
  columns: number;
  rows: number;
}

interface TileStyle {
  fill: string;
  stroke: string;
}

/** Paleta por tipo de célula — precisa ser visível sobre a imagem de fundo. */
const TILE_STYLES: Record<DungeonTileType, TileStyle> = {
  floor: { fill: 'rgba(214, 184, 138, 0.55)', stroke: 'rgba(120, 88, 48, 0.75)' },
  wall: { fill: 'rgba(58, 58, 68, 0.92)', stroke: 'rgba(16, 16, 22, 1)' },
  door: { fill: 'rgba(255, 179, 0, 0.9)', stroke: 'rgba(120, 80, 0, 1)' },
  water: { fill: 'rgba(33, 150, 243, 0.6)', stroke: 'rgba(13, 71, 161, 0.85)' },
  difficult: { fill: 'rgba(76, 175, 80, 0.55)', stroke: 'rgba(27, 94, 32, 0.85)' },
};

/**
 * Editor de masmorra baseado em grade.
 *
 * Pinta células (`cellSize` px) diretamente sobre a imagem do mapa usando uma
 * projeção em pixels. As células preenchidas são guardadas esparsamente em
 * `MapData.dungeon` e persistidas com um pequeno debounce para não escrever no
 * armazenamento a cada célula pintada.
 */
@Injectable({ providedIn: 'root' })
export class DungeonService {
  private readonly zone = inject(NgZone);
  private readonly mapService = inject(MapService);
  private readonly store = inject(StoreService<MapData>);

  private config: DungeonConfig | null = null;
  private tiles: DungeonTiles = {};
  private tool: DungeonTool = 'floor';

  private layer: OlVectorLayer | null = null;
  private featuresByKey = new Map<string, OlFeature>();
  private OL: {
    Feature: typeof import('ol/Feature').default;
    Polygon: typeof import('ol/geom/Polygon').default;
    Style: typeof import('ol/style/Style').default;
    Fill: typeof import('ol/style/Fill').default;
    Stroke: typeof import('ol/style/Stroke').default;
    VectorLayer: typeof import('ol/layer/Vector').default;
    VectorSource: typeof import('ol/source/Vector').default;
    DragPan: typeof import('ol/interaction/DragPan').default;
  } | null = null;

  private enabled = false;
  private visible = false;
  private painting = false;
  private lastKey: string | null = null;
  private unregisters: (() => void)[] = [];
  private saveTimer: ReturnType<typeof setTimeout> | null = null;

  // ──────────────────────────────────────────
  //  Configuração / carga
  // ──────────────────────────────────────────

  /** (Re)configura o editor para o mapa atual. */
  async configure(config: DungeonConfig): Promise<void> {
    this.disable();
    this.destroyLayer();
    this.config = config;
    await this.loadOl();
    if (this.visible) {
      await this.show();
    }
  }

  /** Carrega as células salvas e renderiza. */
  async load(tiles: DungeonTiles | undefined): Promise<void> {
    this.tiles = { ...(tiles ?? {}) };
    await this.render();
  }

  // ──────────────────────────────────────────
  //  Ferramenta
  // ──────────────────────────────────────────

  setTool(tool: DungeonTool): void {
    this.tool = tool;
  }

  // ──────────────────────────────────────────
  //  Modo de desenho
  // ──────────────────────────────────────────

  async enable(): Promise<void> {
    const map = this.mapService.getMap();
    if (!map || this.enabled || !this.config) return;

    this.enabled = true;
    await this.show();
    this.zone.runOutsideAngular(() => this.bindPointerEvents(map));
  }

  disable(): void {
    if (!this.enabled) return;
    this.enabled = false;
    this.painting = false;
    this.lastKey = null;
    this.flushSave();
    for (const un of this.unregisters) un();
    this.unregisters = [];
    this.restoreInteractions();
  }

  // ──────────────────────────────────────────
  //  Visibilidade da camada
  // ──────────────────────────────────────────

  async toggle(): Promise<void> {
    if (this.visible) {
      this.hide();
    } else {
      await this.show();
    }
  }

  async show(): Promise<void> {
    const layer = await this.getOrCreateLayer();
    const map = this.mapService.getMap();
    if (!map) return;
    if (map.getLayers().getArray().indexOf(layer) === -1) {
      map.addLayer(layer);
    }
    this.visible = true;
  }

  hide(): void {
    const map = this.mapService.getMap();
    if (map && this.layer) {
      map.removeLayer(this.layer);
    }
    this.visible = false;
  }

  // ──────────────────────────────────────────
  //  Edição
  // ──────────────────────────────────────────

  /** Apaga todas as células do mapa atual. */
  clear(): void {
    this.tiles = {};
    this.featuresByKey.clear();
    this.layer?.getSource()?.clear();
    this.scheduleSave();
  }

  // ──────────────────────────────────────────
  //  Ciclo de vida
  // ──────────────────────────────────────────

  destroy(): void {
    this.disable();
    this.flushSave();
    this.destroyLayer();
    this.config = null;
    this.tiles = {};
    this.featuresByKey.clear();
    this.visible = false;
  }

  // ──────────────────────────────────────────
  //  Interno – OpenLayers
  // ──────────────────────────────────────────

  private async loadOl(): Promise<void> {
    if (this.OL) return;
    const [
      Feature,
      Polygon,
      Style,
      Fill,
      Stroke,
      VectorLayer,
      VectorSource,
      DragPan,
    ] = await Promise.all([
      import('ol/Feature').then((m) => m.default),
      import('ol/geom/Polygon').then((m) => m.default),
      import('ol/style/Style').then((m) => m.default),
      import('ol/style/Fill').then((m) => m.default),
      import('ol/style/Stroke').then((m) => m.default),
      import('ol/layer/Vector').then((m) => m.default),
      import('ol/source/Vector').then((m) => m.default),
      import('ol/interaction/DragPan').then((m) => m.default),
    ]);
    this.OL = {
      Feature,
      Polygon,
      Style,
      Fill,
      Stroke,
      VectorLayer,
      VectorSource,
      DragPan,
    };
  }

  private async getOrCreateLayer(): Promise<OlVectorLayer> {
    if (this.layer) return this.layer;
    if (!this.OL) await this.loadOl();
    const OL = this.OL!;

    const source = new OL.VectorSource();
    const layer = new OL.VectorLayer({
      source,
      style: (feature) => {
        const type = (feature.get('type') as DungeonTileType) ?? 'floor';
        const style = TILE_STYLES[type] ?? TILE_STYLES.floor;
        return new OL.Style({
          fill: new OL.Fill({ color: style.fill }),
          stroke: new OL.Stroke({ color: style.stroke, width: 1 }),
        });
      },
      zIndex: 50,
    });

    this.layer = layer;
    await this.render();
    return layer;
  }

  private destroyLayer(): void {
    const map = this.mapService.getMap();
    if (map && this.layer) {
      map.removeLayer(this.layer);
    }
    this.layer = null;
    this.featuresByKey.clear();
  }

  private async render(): Promise<void> {
    const layer = await this.getOrCreateLayer();
    const source = layer.getSource();
    if (!source) return;

    source.clear();
    this.featuresByKey.clear();

    if (!this.OL || !this.config) return;

    for (const [key, type] of Object.entries(this.tiles)) {
      const feature = this.createTileFeature(key, type);
      if (feature) {
        source.addFeature(feature);
        this.featuresByKey.set(key, feature);
      }
    }
  }

  private createTileFeature(key: string, type: DungeonTileType): OlFeature | null {
    if (!this.OL || !this.config) return null;
    const [col, row] = key.split(',').map(Number);
    if (!Number.isFinite(col) || !Number.isFinite(row)) return null;

    const { cellSize } = this.config;
    const x0 = col * cellSize;
    const y0 = row * cellSize;
    const x1 = x0 + cellSize;
    const y1 = y0 + cellSize;

    const feature = new this.OL.Feature({
      geometry: new this.OL.Polygon([
        [
          [x0, y0],
          [x1, y0],
          [x1, y1],
          [x0, y1],
          [x0, y0],
        ],
      ]),
    });
    feature.set('type', type);
    return feature;
  }

  private bindPointerEvents(map: OlMap): void {
    this.disableDefaultInteractions();

    const viewport = map.getViewport();
    const toPixel = (evt: PointerEvent): number[] => {
      const rect = viewport.getBoundingClientRect();
      return [evt.clientX - rect.left, evt.clientY - rect.top];
    };

    const onDown = (evt: PointerEvent) => {
      if (evt.button !== 0) return;
      evt.preventDefault();
      this.painting = true;
      this.lastKey = null;
      this.paintAt(toPixel(evt));
    };

    const onMove = (evt: PointerEvent) => {
      if (!this.painting) return;
      this.paintAt(toPixel(evt));
    };

    const onUp = () => {
      if (!this.painting) return;
      this.painting = false;
      this.lastKey = null;
      this.flushSave();
    };

    viewport.addEventListener('pointerdown', onDown);
    viewport.addEventListener('pointermove', onMove);
    window.addEventListener('pointerup', onUp);

    this.unregisters.push(
      () => viewport.removeEventListener('pointerdown', onDown),
      () => viewport.removeEventListener('pointermove', onMove),
      () => window.removeEventListener('pointerup', onUp),
    );
  }

  private paintAt(pixel: number[]): void {
    const map = this.mapService.getMap();
    if (!map || !this.OL || !this.config || !this.layer) return;

    const coord = map.getCoordinateFromPixel(pixel);
    const { cellSize, columns, rows } = this.config;
    const col = Math.floor(coord[0] / cellSize);
    const row = Math.floor(coord[1] / cellSize);

    if (col < 0 || row < 0 || col >= columns || row >= rows) return;

    const key = `${col},${row}`;
    if (key === this.lastKey) return;
    this.lastKey = key;

    const source = this.layer.getSource();
    if (!source) return;

    if (this.tool === 'erase') {
      if (!this.tiles[key]) return;
      delete this.tiles[key];
      const feature = this.featuresByKey.get(key);
      if (feature) {
        source.removeFeature(feature);
        this.featuresByKey.delete(key);
      }
      this.scheduleSave();
      return;
    }

    this.tiles[key] = this.tool;

    const existing = this.featuresByKey.get(key);
    if (existing) {
      existing.set('type', this.tool);
      existing.changed();
    } else {
      const feature = this.createTileFeature(key, this.tool);
      if (feature) {
        source.addFeature(feature);
        this.featuresByKey.set(key, feature);
      }
    }
    this.scheduleSave();
  }

  // ──────────────────────────────────────────
  //  Interno – persistência / interações
  // ──────────────────────────────────────────

  private scheduleSave(): void {
    if (this.saveTimer !== null) clearTimeout(this.saveTimer);
    this.saveTimer = setTimeout(() => {
      this.saveTimer = null;
      this.flushSave();
    }, 500);
  }

  private flushSave(): void {
    if (this.saveTimer !== null) {
      clearTimeout(this.saveTimer);
      this.saveTimer = null;
    }
    const mapId = this.config?.mapId;
    if (!mapId) return;
    this.store.patch('maps', mapId, {
      dungeon: { ...this.tiles },
      updatedAt: new Date(),
    });
  }

  private savedInteractions: { active: boolean; interaction: { setActive(v: boolean): void } }[] = [];

  private disableDefaultInteractions(): void {
    const map = this.mapService.getMap();
    if (!map || !this.OL) return;
    this.savedInteractions = [];
    const interactions = map.getInteractions().getArray();
    for (const interaction of interactions) {
      this.savedInteractions.push({
        interaction,
        active: interaction.getActive(),
      });
      if (interaction instanceof this.OL.DragPan) {
        interaction.setActive(false);
      }
    }
  }

  private restoreInteractions(): void {
    for (const { interaction, active } of this.savedInteractions) {
      interaction.setActive(active);
    }
    this.savedInteractions = [];
  }
}
