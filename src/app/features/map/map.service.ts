import { Injectable, inject, NgZone } from '@angular/core';
import { StoreService } from '../../core/store/store.service';
import type { MapData, MapMarker, SubmapPin } from '../../core/models/map';

type OlMap = import('ol/Map').default;
type OlView = import('ol/View').default;
type OlVectorLayer = import('ol/layer/Vector').default;
type OlVectorSource = import('ol/source/Vector').default;
type OlImageLayer = import('ol/layer/Image').default<import('ol/source/ImageStatic').default>;
type OlImageStatic = import('ol/source/ImageStatic').default;
type OlFeature = import('ol/Feature').default;

export interface MapInitOptions {
  zoom?: number;
  center?: [number, number];
}

export interface MapClickResult {
  type: 'empty' | 'poi' | 'submap';
  coords?: [number, number];
  marker?: MapMarker;
  targetMapId?: string;
}

// Constructor types used in feature factories
type FeatureCtor = typeof import('ol/Feature').default;
type PolygonCtor = typeof import('ol/geom/Polygon').default;
type LineStringCtor = typeof import('ol/geom/LineString').default;

@Injectable({ providedIn: 'root' })
export class MapService {
  private map: OlMap | null = null;
  private initPromise: Promise<OlMap> | null = null;
  private readonly zone = inject(NgZone);
  private readonly store = inject(StoreService<MapData>);

  private currentMapId: string | null = null;

  // Base layers
  private osmLayer: import('ol/layer/Tile').default | null = null;
  private imageLayer: OlImageLayer | null = null;
  private imageExtent: [number, number, number, number] | null = null;
  private imageMode = false;

  // Vector layer references
  private hexGridLayer: OlVectorLayer | null = null;
  private squareGridLayer: OlVectorLayer | null = null;
  private fogLayer: OlVectorLayer | null = null;
  private markersLayer: OlVectorLayer | null = null;
  private poisLayer: OlVectorLayer | null = null;
  private submapPinsLayer: OlVectorLayer | null = null;

  // Visibility state
  private hexGridVisible = false;
  private squareGridVisible = false;
  private fogVisible = false;
  private markersVisible = false;
  private poisVisible = false;
  private submapPinsVisible = false;

  /**
   * Initialize the OpenLayers map inside the given container element.
   * OpenLayers modules are dynamically imported — never in the initial bundle.
   */
  async initialize(
    container: HTMLElement,
    options?: MapInitOptions,
  ): Promise<OlMap> {
    if (this.map) return this.map;
    if (this.initPromise) return this.initPromise;

    this.initPromise = this.zone.runOutsideAngular(async () => {
      const [OLMap, OLView, OLTileLayer, OSM] = await Promise.all([
        import('ol/Map').then((m) => m.default),
        import('ol/View').then((m) => m.default),
        import('ol/layer/Tile').then((m) => m.default),
        import('ol/source/OSM').then((m) => m.default),
      ]);

      const osmLayer = new OLTileLayer({
        source: new OSM(),
      });
      this.osmLayer = osmLayer;

      const map = new OLMap({
        target: container,
        layers: [osmLayer],
        view: new OLView({
          center: options?.center ?? [-46.6333, -23.5505],
          zoom: options?.zoom ?? 10,
        }),
      });

      this.map = map;
      return map;
    });

    return this.initPromise;
  }

  /** Returns the current OL Map instance, or null if not initialized. */
  getMap(): OlMap | null {
    return this.map;
  }

  /** Set the current zoom level. */
  zoomTo(zoom: number): void {
    this.map?.getView().setZoom(zoom);
  }

  /** Pan the map to a center coordinate. */
  centerOn(center: [number, number]): void {
    this.map?.getView().setCenter(center);
  }

  /** Toggle fullscreen on the given element. */
  toggleFullscreen(element: HTMLElement): void {
    if (!document.fullscreenElement) {
      element.requestFullscreen().catch(() => {});
    } else {
      document.exitFullscreen().catch(() => {});
    }
  }

  /** Capture the map's rendered canvas for use as a Three.js texture. */
  getCanvas(): HTMLCanvasElement | null {
    if (!this.map) return null;
    return this.map.getViewport().querySelector('canvas') ?? null;
  }

