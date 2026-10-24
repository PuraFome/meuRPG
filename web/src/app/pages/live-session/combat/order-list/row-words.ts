import {
  type Combatant,
  type Encounter,
  CombatantState,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import { isPlayer, stateWord } from '../../../../core/combat/combat-view';
import { isCreature, kindWord, ofOwner } from '../../../../core/combat/creature-names';
import { joinDots } from '../../../../core/format/text';
import type { CombatantInfo } from '../combat-info';

/** A player's character at 0 hit points that is still in the story. */
export function isDown(c: Combatant): boolean {
  return (
    c.state === CombatantState.DOWN ||
    c.state === CombatantState.DYING ||
    c.state === CombatantState.STABLE
  );
}

export function isDying(c: Combatant): boolean {
  return c.state === CombatantState.DYING;
}

/** "Caída", "Estável", "Morrendo · 3 falhas". */
export function downText(c: Combatant): string {
  return isDying(c)
    ? joinDots(['Morrendo', `${c.deathFailures} falhas`])
    : stateWord(c.state, c.label);
}

export function stateWordOf(c: Combatant): string {
  return stateWord(c.defeated && isPlayer(c) ? CombatantState.DEAD : c.state, c.label);
}

/** The one state word of a closed row, or `''`, by priority: "Derrotado", "Caída", "Morrendo · 3 falhas", "Escondido", "Concentração". */
export function rowState(c: Combatant): string {
  if (c.defeated) {
    return isPlayer(c) ? stateWordOf(c) : 'Derrotado';
  }
  if (isDown(c)) {
    return downText(c);
  }
  if (c.hidden) {
    return 'Escondido';
  }
  // The last of the words of a closed row, by priority: whoever holds a spell says it at a glance; the spell is in the opened row.
  return c.concentrationSpell ? 'Concentração' : '';
}

/** The class and the armor class of a row, without the kind label of an NPC or its challenge rating (the opened row says it). */
export function rowDetail(
  c: Combatant,
  encounter: Encounter,
  info: CombatantInfo | undefined,
  shielded: boolean,
): string {
  const ac = c.armorClass === undefined || shielded ? '' : `CA ${c.armorClass}`;
  if (isCreature(c)) {
    const kind = kindWord(c);
    return [kind === 'Criatura' ? '' : kind, ac, ofOwner(encounter, c)].filter(Boolean).join(' · ');
  }
  const first = isPlayer(c) ? (info?.classSummary ?? '') : '';
  return [first, ac].filter(Boolean).join(' · ');
}

/** The first line of an opened row (C7-02): "CA 13 · 14 de 24 PV", with the shield's bonus after the armor class and Ajuda's tag after the hit points. */
export function rowFacts(c: Combatant, shield: number, aidTag: string): string {
  const ac =
    c.armorClass === undefined ? '' : `CA ${c.armorClass}${shield > 0 ? ` + ${shield}` : ''}`;
  const hp =
    c.hitPointsMax === undefined
      ? ''
      : `${c.hitPointsCurrent} de ${c.hitPointsMax} PV${aidTag ? ` · ${aidTag}` : ''}`;
  return joinDots([ac, hp].filter(Boolean));
}
