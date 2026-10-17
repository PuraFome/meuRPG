import { EncounterBlockedReason } from '../../../gen/meurpg/play/v1/combat_pb';

/** The refusals of what a class or a table maneuver asks of the turn: what happened and what to do instead. */
export const CLASS_BLOCKED_TEXT: ReadonlyMap<EncounterBlockedReason, string> = new Map([
  [
    EncounterBlockedReason.RECKLESS_TOO_LATE,
    'O ataque descuidado se declara no primeiro ataque do turno, e você já atacou neste turno.',
  ],
  [
    EncounterBlockedReason.FRENZY_UNAVAILABLE,
    'Só o bárbaro do Caminho do Berserker tem frenesi. Entre em fúria sem ele.',
  ],
  [
    EncounterBlockedReason.NO_MELEE_HIT,
    'Essa manobra pede um acerto corpo a corpo com arma neste turno, e você ainda não acertou.',
  ],
]);
