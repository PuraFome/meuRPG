import { EncounterBlockedReason } from '../../../gen/meurpg/play/v1/combat_pb';

/** The refusals of the legendary actions and of Legendary Resistance (the master's alone): what was wrong, and that nothing was spent. */
export const LEGENDARY_BLOCKED_TEXT: ReadonlyMap<EncounterBlockedReason, string> = new Map([
  [
    EncounterBlockedReason.LEGENDARY_OWN_TURN,
    'Ações lendárias se usam no fim do turno de outra criatura, nunca no turno dela. Nada foi gasto.',
  ],
  [
    EncounterBlockedReason.LEGENDARY_NONE_LEFT,
    'A criatura não tem ações lendárias suficientes para essa opção. Nada foi gasto.',
  ],
  [
    EncounterBlockedReason.LEGENDARY_OPTION_UNKNOWN,
    'Essa opção não é uma das ações lendárias da criatura. A tela foi atualizada; nada foi gasto.',
  ],
  [
    EncounterBlockedReason.LEGENDARY_WRONG_KIND,
    'Essa opção se usa de outro jeito: ataques pelo ataque e movimentos pelo movimento. Nada foi gasto.',
  ],
  [
    EncounterBlockedReason.LEGENDARY_UNAVAILABLE,
    'A criatura não tem ações lendárias para usar agora: ela não as tem, caiu ou está incapacitada. Nada foi gasto.',
  ],
  [
    EncounterBlockedReason.LEGENDARY_RESISTANCE_NONE_LEFT,
    'A criatura não tem mais usos de Resistência Lendária. O teste continua como foi rolado; nada foi gasto.',
  ],
  [
    EncounterBlockedReason.LEGENDARY_RESISTANCE_UNAVAILABLE,
    'Só uma criatura com Resistência Lendária pode usá-la. Nada foi gasto.',
  ],
]);
