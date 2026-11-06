import { ReactionKind } from '../../../gen/meurpg/play/v1/combat_pb';

/** The names of the reactions, as `names_pt.json` writes them. */
export const REACTION_NAMES = {
  shield: 'Escudo Arcano',
  uncannyDodge: 'Esquiva Sobrenatural',
  hellishRebuke: 'Repreensão Infernal',
  counterspell: 'Contramágica',
  cuttingWords: 'Palavras de Interrupção',
  deflectMissiles: 'Defletir Projéteis',
  featherFall: 'Queda Suave',
  slowFall: 'Queda Lenta',
  protection: 'Proteção',
} as const;

/** The name of a kind of reaction ("Reação" for one the table has no name for). */
export function reactionKindName(kind: ReactionKind): string {
  switch (kind) {
    case ReactionKind.SHIELD:
      return REACTION_NAMES.shield;
    case ReactionKind.UNCANNY_DODGE:
      return REACTION_NAMES.uncannyDodge;
    case ReactionKind.HELLISH_REBUKE:
      return REACTION_NAMES.hellishRebuke;
    case ReactionKind.COUNTERSPELL:
      return REACTION_NAMES.counterspell;
    case ReactionKind.CUTTING_WORDS:
      return REACTION_NAMES.cuttingWords;
    case ReactionKind.DEFLECT_MISSILES:
      return REACTION_NAMES.deflectMissiles;
    case ReactionKind.MANEUVER_REDUCE:
      return 'Manobra';
    case ReactionKind.PROTECTION:
      return REACTION_NAMES.protection;
    case ReactionKind.FEATHER_FALL:
      return REACTION_NAMES.featherFall;
    case ReactionKind.SLOW_FALL:
      return REACTION_NAMES.slowFall;
    case ReactionKind.CONCENTRATION_SAVE:
      return 'Teste de concentração';
    case ReactionKind.OPPORTUNITY:
      return 'Ataque de oportunidade';
    default:
      return 'Reação';
  }
}

/** The part of a combatant that says which reaction it spent and when (the master's, the owner's and a joint turn's). */
export interface ReactionRecord {
  readonly reactionKind?: ReactionKind;
  readonly reactionRound?: number;
}

/**
 * What a spent reaction says of itself: "Escudo Arcano, rodada 2". The kind is left out when the server names none (an
 * opportunity attack has its own name; a feature's reaction has none) and the round when it sent none; `''` for neither.
 */
export function reactionRecordText(c: ReactionRecord): string {
  const kind = c.reactionKind ?? ReactionKind.UNSPECIFIED;
  const parts: string[] = [];
  if (kind !== ReactionKind.UNSPECIFIED) {
    parts.push(reactionKindName(kind));
  }
  if ((c.reactionRound ?? 0) > 0) {
    parts.push(`rodada ${c.reactionRound}`);
  }
  return parts.join(', ');
}

/** "Reação usada (Escudo Arcano, rodada 2)": the words of a spent reaction, with its record when there is one. */
export function reactionUsedText(c: ReactionRecord): string {
  const record = reactionRecordText(c);
  return record ? `Reação usada (${record})` : 'Reação usada';
}
