import { CharacterKind } from '../../../gen/meurpg/characters/v1/characters_pb';
import { combatantInitial } from '../combat/combat-view';
import { type ViewToken, tokenInitial } from '../../shared/map-view/map-geometry';

/** The letter on a token as the map language draws it (docs/design.md): an NPC's own number ("G2", "C"), anyone else's initial. */
export function mapTokenInitial(token: ViewToken, all: readonly ViewToken[]): string {
  const npc =
    token.kind !== undefined &&
    token.kind !== CharacterKind.PLAYER &&
    token.kind !== CharacterKind.UNSPECIFIED &&
    !token.creatureId;
  return npc ? combatantInitial(token.name) : tokenInitial(token, all);
}
