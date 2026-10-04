import { CharacterState } from '../../../core/characters/characters.types';
import { CampaignCharacterListItemVm } from './campaign-characters.types';

/**
 * The second line of a character's row: "Mago 3, de Vinicius" on the
 * master's lists, just "Mago 3" on a player's own. A player with no display
 * name reads "de um jogador sem nome" (never "Sem nome"). Empty when there
 * is nothing to say (a basic-sheet NPC has no class).
 */
export function characterRowSub(c: CampaignCharacterListItemVm, isMaster: boolean): string {
  const parts: string[] = [];
  if (c.classSummary) {
    parts.push(c.classSummary);
  }
  if (isMaster && c.kind === 'player') {
    // No-break spaces: the line never ends on an orphan word ("Mago 4, de um jogador sem" / "nome").
    parts.push(c.playerDisplayName ? `de ${c.playerDisplayName}`.replace(/ /g, '\u00a0') : 'de\u00a0um\u00a0jogador\u00a0sem\u00a0nome');
  }
  return parts.join(', ');
}

/** The tag's tone for a character's state (docs/design.md#cor): pending in
 * the warning colour, dead in the danger colour, draft and locked neutral. */
export function stateTagClass(state: CharacterState): string {
  switch (state) {
    case 'pending':
      return 'mr-tag--pending';
    case 'dead':
      return 'mr-tag--danger';
    default:
      return '';
  }
}
