import { MapPointKind } from '../../../gen/meurpg/maps/v1/maps_pb';

/** "Batalha", "Submapa", "Cena de RP": what a kind is called on screen. */
export function pointKindLabel(kind: number): string {
  switch (kind) {
    case MapPointKind.BATTLE:
      return 'Batalha';
    case MapPointKind.SUBMAP:
      return 'Submapa';
    case MapPointKind.SCENE:
      return 'Cena de RP';
    case MapPointKind.TRAP:
      return 'Armadilha';
    case MapPointKind.TREASURE:
      return 'Tesouro';
    case MapPointKind.LIGHT:
      return 'Luz';
    default:
      return 'Ponto';
  }
}

/** The kind's glyph (Material Symbols): crossed swords, stairs, a speech
 * bubble (README-B, "Map markers"). */
export function pointKindIcon(kind: number): string {
  switch (kind) {
    case MapPointKind.BATTLE:
      return 'swords';
    case MapPointKind.SUBMAP:
      return 'stairs';
    case MapPointKind.TRAP:
      return 'warning';
    case MapPointKind.TREASURE:
      return 'inventory_2';
    case MapPointKind.LIGHT:
      return 'lightbulb';
    default:
      return 'chat_bubble';
  }
}

/** The marker's accessible name: "Taverna do Javali, Cena de RP, escondido". */
export function pointAriaLabel(point: { name: string; kind: number; revealed: boolean }): string {
  return `${point.name}, ${pointKindLabel(point.kind)}${point.revealed ? '' : ', escondido'}`;
}

/** What a token is, for the lists: "NPC, inimigo" (a player's character
 * shows its class line instead, from the party info). */
export function tokenKindLabel(kind: number): string {
  // CharacterKind: 1 player, 2 enemy, 3 boss, 4 minion, 5 story.
  switch (kind) {
    case 2:
      return 'NPC, inimigo';
    case 3:
      return 'NPC, boss';
    case 4:
      return 'NPC, minion';
    case 5:
      return 'NPC de história';
    default:
      return 'Personagem de jogador';
  }
}
