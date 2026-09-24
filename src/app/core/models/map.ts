/** Categoria do mapa, usada para organizar mundos, cidades, masmorras e locais. */
export type MapKind = 'world' | 'city' | 'dungeon' | 'local';

export interface MapGrid {
  /** Tamanho de uma célula em pixels da imagem de fundo. */
  cellSize: number;
  columns: number;
  rows: number;
  color?: string;
  visible: boolean;
}

export interface FogOfWar {
  explored: { x: number; y: number }[][];
  visible: boolean;
}

export interface MapMarker {
  id: string;
  x: number;
  y: number;
  label: string;
  icon?: string;
  color?: string;
  description?: string;
  targetMapId?: string;
}

export interface MapLayer {
  id: string;
  name: string;
  visible: boolean;
  opacity: number;
  data: string;
  grid?: MapGrid;
  fogOfWar?: FogOfWar;
  markers: MapMarker[];
}

export interface SubmapPin {
  id: string;
  x: number;
  y: number;
  targetMapId: string;
  label: string;
  icon?: string;
  color?: string;
}

/** Tipos de célula usados para desenhar a masmorra sobre a grade. */
export type DungeonTileType =
  | 'floor'
  | 'wall'
  | 'door'
  | 'water'
  | 'difficult';

/**
 * Grade esparsa da masmorra: a chave é `"col,row"` e o valor é o tipo da
 * célula. Guardar apenas as células preenchidas mantém o JSON pequeno mesmo
 * em masmorras grandes.
 */
export type DungeonTiles = Record<string, DungeonTileType>;

export interface MapData {
  id: string;
  name: string;
  description?: string;
  /** Categoria do mapa (mundo, cidade ou masmorra). */
  kind?: MapKind;
  backgroundImage?: string;
  width: number;
  height: number;
  layers: MapLayer[];
  grid: MapGrid;
  fogOfWar: FogOfWar;
  markers: MapMarker[];
  submaps: SubmapPin[];
  /** Células desenhadas da masmorra (opcional em mapas antigos). */
  dungeon?: DungeonTiles;
  createdAt: Date;
  updatedAt: Date;
}