  /** Returns the current view centre as [lon, lat], or a default. */
  getCenter(): [number, number] {
    if (!this.map) return [-46.6333, -23.5505];
    const c = this.map.getView().getCenter();
    return c ? [c[0], c[1]] : [-46.6333, -23.5505];
  }

  /** Returns the current zoom level, or a default. */
  getZoom(): number {
    if (!this.map) return 10;
    return this.map.getView().getZoom() ?? 10;
  }

  onMapClick(handler: (coords: [number, number]) => void): () => void {
    if (!this.map) return () => {};

    const key = this.map.on('singleclick', (evt) => {
      const hit = this.getHitResult(evt.pixel);
      if (hit) return;
      void this.toMapCoords(evt.coordinate as [number, number]).then(handler);
    });

    return () => this.map?.un('singleclick', key.listener);
  }

  /** Detect clicks on POI/submap features, returning an enriched result. */
  onFeatureClick(handler: (result: MapClickResult) => void): () => void {
    if (!this.map) return () => {};

    const key = this.map.on('singleclick', (evt) => {
      const hit = this.getHitResult(evt.pixel);
      if (hit) handler(hit);
    });

    return () => this.map?.un('singleclick', key.listener);
  }

  private getHitResult(pixel: number[]): MapClickResult | null {
    if (!this.map) return null;

    const feature = this.map.forEachFeatureAtPixel(
      pixel,
      (f) => f as OlFeature,
      { hitTolerance: 6 },
    );
    if (!feature) return null;

    const poiId = feature.get('poiId');
    if (poiId && this.currentMapId) {
      const map = this.getMapById(this.currentMapId);
      const marker = map?.markers.find((m) => m.id === poiId);
      if (marker) {
        return { type: 'poi', marker };
      }
    }

    const targetMapId = feature.get('targetMapId');
    if (targetMapId) {
      return { type: 'submap', targetMapId };
    }

    return null;
  }

  /** Destroy the map and release resources. */
  destroy(): void {
    if (this.submapPinsLayer) {
      this.map?.removeLayer(this.submapPinsLayer);
      this.submapPinsLayer = null;
    }
    if (this.poisLayer) {
      this.map?.removeLayer(this.poisLayer);
      this.poisLayer = null;
    }
    if (this.imageLayer) {
      this.map?.removeLayer(this.imageLayer);
      this.imageLayer = null;
    }
    if (this.map) {
      this.map.setTarget(undefined);
      this.map = null;
      this.initPromise = null;
    }
    this.osmLayer = null;
    this.imageExtent = null;
    this.imageMode = false;
  }

  // ──────────────────────────────────────────
  //  Image background (RPG maps)
  // ──────────────────────────────────────────

  /** True when the current map renders a custom image instead of OSM tiles. */
  isImageMap(): boolean {
    return this.imageMode;
  }

  /** Returns the current image extent, or null when in OSM mode. */
  getImageExtent(): [number, number, number, number] | null {
    return this.imageExtent;
  }

  /**
   * Replace the OSM base layer with a static image map (pixel space:
   * x in [0, width], y in [0, height]). Pins placed afterwards are stored
   * in image-pixel coordinates.
   */
  async setImageBackground(dataUrl: string, width: number, height: number): Promise<void> {
    if (!this.map) return;

    await this.zone.runOutsideAngular(async () => {
      const [OLImageLayer, OLImageStatic, OLView, OLProjection] = await Promise.all([
        import('ol/layer/Image').then((m) => m.default),
        import('ol/source/ImageStatic').then((m) => m.default),
        import('ol/View').then((m) => m.default),
        import('ol/proj/Projection').then((m) => m.default),
      ]);

      const extent: [number, number, number, number] = [0, 0, width, height];

      // Pixel-units projection: view coordinates == image pixel coordinates.
      const projection = new OLProjection({
        code: 'image-pixels',
        units: 'pixels',
        extent,
      });

      if (this.osmLayer) {
        this.map?.removeLayer(this.osmLayer);
      }
      if (this.imageLayer) {
        this.map?.removeLayer(this.imageLayer);
      }

      const imageLayer = new OLImageLayer({
        source: new OLImageStatic({
          url: dataUrl,
          imageExtent: extent,
          projection,
        }),
      });
      this.map?.addLayer(imageLayer);
      this.imageLayer = imageLayer;
      this.imageExtent = extent;
      this.imageMode = true;

      const view = new OLView({
        projection,
        center: [width / 2, height / 2],
        zoom: 0,
        minZoom: -3,
        maxZoom: 8,
      });
      view.fit(extent, { padding: [24, 24, 24, 24] });
      this.map?.setView(view);
    });
  }

