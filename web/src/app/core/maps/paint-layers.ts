import { signal } from '@angular/core';

import { MapLayer } from '../../../gen/meurpg/maps/v1/maps_pb';
import type { Square } from '../combat/combat-grid';
import { type MapLayers, NO_LAYERS, type PackedLayers, decodeLayers } from './layers';

/**
 * The master's copy of the painted layers while he paints (MR-034, E9-01): the packed bytes `GetMapLayers`
 * sent, kept as full-size arrays so a stroke changes them at once (the drawing never waits for the server) and
 * decoded again into the squares the overlay draws. Pure TypeScript, so the byte layout (`rules/grid`'s: one
 * bit a square for terrain and wall, two for cover and light, row-major) is tested without a DOM. It decides
 * nothing: what a wall does is the server's.
 */
export class PaintedLayers {
  readonly layers = signal<MapLayers>(NO_LAYERS);

  private columns = 0;
  private rows = 0;
  private terrain = new Uint8Array();
  private wall = new Uint8Array();
  private cover = new Uint8Array();
  private light = new Uint8Array();
  private frame: number | null = null;

  /** `perFrame`: the bytes change at once but the squares the screen draws are decoded at most once an animation frame, so a drag over a
   * 200 × 400 map decodes the grid once a frame and not once a pointer event. */
  constructor(private readonly perFrame = false) {}

  /** Replaces everything with what the server sent (an empty layer is "nothing painted"). */
  load(packed: PackedLayers): void {
    this.columns = packed.gridColumns;
    this.rows = packed.gridRows;
    const squares = this.columns * this.rows;
    this.terrain = fit(packed.difficultTerrain, Math.ceil(squares / 8));
    this.wall = fit(packed.wall, Math.ceil(squares / 8));
    this.cover = fit(packed.cover, Math.ceil(squares / 4));
    this.light = fit(packed.light ?? new Uint8Array(), Math.ceil(squares / 4));
    this.publish();
  }

  clear(): void {
    this.columns = 0;
    this.rows = 0;
    this.layers.set(NO_LAYERS);
  }

  /** The value of one square of a layer (0 when nothing is painted or the square is off the grid). */
  value(layer: MapLayer, col: number, row: number): number {
    if (col < 0 || row < 0 || col >= this.columns || row >= this.rows) {
      return 0;
    }
    const n = row * this.columns + col;
    switch (layer) {
      case MapLayer.DIFFICULT_TERRAIN:
        return (this.terrain[n >> 3] >> (n & 7)) & 1;
      case MapLayer.WALL:
        return (this.wall[n >> 3] >> (n & 7)) & 1;
      case MapLayer.COVER:
        return (this.cover[n >> 2] >> (2 * (n & 3))) & 3;
      case MapLayer.LIGHT:
        return (this.light[n >> 2] >> (2 * (n & 3))) & 3;
      default:
        return 0;
    }
  }

  /** Paints `value` on `squares` of `layer` in the local copy; the squares that changed (the rest already had it). */
  paint(layer: MapLayer, value: number, squares: readonly Square[]): Square[] {
    const changed: Square[] = [];
    for (const s of squares) {
      if (s.col < 0 || s.row < 0 || s.col >= this.columns || s.row >= this.rows) {
        continue;
      }
      if (this.value(layer, s.col, s.row) === value) {
        continue;
      }
      this.write(layer, s.row * this.columns + s.col, value);
      changed.push(s);
    }
    if (changed.length > 0) {
      this.schedule();
    }
    return changed;
  }

  private write(layer: MapLayer, n: number, value: number): void {
    switch (layer) {
      case MapLayer.DIFFICULT_TERRAIN:
        this.terrain[n >> 3] = (this.terrain[n >> 3] & ~(1 << (n & 7))) | ((value & 1) << (n & 7));
        break;
      case MapLayer.WALL:
        this.wall[n >> 3] = (this.wall[n >> 3] & ~(1 << (n & 7))) | ((value & 1) << (n & 7));
        break;
      case MapLayer.COVER:
        this.cover[n >> 2] = (this.cover[n >> 2] & ~(3 << (2 * (n & 3)))) | ((value & 3) << (2 * (n & 3)));
        break;
      case MapLayer.LIGHT:
        this.light[n >> 2] = (this.light[n >> 2] & ~(3 << (2 * (n & 3)))) | ((value & 3) << (2 * (n & 3)));
        break;
    }
  }

  /** Decodes now what a frame has not shown yet. */
  flush(): void {
    if (this.frame !== null) {
      cancelAnimationFrame(this.frame);
      this.frame = null;
      this.publish();
    }
  }

  private schedule(): void {
    if (!this.perFrame || typeof requestAnimationFrame !== 'function') {
      this.publish();
    } else if (this.frame === null) {
      this.frame = requestAnimationFrame(() => {
        this.frame = null;
        this.publish();
      });
    }
  }

  private publish(): void {
    this.layers.set(
      decodeLayers({
        gridColumns: this.columns,
        gridRows: this.rows,
        difficultTerrain: this.terrain,
        wall: this.wall,
        cover: this.cover,
        light: this.light,
      }),
    );
  }
}

/** The bytes of a layer at full size: what the server sent, or zeros when it sent none. */
function fit(bytes: Uint8Array, size: number): Uint8Array<ArrayBuffer> {
  const out = new Uint8Array(size);
  out.set(bytes.subarray(0, size));
  return out;
}
