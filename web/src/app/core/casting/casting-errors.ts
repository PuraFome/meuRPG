import { Code, ConnectError } from '@connectrpc/connect';

import {
  CastingBlockedReason,
  type CastingBlocked,
  CastingBlockedSchema,
} from '../../../gen/meurpg/play/v1/casting_pb';
import { EncounterBlockedReason } from '../../../gen/meurpg/play/v1/combat_pb';
import { sessionClosed, encounterBlocked } from '../combat/combat-errors';
import { describeConnectError } from '../connect/connect-errors';
import { metersText } from '../units';

/** The typed detail of a `failed_precondition` from `CastingService`, or `null`. Never read from the message. */
export function castingBlocked(err: unknown): CastingBlocked | null {
  const connectErr = ConnectError.from(err, Code.Unavailable);
  if (connectErr.code !== Code.FailedPrecondition) {
    return null;
  }
  return connectErr.findDetails(CastingBlockedSchema)[0] ?? null;
}

/** What a refusal of a cast says, in Portuguese, from its code and its typed reason (never from the server's message). */
export function castingErrorText(err: unknown): string {
  const blocked = castingBlocked(err);
  if (blocked) {
    return blockedText(blocked);
  }
  if (sessionClosed(err)) {
    return 'A sessão já terminou.';
  }
  const encounter = encounterBlocked(err);
  if (encounter?.reason === EncounterBlockedReason.WILD_SHAPE_NO_SPELLS) {
    return 'Em forma de fera, o druida não conjura magias.';
  }
  if (encounter?.reason === EncounterBlockedReason.WRONG_DICE_MODE) {
    return 'A campanha mudou a forma de rolar os dados. Recarregue a página.';
  }
  if (encounter) {
    return 'Esta magia não pode ser conjurada agora.';
  }
  return describeConnectError(err, {
    [Code.InvalidArgument]: 'Esta conjuração não vale: confira a magia, o espaço e os alvos.',
    [Code.NotFound]: 'Não encontramos essa conjuração, o personagem ou o alvo.',
    [Code.PermissionDenied]: 'Só o jogador do personagem ou o mestre pode fazer isso.',
  });
}

function blockedText(b: CastingBlocked): string {
  switch (b.reason) {
    case CastingBlockedReason.IN_COMBAT:
      return 'O personagem está em um combate: conjure por lá.';
    case CastingBlockedReason.CAST_IN_PROGRESS:
      return 'O personagem já está conjurando: conclua ou pare a conjuração antes.';
    case CastingBlockedReason.NO_SLOT:
      return b.minLevel > 0
        ? `Não há espaço de magia livre de ${b.minLevel}º nível ou maior.`
        : 'Não há espaço de magia livre.';
    case CastingBlockedReason.TARGET_WEARS_ARMOR:
      return 'Está de armadura: a Armadura Arcana não funciona.';
    case CastingBlockedReason.CAST_NOT_GOING:
      return 'Essa conjuração já não está em andamento.';
    case CastingBlockedReason.CAST_NOT_ACTIVE:
      return 'A magia ainda está sendo conjurada: pare a conjuração.';
    case CastingBlockedReason.TARGET_OUT_OF_REACH:
      return `Um alvo está fora do alcance${b.missingFt > 0 ? ` (faltam ${metersText(b.missingFt)})` : ''}.`;
    case CastingBlockedReason.CLASS_CANNOT_RITUAL:
      return 'A classe do personagem não conjura essa magia como ritual.';
    case CastingBlockedReason.NOT_A_RITUAL:
      return 'Só magias com a etiqueta "Ritual" podem ser conjuradas assim.';
    default:
      return 'Esta magia não pode ser conjurada agora.';
  }
}