  /** Restore the OSM base layer and a geographic view, discarding the custom image. */
  async clearImageBackground(): Promise<void> {
    if (this.imageLayer && this.map) {
      this.map.removeLayer(this.imageLayer);
      this.imageLayer = null;
    }
    this.imageExtent = null;
    this.imageMode = false;

    if (!this.map) return;

    if (this.osmLayer && this.map.getLayers().getArray().indexOf(this.osmLayer) === -1) {
      this.map.addLayer(this.osmLayer);
    }

    const current = this.map.getView();
    if (current.getProjection().getCode() !== 'EPSG:3857') {
      const OLView = await import('ol/View').then((m) => m.default);
      this.map.setView(
        new OLView({
          center: [-46.6333, -23.5505],
          zoom: 10,
        }),
      );
    }
  }

  /** Convert an OpenLayers view coordinate to map space (image pixels or lon/lat). */
  async toMapCoords(coordinate: [number, number]): Promise<[number, number]> {
    if (this.imageMode) {
      return [coordinate[0], coordinate[1]];
    }
    const proj = await import('ol/proj');
    const lonLat = proj.toLonLat(coordinate);
    return [lonLat[0], lonLat[1]];
  }

  /** Project a stored pin coordinate (image px or lon/lat) into the OL view. */
  async project(coordinate: [number, number]): Promise<number[]> {
    if (this.imageMode) {
      return [coordinate[0], coordinate[1]];
    }
    const proj = await import('ol/proj');
    return proj.fromLonLat(coordinate);
  }

  // ──────────────────────────────────────────
  //  Layer toggles
  // ──────────────────────────────────────────

  /** Toggle hex or square grid visibility. */
  async toggleGrid(type: 'hex' | 'square'): Promise<void> {
    if (!this.map) return;

    if (type === 'hex') {
      this.hexGridVisible = !this.hexGridVisible;
      if (this.hexGridVisible) {
        this.map.addLayer(await this.getOrCreateHexGridLayer());
      } else if (this.hexGridLayer) {
        this.map.removeLayer(this.hexGridLayer);
      }
    } else {
      this.squareGridVisible = !this.squareGridVisible;
      if (this.squareGridVisible) {
        this.map.addLayer(await this.getOrCreateSquareGridLayer());
      } else if (this.squareGridLayer) {
        this.map.removeLayer(this.squareGridLayer);
      }
    }
  }

  /** Toggle fog-of-war overlay. */
  async toggleFogOfWar(): Promise<void> {
    if (!this.map) return;

    this.fogVisible = !this.fogVisible;
    if (this.fogVisible) {
      this.map.addLayer(await this.getOrCreateFogLayer());
    } else if (this.fogLayer) {
      this.map.removeLayer(this.fogLayer);
    }
  }

  /** Toggle markers layer (Points of Interest). */
  async toggleMarkers(): Promise<void> {
    if (!this.map) return;

    this.poisVisible = !this.poisVisible;
    if (this.poisVisible) {
      const layer = await this.getOrCreatePoisLayer();
      if (this.map.getLayers().getArray().indexOf(layer) === -1) {
        this.map.addLayer(layer);
      }
    } else if (this.poisLayer) {
      this.map.removeLayer(this.poisLayer);
    }
  }

  /** Set the opacity of the fog-of-war layer (0–1). */
  setFogOpacity(value: number): void {
    if (this.fogLayer) {
      this.fogLayer.setOpacity(value);
    }
  }

  /** Remove and re-create the fog-of-war overlay (full reset). */
  async resetFogOfWar(): Promise<void> {
    if (!this.fogLayer || !this.map) return;

    this.map.removeLayer(this.fogLayer);
    this.fogLayer = null;

    if (this.fogVisible) {
      this.map.addLayer(await this.getOrCreateFogLayer());
    }
  }

  // ──────────────────────────────────────────
  //  Submap Pins
  // ──────────────────────────────────────────

