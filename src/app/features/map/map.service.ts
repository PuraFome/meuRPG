import { Injectable, inject, NgZone } from '@angular/core';

type OlMap = import('ol/Map').default;
type OlTileLayer = import('ol/layer/Tile').default;

export interface MapInitOptions {
  zoom?: number;
  center?: [number, number];
}

@Injectable({ providedIn: 'root' })
export class MapService {
  private map: OlMap | null = null;
  private initPromise: Promise<OlMap> | null = null;
  private readonly zone = inject(NgZone);

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

  /** Add a tile layer to the map. */
  addLayer(layer: OlTileLayer): void {
    this.map?.addLayer(layer);
  }

  /** Remove a tile layer from the map. */
  removeLayer(layer: OlTileLayer): void {
    this.map?.removeLayer(layer);
  }

  /** Toggle fullscreen on the given element. */
  toggleFullscreen(element: HTMLElement): void {
    if (!document.fullscreenElement) {
      element.requestFullscreen().catch(() => {});
    } else {
      document.exitFullscreen().catch(() => {});
    }
  }

  /** Destroy the map and release resources. */
  destroy(): void {
    if (this.map) {
      this.map.setTarget(undefined);
      this.map = null;
      this.initPromise = null;
    }
  }
}
