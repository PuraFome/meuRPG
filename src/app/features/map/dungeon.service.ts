import { Injectable, inject, NgZone } from '@angular/core';
import { MapService } from './map.service';
import { StoreService } from '../../core/store/store.service';
import type {
  DungeonData,
  DungeonObject,
  DungeonObjects,
  DungeonObjectType,
  DungeonTileType,
  DungeonTiles,
  MapData,
} from '../../core/models/map';

type OlMap = import('ol/Map').default;
type OlVectorLayer = import('ol/layer/Vector').default;
type OlVectorSource = import('ol/source/Vector').default;
type OlFeature = import('ol/Feature').default;

export type DungeonBrushTool =
  | 'floor'
  | 'water'
  | 'difficult'
  | 'rubble'
  | 'wall'
  | 'false_wall';

export type DungeonStampTool =
  | 'door'
  | 'secret_door'
  | 'trap'
  | 'chest'
  | 'mimic'
  | 'character';

/** Ferramenta ativa no editor de masmorra. `erase` remove a célula. */
export type DungeonTool = DungeonBrushTool | DungeonStampTool | 'erase';

const BRUSH_TOOLS: ReadonlySet<string> = new Set<DungeonBrushTool>([
  'floor',
  'water',
  'difficult',
  'rubble',
  'wall',
  'false_wall',
]);

/** Brushes pintam células durante o arrasto; carimbos são um clique único. */
export function isBrushTool(tool: DungeonTool): boolean {
  return BRUSH_TOOLS.has(tool);
}

export interface DungeonConfig {
  mapId: string;
  /** Extensão em pixels da imagem: [minX, minY, maxX, maxY]. */
  extent: [number, number, number, number];
  cellSize: number;
  columns: number;
  rows: number;
}

export interface DungeonCharacter {
  id: string;
  name: string;
  color: string;
}

interface TileStyle {
  fill: string;
  stroke: string;
  width?: number;
  dashed?: boolean;
}

/** Paleta de terreno/estrutura — distinta e legível sobre a imagem de fundo. */
const TILE_STYLES: Record<DungeonTileType, TileStyle> = {
  floor: { fill: 'rgba(214, 184, 138, 0.55)', stroke: 'rgba(120, 88, 48, 0.75)' },
  water: { fill: 'rgba(33, 150, 243, 0.55)', stroke: 'rgba(13, 71, 161, 0.85)' },
  difficult: { fill: 'rgba(76, 175, 80, 0.5)', stroke: 'rgba(27, 94, 32, 0.8)' },
  rubble: { fill: 'rgba(158, 158, 158, 0.55)', stroke: 'rgba(66, 66, 66, 0.85)' },
  wall: { fill: 'rgba(58, 58, 68, 0.95)', stroke: 'rgba(16, 16, 22, 1)', width: 2 },
  false_wall: {
    fill: 'rgba(58, 58, 68, 0.95)',
    stroke: 'rgba(255, 179, 0, 0.95)',
    width: 2,
    dashed: true,
  },
};

/** Glifo de cada elemento posicionado. */
const OBJECT_EMOJI: Record<DungeonObjectType, string> = {
  door: '🚪',
  secret_door: '🕳️',
  trap: '⚠️',
  chest: '💰',
  mimic: '👹',
  character: '🧙',
};

/**
 * Editor de masmorra baseado em grade com ferramentas de desenho.
 *
 * Brushes (piso, água, terreno difícil, escombros, parede, parede falsa) pintam
 * células arrastando. Carimbos (porta, porta secreta, armadilha, baú, mímico e
 * personagens) colocam um elemento na célula com um clique. Terreno e elementos
 * ficam em mapas esparsos separados, persistidos em `MapData.dungeon`.
 */
@Injectable({ providedIn: 'root' })
export class DungeonService {
  private readonly zone = inject(NgZone);
  private readonly mapService = inject(MapService);
  private readonly store = inject(StoreService<MapData>);

  private config: DungeonConfig | null = null;
  private tiles: DungeonTiles = {};
  private objects: DungeonObjects = {};
  private tool: DungeonTool = 'floor';
  private character: DungeonCharacter | null = null;

