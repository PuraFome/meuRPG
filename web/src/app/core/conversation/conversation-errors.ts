import { Code, ConnectError } from '@connectrpc/connect';

import {
  GameSessionBlockedReason,
  GameSessionBlockedSchema,
} from '../../../gen/meurpg/play/v1/play_pb';
import { describeConnectError } from '../connect/connect-errors';

function noOpenSession(err: unknown): boolean {
  const e = ConnectError.from(err, Code.Unavailable);
  return (
    e.code === Code.FailedPrecondition &&
    e.findDetails(GameSessionBlockedSchema)[0]?.reason === GameSessionBlockedReason.NO_OPEN_SESSION
  );
}

/** What a line that did not go says under it (`SendSessionMessage` fails with
 * these codes, see play.proto). Always starts with "Não foi enviada." because the
 * line stays on screen: the reason is what the person can act on. */
export function sendErrorMessage(err: unknown): string {
  const reason = noOpenSession(err)
    ? 'A sessão acabou: a conversa só funciona durante a sessão.'
    : describeConnectError(err, {
        [Code.Unavailable]: 'A conexão caiu.',
        [Code.InvalidArgument]: 'A mensagem precisa ter de 1 a 2.000 caracteres.',
        [Code.PermissionDenied]: 'Você não pode falar assim agora.',
        [Code.NotFound]: 'Quem fala ou quem lê mudou. Recarregue a página.',
        [Code.ResourceExhausted]: 'Muitas mensagens em pouco tempo. Espere um instante.',
      });
  return `Não foi enviada. ${reason}`;
}

/** What the panel says when it could not read the lines (`ListSessionMessages`). */
export function readErrorMessage(err: unknown): string {
  if (noOpenSession(err)) {
    return 'Não há sessão aberta, então ainda não há conversa.';
  }
  return describeConnectError(err, {
    [Code.Unavailable]: 'Não deu para ler a conversa: o servidor não respondeu.',
    [Code.NotFound]: 'Essa conversa não existe mais, ou você não a vê. Recarregue a página.',
  });
}