  /** Set the current map context by id. */
  setCurrentMapId(id: string | null): void {
    this.currentMapId = id;
  }

  /** Get the current map id. */
  getCurrentMapId(): string | null {
    return this.currentMapId;
  }

  /** Get a map by id from the store. */
  getMapById(id: string): MapData | undefined {
    return this.store.snapshot('maps').find((m) => m.id === id);
  }

  /** Get all maps from the store. */
  getAllMaps(): MapData[] {
    return this.store.snapshot('maps');
  }

  /** Find which map has a submap pin or POI pointing to the given map id. */
  getParentMapId(mapId: string): string | null {
    const maps = this.store.snapshot('maps');
    for (const m of maps) {
      if (m.submaps?.some((p) => p.targetMapId === mapId)) {
        return m.id;
      }
      if (m.markers?.some((mk) => mk.targetMapId === mapId)) {
        return m.id;
      }
    }
    return null;
  }

  /** Build breadcrumb hierarchy for a map id. */
  getMapHierarchy(mapId: string): { id: string; name: string }[] {
    const chain: { id: string; name: string }[] = [];
    let currentId: string | null = mapId;
    while (currentId) {
      const map = this.getMapById(currentId);
      if (!map) break;
      chain.unshift({ id: map.id, name: map.name });
      currentId = this.getParentMapId(currentId);
    }
    return chain;
  }

  /** Add a submap pin to the current map's data and re-render. */
  async addSubmapPin(pin: SubmapPin): Promise<void> {
    if (!this.currentMapId) return;
    const map = this.getMapById(this.currentMapId);
    if (!map) return;

    const updated = {
      ...map,
      submaps: [...(map.submaps ?? []), pin],
      updatedAt: new Date(),
    };
    this.store.update('maps', this.currentMapId, updated);

    await this.renderSubmapPins(updated.submaps);
  }

  /** Remove a submap pin by id and re-render. */
  async removeSubmapPin(pinId: string): Promise<void> {
    if (!this.currentMapId) return;
    const map = this.getMapById(this.currentMapId);
    if (!map) return;

    const updated = {
      ...map,
      submaps: (map.submaps ?? []).filter((p) => p.id !== pinId),
      updatedAt: new Date(),
    };
    this.store.update('maps', this.currentMapId, updated);

    await this.renderSubmapPins(updated.submaps);
  }

  /** Get submap pins for the current map. */
  getSubmapPins(): SubmapPin[] {
    if (!this.currentMapId) return [];
    const map = this.getMapById(this.currentMapId);
    return map?.submaps ?? [];
  }

  /** Toggle submap pins layer visibility. */
  async toggleSubmapPins(): Promise<void> {
    if (!this.map) return;

    this.submapPinsVisible = !this.submapPinsVisible;
    if (this.submapPinsVisible) {
      const layer = await this.getOrCreateSubmapPinsLayer();
      if (this.map.getLayers().getArray().indexOf(layer) === -1) {
        this.map.addLayer(layer);
      }
    } else if (this.submapPinsLayer) {
      this.map.removeLayer(this.submapPinsLayer);
    }
  }

  /** Ensure submap pins layer is visible on the map without toggling state. */
  async showSubmapPinsLayer(): Promise<void> {
    if (!this.map) return;
    this.submapPinsVisible = true;
    const layer = await this.getOrCreateSubmapPinsLayer();
    if (this.map.getLayers().getArray().indexOf(layer) === -1) {
      this.map.addLayer(layer);
    }
  }

  /** Re-render submap pins from the given pin data. */
  async renderSubmapPins(pins: SubmapPin[]): Promise<void> {
    const layer = await this.getOrCreateSubmapPinsLayer();
    const source = layer.getSource();
    if (!source) return;

    source.clear();

    if (pins.length === 0) return;

    const [OLFeature, OLPoint, OLStyle, OLFill, OLStroke, OLCircle, OLText] =
      await Promise.all([
        import('ol/Feature').then((m) => m.default),
        import('ol/geom/Point').then((m) => m.default),
        import('ol/style/Style').then((m) => m.default),
        import('ol/style/Fill').then((m) => m.default),
        import('ol/style/Stroke').then((m) => m.default),
        import('ol/style/Circle').then((m) => m.default),
        import('ol/style/Text').then((m) => m.default),
      ]);

    const features = await Promise.all(
      pins.map(async (pin) => {
        const feature = new OLFeature({
          geometry: new OLPoint(await this.project([pin.x, pin.y])),
        });
        feature.set('id', pin.id);
        feature.set('label', pin.label);
        feature.set('targetMapId', pin.targetMapId);
        feature.set('color', pin.color ?? '#e53935');
        return feature;
      }),
    );

    source.addFeatures(features);
  }

