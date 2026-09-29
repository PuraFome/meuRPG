import { Code, ConnectError } from '@connectrpc/connect';

import {
  CharacterBlockedReason as GenCharacterBlockedReason,
  CharacterBlockedSchema,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { describeConnectError } from '../connect/connect-errors';
import { CharacterBlockedReason } from './characters.types';

/**
 * Turns a `CharacterBlocked.reason` into the message the sheet and the
 * editor show as-is. Kept separate from `describeCharacterError` so both can
 * be unit-tested without a `ConnectError` in hand — this one takes the
 * already-decoded local reason, not a wire enum.
 */
export function characterBlockedMessage(reason: CharacterBlockedReason | undefined): string {
  switch (reason) {
    case 'sheet_locked':
      return 'A ficha está travada porque a campanha já começou a jogar. Só o mestre pode editá-la agora.';
    case 'character_dead':
      return 'Esse personagem está morto e a ficha não pode mais ser editada.';
    case 'living_character_exists':
      return 'Você já tem um personagem vivo nesta campanha.';
    case 'story_locked':
      return 'O mestre ainda não liberou a edição da história. Peça para ele liberar em "Permitir editar a história".';
    default:
      return 'Não foi possível concluir a ação agora.';
  }
}

/** Maps the wire `CharacterBlockedReason` enum (characters.proto) onto the
 * local, UI-facing union `characterBlockedMessage` reads. `UNSPECIFIED` and
 * any future value this app does not know about yet fall through to
 * `undefined`, which `characterBlockedMessage` already turns into a safe
 * generic message instead of throwing. */
function mapBlockedReason(reason: GenCharacterBlockedReason | undefined): CharacterBlockedReason | undefined {
  switch (reason) {
    case GenCharacterBlockedReason.SHEET_LOCKED:
      return 'sheet_locked';
    case GenCharacterBlockedReason.CHARACTER_DEAD:
      return 'character_dead';
    case GenCharacterBlockedReason.LIVING_CHARACTER_EXISTS:
      return 'living_character_exists';
    case GenCharacterBlockedReason.STORY_LOCKED:
      return 'story_locked';
    default:
      return undefined;
  }
}

/**
 * Maps any `CharacterService` error to a message a form can show as-is.
 *
 * For `failed_precondition`, this reads the `CharacterBlocked` detail off
 * the error itself (`findDetails(CharacterBlockedSchema)`) — the caller
 * never needs to guess or pass a reason in.
 */
export function describeCharacterError(err: unknown): string {
  const connectErr = ConnectError.from(err, Code.Unavailable);

  if (connectErr.code === Code.FailedPrecondition) {
    const [detail] = connectErr.findDetails(CharacterBlockedSchema);
    return characterBlockedMessage(mapBlockedReason(detail?.reason));
  }
  if (connectErr.code === Code.Aborted) {
    // AIP-154-style stale revision: someone else (the player, the master,
    // or the server locking the sheet) saved first.
    return 'A ficha mudou enquanto você editava. Recarregue a página e tente de novo.';
  }

  return describeConnectError(connectErr, {
    [Code.PermissionDenied]: 'Você não tem permissão para fazer isso.',
    [Code.NotFound]: 'Personagem não encontrado.',
    [Code.InvalidArgument]: 'Confira os campos da ficha.',
  });
}
