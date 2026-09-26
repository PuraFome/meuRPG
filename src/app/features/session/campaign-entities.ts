import type { CampaignFolder } from '../../core/models/campaign';
import type { Character } from '../../core/models/character';
import type { MapData } from '../../core/models/map';

export type CharacterType = Character['type'];

export interface CharacterGroup {
  type: CharacterType;
  label: string;
  characters: Character[];
}

const CHARACTER_TYPE_LABELS: Record<CharacterType, string> = {
  player: 'Jogadores',
  npc: 'NPCs',
  boss: 'Chefes',
  minion: 'Capangas',
};

const CHARACTER_TYPE_ORDER: CharacterType[] = ['player', 'npc', 'boss', 'minion'];

/** Ids da pasta e de todas as suas descendentes. */
export function collectSubtreeFolderIds(
  folders: CampaignFolder[],
  rootId: string,
): Set<string> {
  const ids = new Set<string>([rootId]);
  let changed = true;
  while (changed) {
    changed = false;
    for (const folder of folders) {
      if (folder.parentId && ids.has(folder.parentId) && !ids.has(folder.id)) {
        ids.add(folder.id);
        changed = true;
      }
    }
  }
  return ids;
}

export interface CampaignEntities {
  maps: MapData[];
  characters: Character[];
}

/** Mapas e personagens associados à campanha (pasta + subpastas), alfabéticos. */
export function gatherCampaignEntities(
  folders: CampaignFolder[],
  allMaps: MapData[],
  allCharacters: Character[],
  rootId: string,
): CampaignEntities {
  const folderIds = collectSubtreeFolderIds(folders, rootId);
  const mapIds = new Set<string>();
  const characterIds = new Set<string>();

  for (const folder of folders) {
    if (!folderIds.has(folder.id)) continue;
    for (const id of folder.entityIds?.mapIds ?? []) mapIds.add(id);
    for (const id of folder.entityIds?.characterIds ?? []) characterIds.add(id);
  }

  return {
    maps: allMaps
      .filter((m) => mapIds.has(m.id))
      .sort((a, b) => a.name.localeCompare(b.name, 'pt-BR')),
    characters: allCharacters
      .filter((c) => characterIds.has(c.id))
      .sort((a, b) => a.name.localeCompare(b.name, 'pt-BR')),
  };
}

/** Agrupa personagens por tipo, na ordem de mesa, alfabéticos dentro do grupo. */
export function groupCharactersByType(characters: Character[]): CharacterGroup[] {
  return CHARACTER_TYPE_ORDER.map((type) => ({
    type,
    label: CHARACTER_TYPE_LABELS[type],
    characters: characters
      .filter((c) => c.type === type)
      .sort((a, b) => a.name.localeCompare(b.name, 'pt-BR')),
  })).filter((group) => group.characters.length > 0);
}

export interface SceneEntry {
  map: MapData;
  /** 0 = mapa principal; 1+ = submapa (nível de profundidade). */
  depth: number;
}

/**
 * Expande os mapas principais seguindo os vínculos de submapa (POIs e pins de
 * submapa), para o mestre poder trocar de cena sem associar cada submapa à
 * campanha. Percorre em largura, então os mapas principais vêm antes dos seus
 * submapas.
 */
export function collectScenes(
  allMaps: MapData[],
  rootIds: string[],
): SceneEntry[] {
  const byId = new Map(allMaps.map((map) => [map.id, map]));
  const entries: SceneEntry[] = [];
  const visited = new Set<string>();

  let level: { id: string; depth: number }[] = rootIds.map((id) => ({
    id,
    depth: 0,
  }));

  while (level.length > 0) {
    const next: { id: string; depth: number }[] = [];
    for (const { id, depth } of level) {
      if (visited.has(id)) continue;
      visited.add(id);
      const map = byId.get(id);
      if (!map) continue;
      entries.push({ map, depth });
      const children = [
        ...(map.markers ?? []).map((marker) => marker.targetMapId),
        ...(map.submaps ?? []).map((pin) => pin.targetMapId),
      ].filter((childId): childId is string => !!childId);
      for (const childId of children) {
        if (!visited.has(childId)) next.push({ id: childId, depth: depth + 1 });
      }
    }
    level = next;
  }

  return entries;
}

export function characterTypeLabel(type: CharacterType): string {
  return CHARACTER_TYPE_LABELS[type] ?? type;
}
