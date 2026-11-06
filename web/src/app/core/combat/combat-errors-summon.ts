import { EncounterBlockedReason } from '../../../gen/meurpg/play/v1/combat_pb';

/** The refusals of casting a spell that needs something placed (a summon's creatures, a wall's shape): what was wrong
 * and what to do instead. */
export const SUMMON_BLOCKED_TEXT: ReadonlyMap<EncounterBlockedReason, string> = new Map([
  [
    EncounterBlockedReason.SUMMON_NEEDS_INITIATIVE,
    'Falta o d20 da iniciativa das criaturas. Se cada uma rola a sua, role no app.',
  ],
  [
    EncounterBlockedReason.SUMMON_CHOICE_INVALID,
    'Essa escolha não vale para o espaço que você usou. Mude a quantidade, a criatura ou o espaço e tente de novo.',
  ],
  [
    EncounterBlockedReason.WALL_PLACEMENT_REQUIRED,
    'Esta magia cria uma muralha: falta dizer a forma, o tamanho e o lado que causa dano. Nada foi gasto.',
  ],
]);