  // ──────────────────────────────────────────
  //  Points of Interest (POIs)
  // ──────────────────────────────────────────

  /** Add a POI to the current map and re-render. */
  async addPoi(marker: MapMarker): Promise<void> {
    if (!this.currentMapId) return;
    const map = this.getMapById(this.currentMapId);
    if (!map) return;

    const updated = {
      ...map,
      markers: [...(map.markers ?? []), marker],
      updatedAt: new Date(),
    };
    this.store.update('maps', this.currentMapId, updated);

    await this.renderPois(updated.markers);
  }

  /** Replace a POI on the current map and re-render. */
  async updatePoi(marker: MapMarker): Promise<void> {
    if (!this.currentMapId) return;
    const map = this.getMapById(this.currentMapId);
    if (!map) return;

    const updated = {
      ...map,
      markers: (map.markers ?? []).map((m) =>
        m.id === marker.id ? marker : m,
      ),
      updatedAt: new Date(),
    };
    this.store.update('maps', this.currentMapId, updated);

    await this.renderPois(updated.markers);
  }

  /** Remove a POI from the current map and re-render. */
  async deletePoi(markerId: string): Promise<void> {
    if (!this.currentMapId) return;
    const map = this.getMapById(this.currentMapId);
    if (!map) return;

    const updated = {
      ...map,
      markers: (map.markers ?? []).filter((m) => m.id !== markerId),
      updatedAt: new Date(),
    };
    this.store.update('maps', this.currentMapId, updated);

    await this.renderPois(updated.markers);
  }

  /** Render the current map's POIs as labeled pins. */
  async renderPois(markers: MapMarker[]): Promise<void> {
    const layer = await this.getOrCreatePoisLayer();
    const source = layer.getSource();
    if (!source) return;

    source.clear();

    if (markers.length === 0) return;

    const [OLFeature, OLPoint, OLStyle, OLFill, OLStroke, OLCircle, OLText] =
      await Promise.all([
        import('ol/Feature').then((m) => m.default),
        import('ol/geom/Point').then((m) => m.default),
        import('ol/style/Style').then((m) => m.default),
        import('ol/style/Fill').then((m) => m.default),
        import('ol/style/Stroke').then((m) => m.default),
        import('ol/style/Circle').then((m) => m.default),
        import('ol/style/Text').then((m) => m.default),
      ]);

    const features = await Promise.all(
      markers.map(async (marker) => {
        const feature = new OLFeature({
          geometry: new OLPoint(await this.project([marker.x, marker.y])),
        });
        feature.set('poiId', marker.id);
        feature.set('label', marker.label);
        feature.set('color', marker.color ?? '#7c4dff');
        feature.set('icon', marker.icon ?? 'place');
        return feature;
      }),
    );

    source.addFeatures(features);
  }

  /** Ensure the POI layer is visible. */
  async showPoisLayer(): Promise<void> {
    if (!this.map) return;
    this.poisVisible = true;
    const layer = await this.getOrCreatePoisLayer();
    if (this.map.getLayers().getArray().indexOf(layer) === -1) {
      this.map.addLayer(layer);
    }
  }

  // ──────────────────────────────────────────
  //  Private – lazy layer creation
  // ──────────────────────────────────────────

  private async getOrCreateHexGridLayer(): Promise<OlVectorLayer> {
    if (this.hexGridLayer) return this.hexGridLayer;

    const [
      OLVectorLayer,
      OLVectorSource,
      OLStyle,
      OLStroke,
      OLFill,
      OLFeature,
      OLPolygon,
    ] = await Promise.all([
      import('ol/layer/Vector').then((m) => m.default),
      import('ol/source/Vector').then((m) => m.default),
      import('ol/style/Style').then((m) => m.default),
      import('ol/style/Stroke').then((m) => m.default),
      import('ol/style/Fill').then((m) => m.default),
      import('ol/Feature').then((m) => m.default),
      import('ol/geom/Polygon').then((m) => m.default),
    ]);

    const source = new OLVectorSource();
    const layer = new OLVectorLayer({
      source,
      style: new OLStyle({
        stroke: new OLStroke({ color: 'rgba(255,255,255,0.3)', width: 1 }),
        fill: new OLFill({ color: 'rgba(255,255,255,0.05)' }),
      }),
    });

    source.addFeatures(this.createHexGrid(OLFeature, OLPolygon));
    this.hexGridLayer = layer;
    return layer;
  }

