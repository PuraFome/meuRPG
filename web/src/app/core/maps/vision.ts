import type { GetMapVisionResponse } from '../../../gen/meurpg/maps/v1/maps_pb';
import type { Square } from '../combat/combat-grid';

/**
 * What `GetMapVision` says of each square (maps.proto): the values of the
 * packed four-bit states. The browser only names them and draws them; which
 * square is which is the server's work (RN-10, `rules/vision`).
 */
export const Sight = {
  /** Nothing is known of the square: solid black. */
  Unseen: 0,
  /** A wall seen because a square next to it is seen. */
  Wall: 1,
  /** Seen in the dark, by darkvision or blindsight: the image without colour. */
  Grey: 2,
  /** Seen in dim light (penumbra). */
  Dim: 3,
  /** Seen in bright light. */
  Bright: 4,
  /** Seen before, not now: darkened, with dots, and no NPCs. */
  Remembered: 5,
} as const;

/** The four ways a square is drawn over the image. "Visto" (bright light, or a
 * wall seen now) is the image as it is, so it has no shade. */
export type Shade = 'dim' | 'grey' | 'remembered' | 'unseen';

/** One tile of the viewer's image (`MapTile`). */
export interface VisionTile {
  readonly tx: number;
  readonly ty: number;
  readonly revision: number;
}

/** `GetMapVision`, unpacked: one state per square, row-major. */
export interface Vision {
  readonly columns: number;
  readonly rows: number;
  readonly states: Uint8Array;
  readonly revision: number;
  readonly characterOnMap: boolean;
  readonly fogEnabled: boolean;
  readonly groupVision: boolean;
  /** The tiles' route, "/images/maps/<id>/tiles/"; empty when the viewer reads the whole image. */
  readonly tilesPath: string;
  readonly tileSquares: number;
  readonly tiles: readonly VisionTile[];
}

/** A row of squares drawn alike: `len` squares from (`col`, `row`). */
export interface ShadeRun {
  readonly shade: Shade;
  readonly col: number;
  readonly row: number;
  readonly len: number;
}

/** A tile placed on the grid: the squares it covers, and where it is fetched. */
export interface TileRect {
  readonly key: string;
  readonly tx: number;
  readonly ty: number;
  readonly revision: number;
  readonly col: number;
  readonly row: number;
  readonly cols: number;
  readonly rows: number;
}

/** Unpacks the states: four bits a square, two squares to a byte, the even
 * square in the low bits (square n = row * columns + col). */
export function unpackStates(packed: Uint8Array, columns: number, rows: number): Uint8Array {
  const squares = Math.max(0, columns) * Math.max(0, rows);
  const out = new Uint8Array(squares);
  for (let n = 0; n < squares; n++) {
    const byte = packed[n >> 1] ?? 0;
    out[n] = n & 1 ? byte >> 4 : byte & 0x0f;
  }
  return out;
}

export function decodeVision(res: GetMapVisionResponse): Vision {
  return {
    columns: res.gridColumns,
    rows: res.gridRows,
    states: unpackStates(res.states, res.gridColumns, res.gridRows),
    revision: res.revision,
    characterOnMap: res.characterOnMap,
    fogEnabled: res.fogEnabled,
    groupVision: res.groupVision,
    tilesPath: res.tilesPath,
    tileSquares: res.tileSquares,
    tiles: res.tiles.map((t) => ({ tx: t.tx, ty: t.ty, revision: t.revision })),
  };
}

/** The shade a state is drawn with, or `null` for "Visto" (the image as it is). */
export function shadeOf(state: number): Shade | null {
  switch (state) {
    case Sight.Dim:
      return 'dim';
    case Sight.Grey:
      return 'grey';
    case Sight.Remembered:
      return 'remembered';
    case Sight.Unseen:
      return 'unseen';
    default:
      return null;
  }
}

/** The squares that need a shade, joined in rows: a handful of elements for a
 * whole map instead of one for each square. */
export function shadeRuns(vision: Vision): ShadeRun[] {
  const runs: ShadeRun[] = [];
  const { columns, rows, states } = vision;
  for (let row = 0; row < rows; row++) {
    let col = 0;
    while (col < columns) {
      const shade = shadeOf(states[row * columns + col] ?? Sight.Unseen);
      if (shade === null) {
        col++;
        continue;
      }
      let end = col + 1;
      while (end < columns && shadeOf(states[row * columns + end] ?? Sight.Unseen) === shade) {
        end++;
      }
      runs.push({ shade, col, row, len: end - col });
      col = end;
    }
  }
  return runs;
}

/** How many squares the viewer sees now (a wall next to a seen square counts:
 * it is seen). Counting, not rules: the server decided each square. */
export function seenCount(vision: Vision): number {
  let n = 0;
  for (const s of vision.states) {
    if (s >= Sight.Wall && s <= Sight.Bright) {
      n++;
    }
  }
  return n;
}

/** A rectangle of squares: `cols` by `rows` from (`col`, `row`). */
export interface SquareWindow {
  readonly col: number;
  readonly row: number;
  readonly cols: number;
  readonly rows: number;
}

