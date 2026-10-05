import { MapLayer } from '../../../gen/meurpg/maps/v1/maps_pb';
import { type Square, squareAt } from '../combat/combat-grid';
import { tight } from '../format/text';
import { type MapLayers, lightCount } from './layers';

/** The four tools of "Pintar" (E9-01). */
export type PaintTool = 'terrain' | 'wall' | 'cover' | 'light';

/** How much cover a stroke paints: the layer's own values (1 half, 2 three-quarters). */
export type CoverDegree = 1 | 2;

/** How lit a stroke paints: `LightLevel` (1 Escuro, 2 Penumbra, 3 Claro). */
export type LightDegree = 1 | 2 | 3;

export interface PaintSettings {
  readonly tool: PaintTool;
  readonly cover: CoverDegree;
  /** The light a stroke paints: the map's base light is the starting point (`Claro`, `Penumbra`, `Escuro`). */
  readonly light: LightDegree;
  readonly brush: 1 | 3;
  readonly erase: boolean;
}

export const DEFAULT_SETTINGS: PaintSettings = { tool: 'terrain', cover: 1, light: 2, brush: 1, erase: false };

export const TOOL_LABEL: Readonly<Record<PaintTool, string>> = {
  terrain: 'Terreno difícil',
  wall: 'Parede',
  cover: 'Cobertura',
  light: 'Luz',
};

export const COVER_LABEL: Readonly<Record<CoverDegree, string>> = { 1: 'Meia', 2: 'Três quartos' };
export const LIGHT_LABEL: Readonly<Record<LightDegree, string>> = { 3: 'Claro', 2: 'Penumbra', 1: 'Escuro' };

const LAYER: Readonly<Record<PaintTool, MapLayer>> = {
  terrain: MapLayer.DIFFICULT_TERRAIN,
  wall: MapLayer.WALL,
  cover: MapLayer.COVER,
  light: MapLayer.LIGHT,
};

/** The layer and value a stroke sends: erasing is value 0 of the chosen tool's layer. */
export function strokeOf(s: PaintSettings): { layer: MapLayer; value: number } {
  const layer = LAYER[s.tool];
  if (s.erase) {
    return { layer, value: 0 };
  }
  switch (s.tool) {
    case 'cover':
      return { layer, value: s.cover };
    case 'light':
      return { layer, value: s.light };
    default:
      return { layer, value: 1 };
  }
}

/** The squares a brush covers when its center is on `at` (1 square, or 3 × 3), kept inside the grid. */
export function brushSquares(at: Square, brush: 1 | 3, columns: number, rows: number): Square[] {
  const reach = brush === 3 ? 1 : 0;
  const out: Square[] = [];
  for (let row = at.row - reach; row <= at.row + reach; row++) {
    for (let col = at.col - reach; col <= at.col + reach; col++) {
      if (col >= 0 && row >= 0 && col < columns && row < rows) {
        out.push({ col, row });
      }
    }
  }
  return out;
}

/** Every square on the straight line from one square to another (Bresenham), both ends included: a fast drag
 * skips squares between two pointer events, and "square by square" means none is left unpainted. */
export function lineSquares(from: Square, to: Square): Square[] {
  const out: Square[] = [];
  let { col, row } = from;
  const dc = Math.abs(to.col - col);
  const dr = -Math.abs(to.row - row);
  const sc = col < to.col ? 1 : -1;
  const sr = row < to.row ? 1 : -1;
  let err = dc + dr;
  for (;;) {
    out.push({ col, row });
    if (col === to.col && row === to.row) {
      return out;
    }
    const e2 = 2 * err;
    if (e2 >= dr) {
      err += dr;
      col += sc;
    }
    if (e2 <= dc) {
      err += dc;
      row += sr;
    }
  }
}

/** The square under a pointer, from its position inside the map's box (fractions 0 to 1). */
export function squareUnder(x: number, y: number, columns: number, rows: number): Square {
  return squareAt(x, y, columns, rows);
}

/** "Terreno difícil · arraste para pintar · Shift apaga": the line over the map's corner. */
export function paintHint(s: PaintSettings): string {
  const what =
    s.tool === 'cover' ? `Cobertura · ${COVER_LABEL[s.cover]}` : s.tool === 'light' ? `Luz · ${LIGHT_LABEL[s.light]}` : TOOL_LABEL[s.tool];
  return s.erase ? `${TOOL_LABEL[s.tool]} · arraste para apagar` : `${what} · arraste para pintar · Shift apaga`;
}

function squares(n: number): string {
  return tight(`${n.toLocaleString('pt-BR')} ${n === 1 ? 'quadrado' : 'quadrados'}`);
}

/** What the "Camadas" list says under each layer's name. */
export interface LayerLine {
  readonly tool: PaintTool;
  readonly name: string;
  readonly detail: string;
  readonly count: number;
}

export function layerLines(l: MapLayers): readonly LayerLine[] {
  const cover = l.half.length + l.threeQuarters.length;
  const light = lightCount(l);
  let coverText = 'nada pintado';
  if (l.half.length > 0 && l.threeQuarters.length > 0) {
    coverText = tight(`${squares(l.half.length)} de meia cobertura e ${l.threeQuarters.length} de três quartos`);
  } else if (l.half.length > 0) {
    coverText = tight(`${squares(l.half.length)} de meia cobertura`);
  } else if (l.threeQuarters.length > 0) {
    coverText = tight(`${squares(l.threeQuarters.length)} de três quartos`);
  }
  return [
    {
      tool: 'terrain',
      name: 'Terreno difícil',
      count: l.terrain.length,
      detail: l.terrain.length > 0 ? tight(`${squares(l.terrain.length)} · custa +1,5 m por quadrado`) : 'nada pintado',
    },
    {
      tool: 'wall',
      name: 'Parede',
      count: l.walls.length,
      detail: l.walls.length > 0 ? `${squares(l.walls.length)} · bloqueia movimento, visão e luz` : 'nada pintado',
    },
    { tool: 'cover', name: 'Cobertura', count: cover, detail: coverText },
    { tool: 'light', name: 'Luz', count: light, detail: light > 0 ? `${squares(light)} ${light === 1 ? 'pintado' : 'pintados'}` : 'nada pintado' },
  ];
}
