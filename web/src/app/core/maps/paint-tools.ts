import { MapLayer } from '../../../gen/meurpg/maps/v1/maps_pb';
import type { Square } from '../combat/combat-grid';
import { tight } from '../format/text';
import { squaresText } from '../units';
import { type DoorKind, type MapLayers, doorCounts, lightCount } from './layers';

/** The five tools of "Pintar" (E9-01, and "Porta" of Etapa 10). */
export type PaintTool = 'terrain' | 'wall' | 'cover' | 'light' | 'door';

/** How much cover a stroke paints: the layer's own values (1 half, 2 three-quarters). */
export type CoverDegree = 1 | 2;

/** How lit a stroke paints: `LightLevel` (1 Escuro, 2 Penumbra, 3 Claro). */
export type LightDegree = 1 | 2 | 3;

export interface PaintSettings {
  /** The tool in hand, or `null` while none was chosen: a stroke paints nothing until the master picks one. */
  readonly tool: PaintTool | null;
  readonly cover: CoverDegree;
  /** The light a stroke paints: the map's base light is the starting point (`Claro`, `Penumbra`, `Escuro`). */
  readonly light: LightDegree;
  /** The door a tap paints (`DoorState`): the door tool puts one square at a time, whatever the brush. */
  readonly door: DoorKind;
  readonly brush: 1 | 3;
  readonly erase: boolean;
}

export const DEFAULT_SETTINGS: PaintSettings = {
  tool: null,
  cover: 1,
  light: 2,
  door: 2,
  brush: 1,
  erase: false,
};

export const TOOL_LABEL: Readonly<Record<PaintTool, string>> = {
  terrain: 'Terreno difícil',
  wall: 'Parede',
  cover: 'Cobertura',
  light: 'Luz',
  door: 'Porta',
};

/** The kinds the door tool offers, in the order of the second row ("Tipo de porta"). */
export const DOOR_CHOICES: readonly DoorKind[] = [2, 1, 3, 4, 5];
/** What each kind is called on the tool's buttons (the legend says "Porta fechada"; the row is already about doors). */
export const DOOR_LABEL: Readonly<Record<DoorKind, string>> = {
  1: 'Aberta',
  2: 'Fechada',
  3: 'Trancada',
  4: 'Grade',
  5: 'Secreta',
};

export const COVER_LABEL: Readonly<Record<CoverDegree, string>> = { 1: 'Meia', 2: 'Três quartos' };
export const LIGHT_LABEL: Readonly<Record<LightDegree, string>> = {
  3: 'Claro',
  2: 'Penumbra',
  1: 'Escuro',
};

const LAYER: Readonly<Record<PaintTool, MapLayer>> = {
  terrain: MapLayer.DIFFICULT_TERRAIN,
  wall: MapLayer.WALL,
  cover: MapLayer.COVER,
  light: MapLayer.LIGHT,
  door: MapLayer.DOORS,
};

/** The layer and value a stroke sends: erasing is value 0 of the chosen tool's layer. No tool in hand, no stroke. */
export function strokeOf(s: PaintSettings): { layer: MapLayer; value: number } | null {
  if (s.tool === null) {
    return null;
  }
  const layer = LAYER[s.tool];
  if (s.erase) {
    return { layer, value: 0 };
  }
  switch (s.tool) {
    case 'cover':
      return { layer, value: s.cover };
    case 'light':
      return { layer, value: s.light };
    case 'door':
      return { layer, value: s.door };
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

/** "Terreno difícil · arraste para pintar · Shift apaga": the line over the map's corner. */
export function paintHint(s: PaintSettings): string {
  if (s.tool === null) {
    return 'Escolha uma ferramenta para pintar';
  }
  if (s.tool === 'door') {
    return s.erase
      ? 'Porta · toque numa porta para tirá-la'
      : `Porta · ${DOOR_LABEL[s.door]} · toque para pôr · Shift tira`;
  }
  const what =
    s.tool === 'cover'
      ? `Cobertura · ${COVER_LABEL[s.cover]}`
      : s.tool === 'light'
        ? `Luz · ${LIGHT_LABEL[s.light]}`
        : TOOL_LABEL[s.tool];
  return s.erase
    ? `${TOOL_LABEL[s.tool]} · arraste para apagar`
    : `${what} · arraste para pintar · Shift apaga`;
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
  const doors = l.doors?.length ?? 0;
  let coverText = 'nada pintado';
  if (l.half.length > 0 && l.threeQuarters.length > 0) {
    coverText = tight(
      `${squaresText(l.half.length)} de meia cobertura e ${l.threeQuarters.length} de três quartos`,
    );
  } else if (l.half.length > 0) {
    coverText = tight(`${squaresText(l.half.length)} de meia cobertura`);
  } else if (l.threeQuarters.length > 0) {
    coverText = tight(`${squaresText(l.threeQuarters.length)} de três quartos`);
  }
  return [
    {
      tool: 'terrain',
      name: 'Terreno difícil',
      count: l.terrain.length,
      detail:
        l.terrain.length > 0
          ? tight(`${squaresText(l.terrain.length)} · custa +1,5 m por quadrado`)
          : 'nada pintado',
    },
    {
      tool: 'wall',
      name: 'Parede',
      count: l.walls.length,
      detail:
        l.walls.length > 0
          ? `${squaresText(l.walls.length)} · bloqueia movimento, visão e luz`
          : 'nada pintado',
    },
    { tool: 'cover', name: 'Cobertura', count: cover, detail: coverText },
    {
      tool: 'light',
      name: 'Luz',
      count: light,
      detail:
        light > 0
          ? `${squaresText(light)} ${light === 1 ? 'pintado' : 'pintados'}`
          : 'nada pintado',
    },
    { tool: 'door', name: 'Portas', count: doors, detail: doorsText(l) },
  ];
}

/** "3 portas · 1 trancada": how many doors, and the ones the master should not forget. */
function doorsText(l: MapLayers): string {
  const counts = doorCounts(l);
  const total = l.doors?.length ?? 0;
  if (total === 0) {
    return 'nada pintado';
  }
  const notes = [
    counts[3] > 0 ? `${counts[3]} ${counts[3] === 1 ? 'trancada' : 'trancadas'}` : '',
    counts[5] > 0 ? `${counts[5]} ${counts[5] === 1 ? 'secreta' : 'secretas'}` : '',
  ].filter(Boolean);
  return tight([`${total} ${total === 1 ? 'porta' : 'portas'}`, ...notes].join(' · '));
}