/**
 * The squares the viewer knows anything of (seen now, seen before, or a wall
 * beside them) and the extra squares `squares` that hold a token, as the
 * smallest rectangle around them plus `margin` squares each side, kept on the
 * grid; `null` when nothing is known. Counting, not rules: the server decided
 * each square.
 */
export function knownWindow(
  vision: Vision,
  tokens: readonly Square[],
  margin: number,
): SquareWindow | null {
  let c0 = vision.columns;
  let r0 = vision.rows;
  let c1 = -1;
  let r1 = -1;
  const take = (col: number, row: number) => {
    c0 = Math.min(c0, col);
    r0 = Math.min(r0, row);
    c1 = Math.max(c1, col);
    r1 = Math.max(r1, row);
  };
  vision.states.forEach((state, n) => {
    if (state !== Sight.Unseen) {
      take(n % vision.columns, Math.floor(n / vision.columns));
    }
  });
  for (const t of tokens) {
    take(t.col, t.row);
  }
  if (c1 < 0) {
    return null;
  }
  const col = Math.max(0, c0 - margin);
  const row = Math.max(0, r0 - margin);
  return {
    col,
    row,
    cols: Math.min(vision.columns, c1 + margin + 1) - col,
    rows: Math.min(vision.rows, r1 + margin + 1) - row,
  };
}

/** Which states are on the map, for the legend (it names only what is drawn). */
export interface VisionLegend {
  readonly seen: boolean;
  readonly dim: boolean;
  readonly grey: boolean;
  readonly remembered: boolean;
  readonly unseen: boolean;
}

export function visionLegend(vision: Vision): VisionLegend {
  const present = new Set<number>();
  for (const s of vision.states) {
    present.add(s);
  }
  return {
    seen: present.has(Sight.Bright) || present.has(Sight.Wall),
    dim: present.has(Sight.Dim),
    grey: present.has(Sight.Grey),
    remembered: present.has(Sight.Remembered),
    unseen: present.has(Sight.Unseen),
  };
}

/** The tiles the viewer has, placed on the grid (the last row and column of
 * tiles may be smaller). */
export function tileRects(vision: Vision): TileRect[] {
  const size = vision.tileSquares;
  if (size <= 0 || vision.tilesPath === '') {
    return [];
  }
  return vision.tiles.map((t) => {
    const col = t.tx * size;
    const row = t.ty * size;
    return {
      key: `${t.tx}:${t.ty}`,
      tx: t.tx,
      ty: t.ty,
      revision: t.revision,
      col,
      row,
      cols: Math.max(0, Math.min(size, vision.columns - col)),
      rows: Math.max(0, Math.min(size, vision.rows - row)),
    };
  });
}

/** Where a tile is fetched: the route, the tile and its revision (`r`, which
 * changes when the tile gains a square, so an unchanged tile keeps its URL and
 * the browser keeps its copy), and, for the master reading as a player, `as`. */
export function tileUrl(
  vision: Vision,
  tile: { tx: number; ty: number; revision: number },
  as: string | null,
): string {
  const base = `${vision.tilesPath}${tile.tx}/${tile.ty}?r=${tile.revision}`;
  return as ? `${base}&as=${encodeURIComponent(as)}` : base;
}

/** The tiles that are not on screen yet, of all of them: "parte N de M". */
export interface TileProgress {
  readonly total: number;
  readonly done: number;
}

export function tileProgress(
  rects: readonly TileRect[],
  settled: ReadonlySet<string>,
): TileProgress {
  return { total: rects.length, done: rects.filter((r) => settled.has(r.key)).length };
}

/** A block of squares drawn alike: `cols` wide and `rows` tall from (`col`, `row`). */
export interface ShadeRect {
  readonly shade: Shade;
  readonly col: number;
  readonly row: number;
  readonly cols: number;
  readonly rows: number;
}

/** The runs, with the ones that sit one on top of the other, equal in column and
 * length, joined into one block: the cave's black is a few blocks, not hundreds
 * of squares, so there are no seams between them. */
export function shadeRects(vision: Vision): ShadeRect[] {
  const open = new Map<string, ShadeRect>();
  const done: ShadeRect[] = [];
  let previous: string[] = [];
  const runsByRow = new Map<number, ShadeRun[]>();
  for (const run of shadeRuns(vision)) {
    const list = runsByRow.get(run.row) ?? [];
    list.push(run);
    runsByRow.set(run.row, list);
  }
  for (let row = 0; row < vision.rows; row++) {
    const keys: string[] = [];
    for (const run of runsByRow.get(row) ?? []) {
      const key = `${run.shade}:${run.col}:${run.len}`;
      keys.push(key);
      const above = open.get(key);
      if (above && previous.includes(key)) {
        open.set(key, { ...above, rows: above.rows + 1 });
      } else {
        open.set(key, { shade: run.shade, col: run.col, row, cols: run.len, rows: 1 });
      }
    }
    for (const key of previous) {
      if (!keys.includes(key)) {
        const rect = open.get(key);
        if (rect) {
          done.push(rect);
        }
        open.delete(key);
      }
    }
    previous = keys;
  }
  done.push(...open.values());
  return done;
}