  private async getOrCreateSquareGridLayer(): Promise<OlVectorLayer> {
    if (this.squareGridLayer) return this.squareGridLayer;

    const [
      OLVectorLayer,
      OLVectorSource,
      OLStyle,
      OLStroke,
      OLFeature,
      OLLineString,
    ] = await Promise.all([
      import('ol/layer/Vector').then((m) => m.default),
      import('ol/source/Vector').then((m) => m.default),
      import('ol/style/Style').then((m) => m.default),
      import('ol/style/Stroke').then((m) => m.default),
      import('ol/Feature').then((m) => m.default),
      import('ol/geom/LineString').then((m) => m.default),
    ]);

    const source = new OLVectorSource();
    const layer = new OLVectorLayer({
      source,
      style: new OLStyle({
        stroke: new OLStroke({ color: 'rgba(255,255,255,0.3)', width: 1 }),
      }),
    });

    source.addFeatures(this.createSquareGrid(OLFeature, OLLineString));
    this.squareGridLayer = layer;
    return layer;
  }

  private async getOrCreateFogLayer(): Promise<OlVectorLayer> {
    if (this.fogLayer) return this.fogLayer;

    const [
      OLVectorLayer,
      OLVectorSource,
      OLStyle,
      OLFill,
      OLFeature,
      OLPolygon,
    ] = await Promise.all([
      import('ol/layer/Vector').then((m) => m.default),
      import('ol/source/Vector').then((m) => m.default),
      import('ol/style/Style').then((m) => m.default),
      import('ol/style/Fill').then((m) => m.default),
      import('ol/Feature').then((m) => m.default),
      import('ol/geom/Polygon').then((m) => m.default),
    ]);

    const source = new OLVectorSource();
    const layer = new OLVectorLayer({
      source,
      style: new OLStyle({
        fill: new OLFill({ color: 'rgba(0,0,0,0.85)' }),
      }),
      opacity: 0.7,
    });

    source.addFeatures(this.createFogPolygon(OLFeature, OLPolygon));
    this.fogLayer = layer;
    return layer;
  }

  private async getOrCreatePoisLayer(): Promise<OlVectorLayer> {
    if (this.poisLayer) return this.poisLayer;

    const [
      OLVectorLayer,
      OLVectorSource,
      OLStyle,
      OLFill,
      OLStroke,
      OLCircle,
      OLText,
    ] = await Promise.all([
      import('ol/layer/Vector').then((m) => m.default),
      import('ol/source/Vector').then((m) => m.default),
      import('ol/style/Style').then((m) => m.default),
      import('ol/style/Fill').then((m) => m.default),
      import('ol/style/Stroke').then((m) => m.default),
      import('ol/style/Circle').then((m) => m.default),
      import('ol/style/Text').then((m) => m.default),
    ]);

    const source = new OLVectorSource();
    const layer = new OLVectorLayer({
      source,
      style: (feature) =>
        new OLStyle({
          image: new OLCircle({
            radius: 9,
            fill: new OLFill({ color: feature.get('color') ?? '#7c4dff' }),
            stroke: new OLStroke({ color: '#ffffff', width: 2.5 }),
          }),
          text: new OLText({
            text: feature.get('label') ?? '',
            font: 'bold 13px Roboto, sans-serif',
            fill: new OLFill({ color: '#ffffff' }),
            stroke: new OLStroke({ color: '#000000', width: 3 }),
            offsetY: -20,
          }),
        }),
      zIndex: 90,
    });

    this.poisLayer = layer;
    return layer;
  }

  // ──────────────────────────────────────────
  //  Private – geometry / feature generation
  // ──────────────────────────────────────────

