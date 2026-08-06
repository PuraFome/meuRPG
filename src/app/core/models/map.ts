export interface MapGrid {
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

export interface MapData {
  id: string;
  name: string;
  description?: string;
  backgroundImage?: string;
  width: number;
  height: number;
  layers: MapLayer[];
  grid: MapGrid;
  fogOfWar: FogOfWar;
  markers: MapMarker[];
  submaps: SubmapPin[];
  createdAt: Date;
  updatedAt: Date;
}
