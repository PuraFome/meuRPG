import { Ability as GenAbility } from '../../../gen/meurpg/rules/v1/rules_pb';
import type { AbilityKey } from './characters.types';

/** The wire ability → its key. UNSPECIFIED reads as `'str'`; use `abilityKeyOf` where "none" must stay none. */
export const ABILITY_FROM_GEN: Record<GenAbility, AbilityKey> = {
  [GenAbility.UNSPECIFIED]: 'str',
  [GenAbility.STRENGTH]: 'str',
  [GenAbility.DEXTERITY]: 'dex',
  [GenAbility.CONSTITUTION]: 'con',
  [GenAbility.INTELLIGENCE]: 'int',
  [GenAbility.WISDOM]: 'wis',
  [GenAbility.CHARISMA]: 'cha',
};

/** The key of a wire ability, `null` for UNSPECIFIED. */
export function abilityKeyOf(ability: GenAbility): AbilityKey | null {
  return ability === GenAbility.UNSPECIFIED ? null : ABILITY_FROM_GEN[ability];
}
