import {
  CastEffect,
  type OutsideCast,
  OutsideCastStatus,
} from '../../../gen/meurpg/play/v1/casting_pb';
import { CombatantKind, type Encounter } from '../../../gen/meurpg/play/v1/combat_pb';

const MAGE_ARMOR_KEY = 'spell:mage-armor';

/** How many times the combat is read again when a read was dropped for a change that came while it was out. */
export const COMBAT_REREADS = 3;

/** How long the page waits before trying a failed combat read once more, in milliseconds. */
export const COMBAT_RETRY_MS = 1500;

/** How long a burst of vision hints waits to become one read, in milliseconds. */
export const VISION_DEBOUNCE_MS = 120;

/** The armor class the character's lasting Armadura Arcana gives, the best one; 0 without it. */
export function mageArmorClassOf(casts: readonly OutsideCast[], characterId: string): number {
  let best = 0;
  for (const cast of casts) {
    if (cast.spellKey !== MAGE_ARMOR_KEY || cast.status !== OutsideCastStatus.ACTIVE) {
      continue;
    }
    for (const t of cast.targets) {
      if (t.characterId === characterId && t.effect === CastEffect.ARMOR_CLASS) {
        best = Math.max(best, t.armorClass);
      }
    }
  }
  return best;
}

/** The line under the players' "O combate acabou" title: the name, the rounds and how many enemies fell. */
export function highlightsSubOf(e: Encounter | null): string {
  if (!e) {
    return '';
  }
  const rounds = Math.max(1, e.round ?? 1);
  const npcs = e.combatants.filter((c) => c.kind !== CombatantKind.PLAYER);
  const defeated = npcs.filter((c) => c.defeated).length;
  const beaten = npcs.length > 0 ? ` · ${defeated} de ${npcs.length} derrotados` : '';
  return `${e.name} · ${rounds} ${rounds === 1 ? 'rodada' : 'rodadas'}${beaten}`;
}

/** What a screen reader hears of the "Anotações" button: how many clues are new. */
export function notesAria(fresh: number): string {
  return fresh > 0 ? `Anotações, ${fresh} ${fresh === 1 ? 'nova' : 'novas'}` : 'Anotações';
}
