import { Code, ConnectError } from '@connectrpc/connect';

import {
  CharacterBlockedReason,
  CharacterBlockedSchema,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import {
  EncounterBlockedReason,
  EncounterBlockedSchema,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { sessionClosed } from '../combat/combat-errors';
import { circleLabel } from '../combat/combat-grid';
import { describeConnectError } from '../connect/connect-errors';

/** What the person was doing, to finish "Não deu para …". */
export type CreatureAction = 'give' | 'rename' | 'dismiss' | 'adjust' | 'cast' | 'read';

const WHAT: Record<CreatureAction, string> = {
  give: 'dar a criatura',
  rename: 'trocar o nome',
  dismiss: 'dispensar a criatura',
  adjust: 'corrigir os PV',
  cast: 'conjurar a magia',
  read: 'abrir as criaturas',
};

/**
 * The Portuguese message for a failed call of the creatures (MR-037): what
 * happened and how to fix it, by code and by typed detail, never by the
 * message. `CastSummon`'s refusals come as an `EncounterBlocked` or a
 * `GameSessionBlocked`; `GiveCreature` and the correction as a
 * `CharacterBlocked`.
 */
export function creatureErrorMessage(err: unknown, action: CreatureAction): string {
  const connectErr = ConnectError.from(err, Code.Unavailable);
  if (connectErr.code === Code.FailedPrecondition) {
    if (sessionClosed(err)) {
      return 'A sessão acabou: só dá para conjurar durante uma sessão.';
    }
    const blocked = connectErr.findDetails(EncounterBlockedSchema)[0];
    if (blocked) {
      switch (blocked.reason) {
        case EncounterBlockedReason.CASTING_TIME_TOO_LONG:
          return 'Esta magia leva mais tempo do que um combate dá. Conjure fora do combate.';
        case EncounterBlockedReason.SUMMON_CHOICE_INVALID:
          return 'Essa escolha não vale para o espaço que você usou. Mude a forma ou o espaço e tente de novo.';
        case EncounterBlockedReason.SUMMON_IN_COMBAT:
          return 'Há um combate em andamento. Conjure pela sua vez, na tela do combate.';
        case EncounterBlockedReason.WILD_SHAPE_NO_SPELLS:
          return 'Na Forma Selvagem não dá para conjurar. Volte à forma normal e tente de novo.';
        case EncounterBlockedReason.NO_SLOT:
          return blocked.minLevel > 0
            ? `Não há espaço de ${circleLabel(blocked.minLevel)} ou maior livre.`
            : 'Não há espaço de magia livre.';
        default:
          return 'Não dá para fazer isso agora. Feche esta folha, olhe a ficha e tente de novo.';
      }
    }
    const reason = connectErr.findDetails(CharacterBlockedSchema)[0]?.reason;
    if (reason === CharacterBlockedReason.CREATURE_LIMIT) {
      return 'O personagem já tem 40 criaturas. Dispense uma antes.';
    }
    if (reason === CharacterBlockedReason.CREATURE_IN_COMBAT) {
      return 'A criatura está num combate. Corrija os PV dela pela tela do combate.';
    }
    if (reason === CharacterBlockedReason.CHARACTER_DEAD) {
      return 'Esse personagem está morto.';
    }
  }
  return describeConnectError(connectErr, {
    [Code.InvalidArgument]:
      action === 'cast'
        ? 'A ficha não conjura essa magia desse jeito. Confira a forma, o nome e o espaço.'
        : `Não deu para ${WHAT[action]}: confira o nome (1 a 40 letras, numa linha só) e tente de novo.`,
    [Code.NotFound]: 'Essa criatura não existe mais, ou você não a vê. Recarregue a página.',
    [Code.PermissionDenied]: 'Você não pode fazer isso.',
    [Code.Aborted]: 'Algo mudou enquanto você agia. Tente de novo.',
    [Code.Unavailable]: `Não deu para ${WHAT[action]}: o servidor não respondeu. Tente de novo.`,
  });
}

/** Whether a list read failed because the viewer may not see the creatures (RN-20: another player's). */
export function creaturesHidden(err: unknown): boolean {
  return ConnectError.from(err, Code.Unavailable).code === Code.NotFound;
}
