import { MapPointKind, TrapState } from '../../../gen/meurpg/maps/v1/maps_pb';

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

/** What decides whether the players know a point (the generated `MapPoint` fits it). */
export interface PointVisibility {
  readonly kind: number;
  readonly revealed: boolean;
  /** A treasure marked found (everyone sees it then, "Todos veem o tesouro"). */
  readonly treasureFoundAt?: unknown;
  /** A trap shown to some characters. */
  readonly trapRevealedTo?: readonly unknown[];
  /** A trap: a Disparada one is known to everyone who sees the map. */
  readonly trap?: { readonly state: number } | undefined;
}

/**
 * Whether the map draws a point as hidden (a dashed edge, the crossed eye, "Escondido"): the players do not know it yet. A Luz is the
 * master's alone, so it is never "escondida", only his; a treasure found, a trap revealed to a character or already fired is known, whatever its `revealed` flag says.
 */
export function pointHidden(p: PointVisibility): boolean {
  if (p.kind === MapPointKind.LIGHT || p.revealed) {
    return false;
  }
  if (p.kind === MapPointKind.TREASURE) {
    return p.treasureFoundAt === undefined;
  }
  if (p.kind === MapPointKind.TRAP) {
    const state = p.trap?.state;
    return (p.trapRevealedTo?.length ?? 0) === 0 && state !== TrapState.TRIGGERED;
  }
  return true;
}

/** The marker's accessible name: "Taverna do Javali, Cena de RP, escondido"; a Luz says "só você vê". */
export function pointAriaLabel(point: PointVisibility & { name: string }): string {
  const state = point.kind === MapPointKind.LIGHT ? ', só você vê' : pointHidden(point) ? ', escondido' : '';
  return `${point.name}, ${pointKindLabel(point.kind)}${state}`;
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
