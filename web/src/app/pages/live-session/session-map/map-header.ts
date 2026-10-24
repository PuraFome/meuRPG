import { CharacterKind } from '../../../../gen/meurpg/characters/v1/characters_pb';
import type { ViewToken } from '../../../shared/map-view/map-geometry';

/** One token shape of the legend under the master's map (C6-05): the token as the map draws it, and its name. */
export interface TokenSample {
  readonly label: string;
  readonly initial: string;
  readonly token: ViewToken;
}

const sample = (label: string, initial: string, over: Partial<ViewToken>): TokenSample => ({
  label,
  initial,
  token: { characterId: label, name: label, mine: false, xBp: 0, yBp: 0, hidden: false, ...over },
});

/** "Personagem", "NPC", "Escondido (só você vê)" and "Vez": the four shapes the master's map uses. */
export const TOKEN_SAMPLES: readonly TokenSample[] = [
  sample('Personagem', 'P', { kind: CharacterKind.PLAYER }),
  sample('NPC', 'G1', { kind: CharacterKind.STORY }),
  sample('Escondido (só você vê)', 'G1', { kind: CharacterKind.STORY, hidden: true }),
  sample('Vez', 'P', { kind: CharacterKind.PLAYER, mine: true }),
];

/** "24 × 16 · 1,5 m": the grid's squares across and down, and the size of one; empty for a map with no grid. */
export function gridText(columns: number, rows: number): string {
  return columns > 0 && rows > 0 ? `${columns} × ${rows} · 1,5 m` : '';
}
