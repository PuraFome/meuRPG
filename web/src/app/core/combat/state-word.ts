import { CombatantState } from '../../../gen/meurpg/play/v1/combat_pb';
import { article } from '../format/article';

/** How hurt an NPC is, as a word a player may read (RN-20), and where a
 * player's character is when it is at 0 hit points: "Caído", "Estável",
 * "Morrendo" (only the master gets that one) or "Morto". The word of a
 * person has a gender, taken from the name the way `article` does
 * ("Brisa" is "Caída"). */
export function stateWord(state: CombatantState, label = ''): string {
  const feminine = label !== '' && article(label) === 'a';
  switch (state) {
    case CombatantState.UNHURT:
      return 'Ileso';
    case CombatantState.HURT:
      return 'Ferido';
    case CombatantState.BADLY_HURT:
      return 'Muito ferido';
    case CombatantState.DEFEATED:
      return 'Derrotado';
    case CombatantState.DOWN:
      return feminine ? 'Caída' : 'Caído';
    case CombatantState.DYING:
      return 'Morrendo';
    case CombatantState.STABLE:
      return 'Estável';
    case CombatantState.DEAD:
      return feminine ? 'Morta' : 'Morto';
    default:
      return '';
  }
}