  /** Generate pointy-top hex grid features centered on the current view. */
  private createHexGrid(
    Feature: FeatureCtor,
    Polygon: PolygonCtor,
  ): OlFeature[] {
    const center = this.map?.getView().getCenter() ?? [0, 0];
    const hexRadius = 5000;
    const cols = 15;
    const rows = 15;
    const dx = hexRadius * Math.sqrt(3);
    const dy = hexRadius * 1.5;

    const features: OlFeature[] = [];
    const startX = center[0] - (cols / 2) * dx;
    const startY = center[1] - (rows / 2) * dy;

    for (let row = 0; row < rows; row++) {
      for (let col = 0; col < cols; col++) {
        const cx = startX + col * dx + (row % 2) * (dx / 2);
        const cy = startY + row * dy;

        const vertices: Array<[number, number]> = [];
        for (let i = 0; i < 6; i++) {
          const angle = ((60 * i - 30) * Math.PI) / 180; // pointy-top
          vertices.push([
            cx + hexRadius * Math.cos(angle),
            cy + hexRadius * Math.sin(angle),
          ]);
        }
        vertices.push(vertices[0]); // close ring

        features.push(new Feature({ geometry: new Polygon([vertices]) }));
      }
    }

    return features;
  }

  /** Generate square grid line features centered on the current view. */
  private createSquareGrid(
    Feature: FeatureCtor,
    LineString: LineStringCtor,
  ): OlFeature[] {
    const center = this.map?.getView().getCenter() ?? [0, 0];
    const gridSize = 5000;
    const halfExtent = 50000;

    const features: OlFeature[] = [];
    const startX = center[0] - halfExtent;
    const endX = center[0] + halfExtent;
    const startY = center[1] - halfExtent;
    const endY = center[1] + halfExtent;

    // Vertical lines
    for (let x = startX; x <= endX; x += gridSize) {
      features.push(
        new Feature({ geometry: new LineString([[x, startY], [x, endY]]) }),
      );
    }

    // Horizontal lines
    for (let y = startY; y <= endY; y += gridSize) {
      features.push(
        new Feature({ geometry: new LineString([[startX, y], [endX, y]]) }),
      );
    }

    return features;
  }

  /** Create a large dark polygon to serve as the fog-of-war overlay. */
  private createFogPolygon(
    Feature: FeatureCtor,
    Polygon: PolygonCtor,
  ): OlFeature[] {
    const center = this.map?.getView().getCenter() ?? [0, 0];
    const extent = 500_000;

    return [
      new Feature({
        geometry: new Polygon([
          [
            [center[0] - extent, center[1] - extent],
            [center[0] + extent, center[1] - extent],
            [center[0] + extent, center[1] + extent],
            [center[0] - extent, center[1] + extent],
            [center[0] - extent, center[1] - extent],
          ],
        ]),
      }),
    ];
  }

  private async getOrCreateSubmapPinsLayer(): Promise<OlVectorLayer> {
    if (this.submapPinsLayer) return this.submapPinsLayer;

    const [
      OLVectorLayer,
      OLVectorSource,
      OLStyle,
      OLFill,
      OLStroke,
      OLCircle,
      OLText,
    ] = await Promise.all([
      import('ol/layer/Vector').then((m) => m.default),
      import('ol/source/Vector').then((m) => m.default),
      import('ol/style/Style').then((m) => m.default),
      import('ol/style/Fill').then((m) => m.default),
      import('ol/style/Stroke').then((m) => m.default),
      import('ol/style/Circle').then((m) => m.default),
      import('ol/style/Text').then((m) => m.default),
    ]);

    const source = new OLVectorSource();
    const layer = new OLVectorLayer({
      source,
      style: (feature) =>
        new OLStyle({
          image: new OLCircle({
            radius: 12,
            fill: new OLFill({ color: feature.get('color') ?? '#7c4dff' }),
            stroke: new OLStroke({ color: '#ffffff', width: 3 }),
          }),
          text: new OLText({
            text: feature.get('label') ?? '',
            font: 'bold 13px Roboto, sans-serif',
            fill: new OLFill({ color: '#ffffff' }),
            stroke: new OLStroke({ color: '#000000', width: 3 }),
            offsetY: -22,
          }),
        }),
      zIndex: 100,
    });

    this.submapPinsLayer = layer;
    return layer;
  }
}
