import type { Combatant } from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  CombatantStateKind,
  type CombatantEffect,
  type ConditionSource,
  StatePhase,
} from '../../../../../gen/meurpg/play/v1/combat_rolls_pb';
import { conditionName } from '../../../../core/combat/conditions';

/** The chip of a state: its word ("Em fúria", "Marcado por Brisa") and what it does, which a keyboard or a screen reader reaches. */
export interface StateChip {
  readonly id: string;
  readonly text: string;
  readonly description: string;
}

/** "até o fim do turno de Kai", "até o início do turno de Kai", or nothing when it ends with something else. */
export function endsText(
  endsId: string,
  phase: StatePhase,
  people: readonly Pick<Combatant, 'id' | 'label'>[],
): string {
  const who = people.find((c) => c.id === endsId)?.label;
  if (!who) {
    return '';
  }
  return `até ${phase === StatePhase.START_OF_TURN ? 'o início' : 'o fim'} do turno de ${who}`;
}

export function stateChip(
  s: CombatantEffect,
  people: readonly Pick<Combatant, 'id' | 'label'>[],
): StateChip {
  const marked = s.kind === CombatantStateKind.HUNTERS_MARK_TARGET && s.sourceLabel;
  const ends = s.endsCombatantId ? endsText(s.endsCombatantId, s.endsPhase, people) : '';
  const round = s.endsRound > 0 ? `até a rodada ${s.endsRound}` : '';
  const text = marked ? `${s.labelPt} por ${s.sourceLabel}` : s.labelPt;
  const until = ends || round;
  const description = [
    `${marked ? text : s.labelPt}.`,
    s.effectPt,
    until ? `${until[0].toUpperCase()}${until.slice(1)}.` : '',
  ]
    .filter(Boolean)
    .join(' ');
  return { id: s.id, text, description };
}

/** "por Kai, até o fim do turno de Kai" for the tag of a condition that has a source, or an empty string. */
export function conditionSourceText(
  tag: string,
  sources: readonly ConditionSource[],
  people: readonly Pick<Combatant, 'id' | 'label'>[],
): string {
  const found = sources.find((s) => conditionName(s.conditionKey) === tag);
  if (!found) {
    return '';
  }
  const ends = found.endsCombatantId
    ? endsText(found.endsCombatantId, found.endsPhase, people)
    : '';
  return [`por ${found.sourceLabel}`, ends].filter(Boolean).join(', ');
}
