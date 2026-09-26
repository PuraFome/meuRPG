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

/** Terreno/estrutura de uma célula da masmorra. */
export type DungeonTileType =
  | 'floor'
  | 'water'
  | 'difficult'
  | 'rubble'
  | 'wall'
  | 'false_wall';

/** Elementos posicionados sobre uma célula (portas, armadilhas, baús, tokens). */
export type DungeonObjectType =
  | 'door'
  | 'secret_door'
  | 'trap'
  | 'chest'
  | 'mimic'
  | 'character';

export interface DungeonObject {
  type: DungeonObjectType;
  label?: string;
  characterId?: string;
  color?: string;
  icon?: string;
}

/**
 * Grade esparsa da masmorra. As chaves são `"col,row"`: `tiles` guarda o
 * terreno/estrutura e `objects` guarda os elementos posicionados na célula.
 */
export type DungeonTiles = Record<string, DungeonTileType>;
export type DungeonObjects = Record<string, DungeonObject>;

export interface DungeonData {
  tiles: DungeonTiles;
  objects: DungeonObjects;
}

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
  /** Células e elementos desenhados da masmorra (opcional em mapas antigos). */
  dungeon?: DungeonData;
  createdAt: Date;
  updatedAt: Date;
}
