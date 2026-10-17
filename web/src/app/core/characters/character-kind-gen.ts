import { CharacterKind as GenCharacterKind } from '../../../gen/meurpg/characters/v1/characters_pb';
import type { CharacterKind } from './characters.types';

/** The wire kind → its key. UNSPECIFIED reads as a player. */
export const KIND_FROM_GEN: Record<GenCharacterKind, CharacterKind> = {
  [GenCharacterKind.UNSPECIFIED]: 'player',
  [GenCharacterKind.PLAYER]: 'player',
  [GenCharacterKind.ENEMY]: 'enemy',
  [GenCharacterKind.BOSS]: 'boss',
  [GenCharacterKind.MINION]: 'minion',
  [GenCharacterKind.STORY]: 'story',
};
