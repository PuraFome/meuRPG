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

export function characterTypeLabel(type: CharacterType): string {
  return CHARACTER_TYPE_LABELS[type] ?? type;
}
