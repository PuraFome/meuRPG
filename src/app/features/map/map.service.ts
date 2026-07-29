import { Injectable, inject, NgZone } from '@angular/core';

type OlMap = import('ol/Map').default;
type OlVectorLayer = import('ol/layer/Vector').default;
type OlVectorSource = import('ol/source/Vector').default;
type OlFeature = import('ol/Feature').default;

export interface MapInitOptions {
  zoom?: number;
  center?: [number, number];
}

// Constructor types used in feature factories
type FeatureCtor = typeof import('ol/Feature').default;
type PolygonCtor = typeof import('ol/geom/Polygon').default;
type LineStringCtor = typeof import('ol/geom/LineString').default;
type PointCtor = typeof import('ol/geom/Point').default;

@Injectable({ providedIn: 'root' })
export class MapService {
  private map: OlMap | null = null;
  private initPromise: Promise<OlMap> | null = null;
  private readonly zone = inject(NgZone);

  // Vector layer references
  private hexGridLayer: OlVectorLayer | null = null;
  private squareGridLayer: OlVectorLayer | null = null;
  private fogLayer: OlVectorLayer | null = null;
  private markersLayer: OlVectorLayer | null = null;

  // Visibility state
  private hexGridVisible = false;
  private squareGridVisible = false;
  private fogVisible = false;
  private markersVisible = false;

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

      const map = new OLMap({
        target: container,
        layers: [
          new OLTileLayer({
            source: new OSM(),
          }),
        ],
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

  /** Destroy the map and release resources. */
  destroy(): void {
    if (this.map) {
      this.map.setTarget(undefined);
      this.map = null;
      this.initPromise = null;
    }
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

  /** Toggle markers layer. */
  async toggleMarkers(): Promise<void> {
    if (!this.map) return;

    this.markersVisible = !this.markersVisible;
    if (this.markersVisible) {
      this.map.addLayer(await this.getOrCreateMarkersLayer());
    } else if (this.markersLayer) {
      this.map.removeLayer(this.markersLayer);
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

  private async getOrCreateMarkersLayer(): Promise<OlVectorLayer> {
    if (this.markersLayer) return this.markersLayer;

    const [
      OLVectorLayer,
      OLVectorSource,
      OLStyle,
      OLFill,
      OLStroke,
      OLCircle,
      OLText,
      OLFeature,
      OLPoint,
      projModule,
    ] = await Promise.all([
      import('ol/layer/Vector').then((m) => m.default),
      import('ol/source/Vector').then((m) => m.default),
      import('ol/style/Style').then((m) => m.default),
      import('ol/style/Fill').then((m) => m.default),
      import('ol/style/Stroke').then((m) => m.default),
      import('ol/style/Circle').then((m) => m.default),
      import('ol/style/Text').then((m) => m.default),
      import('ol/Feature').then((m) => m.default),
      import('ol/geom/Point').then((m) => m.default),
      import('ol/proj'),
    ]);

    const source = new OLVectorSource();
    const layer = new OLVectorLayer({
      source,
      style: (feature) =>
        new OLStyle({
          image: new OLCircle({
            radius: 8,
            fill: new OLFill({ color: '#e53935' }),
            stroke: new OLStroke({ color: '#ffffff', width: 2 }),
          }),
          text: new OLText({
            text: feature.get('label') ?? '',
            font: '12px Roboto, sans-serif',
            fill: new OLFill({ color: '#ffffff' }),
            stroke: new OLStroke({ color: '#000000', width: 3 }),
            offsetY: -15,
          }),
        }),
    });

    source.addFeatures(
      this.createMarkers(OLFeature, OLPoint, (c: [number, number]) => projModule.fromLonLat(c)),
    );
    this.markersLayer = layer;
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

  /** Create point features with labels for demonstration markers. */
  private createMarkers(
    Feature: FeatureCtor,
    Point: PointCtor,
    project: (coord: [number, number]) => number[],
  ): OlFeature[] {
    const markersData = [
      { lon: -46.6333, lat: -23.5505, label: 'Centro' },
      { lon: -46.62, lat: -23.54, label: 'Pin 1' },
      { lon: -46.65, lat: -23.56, label: 'Pin 2' },
      { lon: -46.61, lat: -23.57, label: 'Pin 3' },
    ];

    return markersData.map((m) => {
      const feature = new Feature({
        geometry: new Point(project([m.lon, m.lat])),
      });
      feature.set('label', m.label);
      return feature;
    });
  }
}
