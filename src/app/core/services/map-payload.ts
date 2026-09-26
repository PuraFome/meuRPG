import type { DungeonData, FogOfWar, MapData, MapGrid } from '../models/map';

function toPositiveInt(value: unknown, fallback: number): number {
  const parsed = typeof value === 'number' ? value : Number(value);
  return Number.isFinite(parsed) && parsed >= 1 ? Math.round(parsed) : fallback;
}

/**
 * Normaliza um mapa vindo da API: o CockroachDB devolve `integer` como string
 * (int de 64 bits) e o registro inclui campos só do backend (`userId`), então
 * o objeto precisa ser reconstruído no formato exato de `MapData`.
 */
export function normalizeMap(raw: Record<string, unknown>): MapData {
  const grid = (raw['grid'] as MapGrid | undefined) ?? undefined;
  return {
    id: String(raw['id']),
    name: String(raw['name'] ?? ''),
    description: (raw['description'] as string | null | undefined) ?? undefined,
    kind: (raw['kind'] as MapData['kind']) ?? undefined,
    backgroundImage:
      (raw['backgroundImage'] as string | null | undefined) ?? undefined,
    width: toPositiveInt(raw['width'], 1024),
    height: toPositiveInt(raw['height'], 768),
    layers: (raw['layers'] as MapData['layers']) ?? [],
    grid: grid ?? { cellSize: 50, columns: 20, rows: 15, visible: false },
    fogOfWar:
      (raw['fogOfWar'] as FogOfWar) ?? { explored: [], visible: false },
    markers: (raw['markers'] as MapData['markers']) ?? [],
    submaps: (raw['submaps'] as MapData['submaps']) ?? [],
    dungeon: (raw['dungeon'] as DungeonData) ?? { tiles: {}, objects: {} },
    createdAt: raw['createdAt'] as Date,
    updatedAt: raw['updatedAt'] as Date,
  };
}

/** Monta apenas os campos aceitos pelo DTO do servidor, com width/height numéricos. */
export function buildMapPayload(
  map: Partial<MapData>,
): Record<string, unknown> {
  const payload: Record<string, unknown> = {};
  if (map.id !== undefined) payload['id'] = map.id;
  if (map.name !== undefined) payload['name'] = map.name;
  if (map.description !== undefined) payload['description'] = map.description;
  if (map.kind !== undefined) payload['kind'] = map.kind;
  if (map.backgroundImage !== undefined)
    payload['backgroundImage'] = map.backgroundImage;
  if (map.width !== undefined) payload['width'] = toPositiveInt(map.width, 1024);
  if (map.height !== undefined)
    payload['height'] = toPositiveInt(map.height, 768);
  if (map.grid !== undefined) payload['grid'] = map.grid;
  if (map.fogOfWar !== undefined) payload['fogOfWar'] = map.fogOfWar;
  if (map.layers !== undefined) payload['layers'] = map.layers;
  if (map.markers !== undefined) payload['markers'] = map.markers;
  if (map.submaps !== undefined) payload['submaps'] = map.submaps;
  if (map.dungeon !== undefined) payload['dungeon'] = map.dungeon;
  return payload;
}
