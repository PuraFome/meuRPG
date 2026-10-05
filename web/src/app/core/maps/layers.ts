import type { Square } from '../combat/combat-grid';

/** What `GetMapLayers` sends (maps.proto): the grid and the packed layers. */
export interface PackedLayers {
  readonly gridColumns: number;
  readonly gridRows: number;
  readonly difficultTerrain: Uint8Array;
  readonly wall: Uint8Array;
  readonly cover: Uint8Array;
  /** The painted light (two bits a square): only the master gets it. */
  readonly light?: Uint8Array;
}

/** Cover as the layer stores it: 1 half, 2 three-quarters (walls are the wall layer). */
export type PaintedCover = 1 | 2;

/** The squares of each layer, ready to draw. */
export interface MapLayers {
  readonly columns: number;
  readonly rows: number;
  readonly walls: readonly Square[];
  readonly terrain: readonly Square[];
  readonly half: readonly Square[];
  readonly threeQuarters: readonly Square[];
  /** The painted light, by level: the master's editor only (a player never receives it). */
  readonly light?: PaintedLight;
}

/** The squares of each painted light level (`LightLevel`: 1 Escuro, 2 Penumbra, 3 Claro). */
export interface PaintedLight {
  readonly dark: readonly Square[];
  readonly dim: readonly Square[];
  readonly bright: readonly Square[];
}

export const NO_LAYERS: MapLayers = { columns: 0, rows: 0, walls: [], terrain: [], half: [], threeQuarters: [] };

/** The squares set in a one-bit layer: square n = row * columns + col is bit
 * n % 8 (least significant first) of byte n / 8 (the layout `rules/grid`
 * writes). An empty layer is "nothing painted". */
function bitSquares(bytes: Uint8Array, columns: number, rows: number): Square[] {
  const out: Square[] = [];
  const squares = columns * rows;
  for (let n = 0; n < squares; n++) {
    const byte = bytes[n >> 3];
    if (byte !== undefined && (byte >> (n & 7)) & 1) {
      out.push({ col: n % columns, row: Math.floor(n / columns) });
    }
  }
  return out;
}

/** The squares of a two-bit layer holding `value`: square n is bits 2 * (n % 4)
 * and 2 * (n % 4) + 1 of byte n / 4. */
function crumbSquares(bytes: Uint8Array, columns: number, rows: number, value: number): Square[] {
  const out: Square[] = [];
  const squares = columns * rows;
  for (let n = 0; n < squares; n++) {
    const byte = bytes[n >> 2];
    if (byte !== undefined && (byte >> (2 * (n & 3))) & 3) {
      if (((byte >> (2 * (n & 3))) & 3) === value) {
        out.push({ col: n % columns, row: Math.floor(n / columns) });
      }
    }
  }
  return out;
}

/** Unpacks the layers the server sent. It is drawing, not rules: nothing here
 * decides what a wall or a cover does. A grid-less map (0 columns) has none. */
export function decodeLayers(packed: PackedLayers): MapLayers {
  const { gridColumns: columns, gridRows: rows } = packed;
  if (columns <= 0 || rows <= 0) {
    return NO_LAYERS;
  }
  const light = packed.light;
  return {
    columns,
    rows,
    walls: bitSquares(packed.wall, columns, rows),
    terrain: bitSquares(packed.difficultTerrain, columns, rows),
    half: crumbSquares(packed.cover, columns, rows, 1),
    threeQuarters: crumbSquares(packed.cover, columns, rows, 2),
    ...(light !== undefined && light.length > 0
      ? {
          light: {
            dark: crumbSquares(light, columns, rows, 1),
            dim: crumbSquares(light, columns, rows, 2),
            bright: crumbSquares(light, columns, rows, 3),
          },
        }
      : {}),
  };
}

/** Whether there is anything to draw or to name in a legend. */
export function hasLayers(layers: MapLayers): boolean {
  return layers.walls.length + layers.terrain.length + layers.half.length + layers.threeQuarters.length > 0;
}

/** How many squares have painted light (the editor's "Luz" row). */
export function lightCount(layers: MapLayers): number {
  const l = layers.light;
  return l ? l.dark.length + l.dim.length + l.bright.length : 0;
}

/** Whether the master painted anything at all, light included: what a new grid or a new image would erase. */
export function hasPainted(layers: MapLayers): boolean {
  return hasLayers(layers) || lightCount(layers) > 0;
}