  private tileLayer: OlVectorLayer | null = null;
  private objectLayer: OlVectorLayer | null = null;
  private tileFeatures = new Map<string, OlFeature>();
  private objectFeatures = new Map<string, OlFeature>();
  private OL: {
    Feature: typeof import('ol/Feature').default;
    Polygon: typeof import('ol/geom/Polygon').default;
    Point: typeof import('ol/geom/Point').default;
    Style: typeof import('ol/style/Style').default;
    Fill: typeof import('ol/style/Fill').default;
    Stroke: typeof import('ol/style/Stroke').default;
    Circle: typeof import('ol/style/Circle').default;
    Text: typeof import('ol/style/Text').default;
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
  private savedInteractions: { active: boolean; interaction: { setActive(v: boolean): void } }[] = [];

  // ──────────────────────────────────────────
  //  Configuração / carga
  // ──────────────────────────────────────────

  /** (Re)configura o editor para o mapa atual. */
  async configure(config: DungeonConfig): Promise<void> {
    this.disable();
    this.destroyLayers();
    this.config = config;
    await this.loadOl();
    if (this.visible) {
      await this.show();
    }
  }

  /** Carrega os dados salvos (aceita o formato antigo de tiles simples). */
  async load(data: DungeonData | DungeonTiles | undefined): Promise<void> {
    const normalized = normalizeDungeon(data);
    this.tiles = normalized.tiles;
    this.objects = normalized.objects;
    await this.render();
  }

  // ──────────────────────────────────────────
  //  Ferramenta
  // ──────────────────────────────────────────

  setTool(tool: DungeonTool): void {
    this.tool = tool;
  }

  setCharacter(character: DungeonCharacter | null): void {
    this.character = character;
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
  //  Visibilidade
  // ──────────────────────────────────────────

  async toggle(): Promise<void> {
    if (this.visible) {
      this.hide();
    } else {
      await this.show();
    }
  }

  async show(): Promise<void> {
    const { tileLayer, objectLayer } = await this.getOrCreateLayers();
    const map = this.mapService.getMap();
    if (!map) return;
    for (const layer of [tileLayer, objectLayer]) {
      if (map.getLayers().getArray().indexOf(layer) === -1) {
        map.addLayer(layer);
      }
    }
    this.visible = true;
  }

  hide(): void {
    const map = this.mapService.getMap();
    if (map) {
      if (this.tileLayer) map.removeLayer(this.tileLayer);
      if (this.objectLayer) map.removeLayer(this.objectLayer);
    }
    this.visible = false;
  }

  // ──────────────────────────────────────────
  //  Edição
  // ──────────────────────────────────────────

  /** Apaga todo o desenho do mapa atual. */
  clear(): void {
    this.tiles = {};
    this.objects = {};
    this.tileFeatures.clear();
    this.objectFeatures.clear();
    this.tileLayer?.getSource()?.clear();
    this.objectLayer?.getSource()?.clear();
    this.scheduleSave();
  }

  // ──────────────────────────────────────────
  //  Ciclo de vida
  // ──────────────────────────────────────────

  destroy(): void {
    this.disable();
    this.flushSave();
    this.destroyLayers();
    this.config = null;
    this.tiles = {};
    this.objects = {};
    this.tileFeatures.clear();
    this.objectFeatures.clear();
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
      Point,
      Style,
      Fill,
      Stroke,
      Circle,
      Text,
      VectorLayer,
      VectorSource,
      DragPan,
    ] = await Promise.all([
      import('ol/Feature').then((m) => m.default),
      import('ol/geom/Polygon').then((m) => m.default),
      import('ol/geom/Point').then((m) => m.default),
      import('ol/style/Style').then((m) => m.default),
      import('ol/style/Fill').then((m) => m.default),
      import('ol/style/Stroke').then((m) => m.default),
      import('ol/style/Circle').then((m) => m.default),
      import('ol/style/Text').then((m) => m.default),
      import('ol/layer/Vector').then((m) => m.default),
      import('ol/source/Vector').then((m) => m.default),
      import('ol/interaction/DragPan').then((m) => m.default),
    ]);
    this.OL = {
      Feature,
      Polygon,
      Point,
      Style,
      Fill,
      Stroke,
      Circle,
      Text,
      VectorLayer,
      VectorSource,
      DragPan,
    };
  }

  private async getOrCreateLayers(): Promise<{
    tileLayer: OlVectorLayer;
    objectLayer: OlVectorLayer;
  }> {
    if (this.tileLayer && this.objectLayer) {
      return { tileLayer: this.tileLayer, objectLayer: this.objectLayer };
    }
    if (!this.OL) await this.loadOl();
    const OL = this.OL!;

    const tileSource = new OL.VectorSource();
    this.tileLayer = new OL.VectorLayer({
      source: tileSource,
      style: (feature) => {
        const type = (feature.get('tile') as DungeonTileType) ?? 'floor';
        const s = TILE_STYLES[type] ?? TILE_STYLES.floor;
        return new OL.Style({
          fill: new OL.Fill({ color: s.fill }),
          stroke: new OL.Stroke({
            color: s.stroke,
            width: s.width ?? 1,
            lineDash: s.dashed ? [5, 4] : undefined,
          }),
        });
      },
      zIndex: 50,
    });

    const objectSource = new OL.VectorSource();
    this.objectLayer = new OL.VectorLayer({
      source: objectSource,
      style: (feature) => this.objectStyle(feature.get.bind(feature)),
      zIndex: 60,
    });

    await this.render();
    return { tileLayer: this.tileLayer, objectLayer: this.objectLayer };
  }

  private objectStyle(
    get: (name: string) => unknown,
  ): import('ol/style/Style').default | import('ol/style/Style').default[] {
    const OL = this.OL!;
    const obj = get('obj') as DungeonObject | undefined;
    if (!obj) return new OL.Style({});

    if (obj.type === 'character') {
      const color = obj.color ?? '#7c4dff';
      const initial = (obj.label ?? '?').charAt(0).toUpperCase();
      return [
        new OL.Style({
          image: new OL.Circle({
            radius: 12,
            fill: new OL.Fill({ color }),
            stroke: new OL.Stroke({ color: '#ffffff', width: 2 }),
          }),
        }),
        new OL.Style({
          text: new OL.Text({
            text: initial,
            font: 'bold 12px Roboto, sans-serif',
            fill: new OL.Fill({ color: '#ffffff' }),
          }),
        }),
        new OL.Style({
          text: new OL.Text({
            text: obj.label ?? '',
            font: 'bold 11px Roboto, sans-serif',
            fill: new OL.Fill({ color: '#ffffff' }),
            stroke: new OL.Stroke({ color: '#000000', width: 3 }),
            offsetY: -22,
          }),
        }),
      ];
    }

    return new OL.Style({
      text: new OL.Text({
        text: obj.icon ?? OBJECT_EMOJI[obj.type],
        font: '16px sans-serif',
        textAlign: 'center',
      }),
    });
  }

  private destroyLayers(): void {
    const map = this.mapService.getMap();
    if (map) {
      if (this.tileLayer) map.removeLayer(this.tileLayer);
      if (this.objectLayer) map.removeLayer(this.objectLayer);
    }
    this.tileLayer = null;
    this.objectLayer = null;
    this.tileFeatures.clear();
    this.objectFeatures.clear();
  }

  private async render(): Promise<void> {
    const { tileLayer, objectLayer } = await this.getOrCreateLayers();
    const tileSource = tileLayer.getSource();
    const objectSource = objectLayer.getSource();
    if (!tileSource || !objectSource) return;

    tileSource.clear();
    objectSource.clear();
    this.tileFeatures.clear();
    this.objectFeatures.clear();

    if (!this.OL || !this.config) return;

    for (const [key, type] of Object.entries(this.tiles)) {
      const feature = this.createTileFeature(key, type);
      if (feature) {
        tileSource.addFeature(feature);
        this.tileFeatures.set(key, feature);
      }
    }
    for (const [key, obj] of Object.entries(this.objects)) {
      const feature = this.createObjectFeature(key, obj);
      if (feature) {
        objectSource.addFeature(feature);
        this.objectFeatures.set(key, feature);
      }
    }
  }

  private cellCenter(key: string): [number, number] | null {
    if (!this.config) return null;
    const [col, row] = key.split(',').map(Number);
    if (!Number.isFinite(col) || !Number.isFinite(row)) return null;
    const { cellSize } = this.config;
    return [col * cellSize + cellSize / 2, row * cellSize + cellSize / 2];
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
    feature.set('tile', type);
    return feature;
  }

  private createObjectFeature(key: string, obj: DungeonObject): OlFeature | null {
    if (!this.OL) return null;
    const center = this.cellCenter(key);
    if (!center) return null;
    const feature = new this.OL.Feature({ geometry: new this.OL.Point(center) });
    feature.set('obj', obj);
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
      this.paintAt(toPixel(evt), false);
    };

    const onMove = (evt: PointerEvent) => {
      if (!this.painting) return;
      this.paintAt(toPixel(evt), true);
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

  private paintAt(pixel: number[], isDrag: boolean): void {
    const map = this.mapService.getMap();
    if (!map || !this.OL || !this.config || !this.tileLayer) return;
    if (isDrag && !isBrushTool(this.tool) && this.tool !== 'erase') return;

    const coord = map.getCoordinateFromPixel(pixel);
    const { cellSize, columns, rows } = this.config;
    const col = Math.floor(coord[0] / cellSize);
    const row = Math.floor(coord[1] / cellSize);

    if (col < 0 || row < 0 || col >= columns || row >= rows) return;

    const key = `${col},${row}`;
    if (key === this.lastKey) return;
    this.lastKey = key;

    if (this.tool === 'erase') {
      this.eraseAt(key);
    } else if (isBrushTool(this.tool)) {
      this.setTile(key, this.tool as DungeonTileType);
    } else {
      this.setObject(key, this.tool as DungeonObjectType);
    }
    this.scheduleSave();
  }

  private setTile(key: string, type: DungeonTileType): void {
    const source = this.tileLayer?.getSource();
    if (!source) return;

    this.tiles[key] = type;
    const existing = this.tileFeatures.get(key);
    if (existing) {
      existing.set('tile', type);
      existing.changed();
      return;
    }
    const feature = this.createTileFeature(key, type);
    if (feature) {
      source.addFeature(feature);
      this.tileFeatures.set(key, feature);
    }
  }

  private setObject(key: string, type: DungeonObjectType): void {
    const source = this.objectLayer?.getSource();
    if (!source) return;

    const obj: DungeonObject = { type };
    if (type === 'character') {
      if (!this.character) return;
      obj.characterId = this.character.id;
      obj.label = this.character.name;
      obj.color = this.character.color;
    }
    this.objects[key] = obj;

    const existing = this.objectFeatures.get(key);
    if (existing) {
      existing.set('obj', obj);
      existing.changed();
      return;
    }
    const feature = this.createObjectFeature(key, obj);
    if (feature) {
      source.addFeature(feature);
      this.objectFeatures.set(key, feature);
    }
  }

  private eraseAt(key: string): void {
    const object = this.objectFeatures.get(key);
    if (object) {
      this.objectLayer?.getSource()?.removeFeature(object);
      this.objectFeatures.delete(key);
      delete this.objects[key];
      return;
    }
    const tile = this.tileFeatures.get(key);
    if (tile) {
      this.tileLayer?.getSource()?.removeFeature(tile);
      this.tileFeatures.delete(key);
      delete this.tiles[key];
    }
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
      dungeon: { tiles: { ...this.tiles }, objects: { ...this.objects } },
      updatedAt: new Date(),
    });
  }

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

/** Aceita tanto o formato atual ({tiles, objects}) quanto o antigo (tiles simples). */
function normalizeDungeon(data: DungeonData | DungeonTiles | undefined): DungeonData {
  if (!data) {
    return { tiles: {}, objects: {} };
  }
  if ('tiles' in data || 'objects' in data) {
    const d = data as DungeonData;
    return { tiles: { ...(d.tiles ?? {}) }, objects: { ...(d.objects ?? {}) } };
  }
  return { tiles: { ...(data as DungeonTiles) }, objects: {} };
}
