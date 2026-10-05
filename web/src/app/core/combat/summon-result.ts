import type { Combatant, Encounter } from '../../../gen/meurpg/play/v1/combat_pb';
import { tight } from '../format/text';
import { article } from './combat-log';
import { groupFeminine, pluralName } from './creature-names';
import { npcGroupOf, npcPlural } from './joint-turn';

/**
 * What the player reads after Conjurar Animais in a combat (MR-037, E9-12 state 2): "Os 2 Lobos atrozes entram no
 * combate com iniciativa 10 (um d20 para os dois: 8 + 2). Eles agem juntos, depois dos Goblins." The numbers are
 * the combat's own (the group's total, the d20 and the bonus the server rolled or took); the words are ours.
 */
export interface SummonResult {
  readonly text: string;
  /** The creatures that joined, in the order of the choice. */
  readonly creatures: readonly Combatant[];
}

/** "depois do Capitão Goblin", "depois dos Goblins", "antes de todos": who plays right before the group. */
export function afterWhom(e: Encounter, members: readonly Combatant[]): string {
  const at = Math.min(...members.map((m) => e.combatants.findIndex((c) => c.id === m.id)));
  if (!Number.isFinite(at) || at <= 0) {
    return 'antes de todos';
  }
  const inGroup = new Set(members.map((m) => m.id));
  for (let i = at - 1; i >= 0; i--) {
    const c = e.combatants[i];
    if (inGroup.has(c.id) || c.defeated) {
      continue;
    }
    const group = npcGroupOf(e, c.id);
    const named = group ? npcPlural(group.map((m) => m.label)) : null;
    return named ? `depois d${named}` : `depois ${article(c.label) === 'a' ? 'da' : 'do'} ${c.label}`;
  }
  return 'antes de todos';
}

/** The sentence and the creatures of a casting's result, from the combat the answer carries. */
export function summonResult(e: Encounter, ids: readonly string[]): SummonResult {
  const creatures = ids.flatMap((id) => e.combatants.find((c) => c.id === id) ?? []);
  if (creatures.length === 0) {
    return { text: '', creatures: [] };
  }
  const first = creatures[0];
  const several = creatures.length > 1;
  const f = groupFeminine(creatures);
  const sameKind = creatures.every((c) => c.monsterKey === first.monsterKey);
  const who = several
    ? `${f ? 'As' : 'Os'} ${creatures.length} ${sameKind && first.monsterNamePt ? pluralName(first.monsterNamePt) : 'criaturas'}`
    : `${f ? 'A' : 'O'} ${first.label}`;
  const total = first.initiative;
  const together = creatures.length === 2 ? 'um d20 para os dois' : 'um d20 para todos';
  const roll =
    first.initiativeFace !== undefined && first.initiativeBonus !== undefined
      ? ` (${several ? together : 'um d20'}: ${first.initiativeFace} ${first.initiativeBonus < 0 ? '−' : '+'} ${Math.abs(first.initiativeBonus)})`
      : '';
  const enters = `${who} ${several ? 'entram' : 'entra'} no combate${total === undefined ? '' : ` com iniciativa ${total}`}${roll}.`;
  const acts = several ? ` Eles agem juntos, ${afterWhom(e, creatures)}.` : ` Ele age ${afterWhom(e, creatures)}.`;
  return { text: tight(enters + acts), creatures };
}
