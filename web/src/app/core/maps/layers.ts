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
  /** The doors (four bits a square, a `DoorState`): as the viewer knows them (a player's has no locked or secret door). */
  readonly doors?: Uint8Array;
}

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
  /** The doors the viewer knows (RN-26). Absent when the map has none. */
  readonly doors?: readonly DoorSquare[];
}

/** A door as the layer stores it (`DoorState`): 1 open, 2 closed, 3 locked, 4 barred (a grade), 5 secret. */
export type DoorKind = 1 | 2 | 3 | 4 | 5;

/** A door's square, its kind and which way the gap runs: `h` when the bar lies along a wall that runs left and right
 * (the passage goes up and down), `v` the other way. Only drawing: the server decides what a door does. */
export interface DoorSquare extends Square {
  readonly state: DoorKind;
  readonly axis: 'h' | 'v';
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

/** The doors of a four-bit layer: square n is the low nibble of byte n / 2 when n is even and the high nibble when odd.
 * The axis follows the same rule as `planDoor` (a door needs floor on both sides): floor above and below means the passage runs up and
 * down, so the bar lies left to right (`h`); otherwise floor left and right turns it (`v`). Doors of one corridor point the same way.
 * `player` is the guard for a player's view (RN-10): the server never sends a locked or a secret door to a player, and if one ever
 * came, a locked door is drawn closed and a secret one is not drawn at all. */
function doorSquares(bytes: Uint8Array, columns: number, rows: number, walls: readonly Square[], player: boolean): DoorSquare[] {
  const out: DoorSquare[] = [];
  const squares = columns * rows;
  const wallAt = new Set(walls.map((w) => w.row * columns + w.col));
  const floor = (col: number, row: number) => col >= 0 && row >= 0 && col < columns && row < rows && !wallAt.has(row * columns + col);
  for (let n = 0; n < squares; n++) {
    const byte = bytes[n >> 1];
    let state = byte === undefined ? 0 : (n & 1 ? byte >> 4 : byte) & 15;
    if (player) {
      if (state === 5) {
        continue;
      }
      if (state === 3) {
        state = 2;
      }
    }
    if (state >= 1 && state <= 5) {
      const col = n % columns;
      const row = Math.floor(n / columns);
      const upDown = floor(col, row - 1) && floor(col, row + 1);
      const leftRight = floor(col - 1, row) && floor(col + 1, row);
      out.push({ col, row, state: state as DoorKind, axis: !upDown && leftRight ? 'v' : 'h' });
    }
  }
  return out;
}

/** Unpacks the layers the server sent. It is drawing, not rules: nothing here
 * decides what a wall or a cover does. A grid-less map (0 columns) has none. */
export function decodeLayers(packed: PackedLayers, player = false): MapLayers {
  const { gridColumns: columns, gridRows: rows } = packed;
  if (columns <= 0 || rows <= 0) {
    return NO_LAYERS;
  }
  const light = packed.light;
  const walls = bitSquares(packed.wall, columns, rows);
  const doors = packed.doors !== undefined && packed.doors.length > 0 ? doorSquares(packed.doors, columns, rows, walls, player) : [];
  return {
    columns,
    rows,
    walls,
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
    ...(doors.length > 0 ? { doors } : {}),
  };
}

/** Whether there is anything to draw or to name in a legend. */
export function hasLayers(layers: MapLayers): boolean {
  return layers.walls.length + layers.terrain.length + layers.half.length + layers.threeQuarters.length + (layers.doors?.length ?? 0) > 0;
}

/** The doors of each kind, for the legend and the "Camadas" row. */
export function doorCounts(layers: MapLayers): Readonly<Record<DoorKind, number>> {
  const counts: Record<DoorKind, number> = { 1: 0, 2: 0, 3: 0, 4: 0, 5: 0 };
  for (const d of layers.doors ?? []) {
    counts[d.state]++;
  }
  return counts;
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
