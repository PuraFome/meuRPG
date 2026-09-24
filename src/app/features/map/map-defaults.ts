import type { FogOfWar, MapData, MapGrid, MapKind } from '../../core/models/map';

export const DEFAULT_CELL_SIZE = 50;
export const DEFAULT_COLUMNS = 24;
export const DEFAULT_ROWS = 18;

export function createDefaultGrid(overrides?: Partial<MapGrid>): MapGrid {
  return {
    cellSize: DEFAULT_CELL_SIZE,
    columns: DEFAULT_COLUMNS,
    rows: DEFAULT_ROWS,
    visible: false,
    ...overrides,
  };
}

export function createDefaultFog(): FogOfWar {
  return { explored: [], visible: false };
}

/** Gera uma imagem de fundo em branco (pergaminho) para mapas sem upload. */
export function createBlankBackground(width: number, height: number): string {
  const canvas = document.createElement('canvas');
  canvas.width = width;
  canvas.height = height;
  const ctx = canvas.getContext('2d');
  if (ctx) {
    ctx.fillStyle = '#f4ecd8';
    ctx.fillRect(0, 0, width, height);
    ctx.strokeStyle = 'rgba(120, 100, 70, 0.25)';
    ctx.lineWidth = 1;
    ctx.strokeRect(0.5, 0.5, width - 1, height - 1);
  }
  return canvas.toDataURL('image/png');
}

export interface MapImageInput {
  dataUrl: string;
  width: number;
  height: number;
}

export function createMapData(
  name: string,
  kind: MapKind,
  image?: MapImageInput,
): MapData {
  const now = new Date();
  const base = createDefaultGrid();
  const width = image?.width ?? base.columns * base.cellSize;
  const height = image?.height ?? base.rows * base.cellSize;
  const grid: MapGrid = {
    ...base,
    columns: Math.max(1, Math.round(width / base.cellSize)),
    rows: Math.max(1, Math.round(height / base.cellSize)),
  };
  return {
    id: crypto.randomUUID(),
    name,
    kind,
    backgroundImage: image?.dataUrl ?? createBlankBackground(width, height),
    width,
    height,
    layers: [],
    grid,
    fogOfWar: createDefaultFog(),
    markers: [],
    submaps: [],
    dungeon: {},
    createdAt: now,
    updatedAt: now,
  };
}
