import { EncounterBlockedReason } from '../../../gen/meurpg/play/v1/combat_pb';

/** The refusals of the legendary actions (#557, the master's alone): what was wrong, and that nothing was spent. */
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
]);
