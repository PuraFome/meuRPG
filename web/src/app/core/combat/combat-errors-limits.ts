import { EncounterBlockedReason } from '../../../gen/meurpg/play/v1/combat_pb';

/** The refusals of an action of a stat block that is spent (the master's alone): what is wrong, and that nothing was spent. */
export const LIMITED_ACTION_BLOCKED_TEXT: ReadonlyMap<EncounterBlockedReason, string> = new Map([
  [
    EncounterBlockedReason.CREATURE_ACTION_RECHARGING,
    'Essa ação da criatura ainda não recarregou. Nada foi gasto.',
  ],
  [
    EncounterBlockedReason.CREATURE_ACTION_NO_USES_LEFT,
    'Essa ação da criatura não tem mais usos. Nada foi gasto.',
  ],
]);
