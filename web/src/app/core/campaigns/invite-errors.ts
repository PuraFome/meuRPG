import { Code, ConnectError } from '@connectrpc/connect';

import { InviteState, InviteUnusableSchema } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { describeConnectError } from '../connect/connect-errors';

/**
 * Maps `AcceptInvite`'s errors (campaigns.proto) to a message the `/invite`
 * page can show as-is: `not_found` and `invalid_argument` (bad or missing
 * token), and `failed_precondition` with the `InviteUnusable` detail that
 * says exactly why the invite cannot be used — expired, revoked or used up.
 */
export function describeAcceptInviteError(err: unknown): string {
  const connectErr = ConnectError.from(err, Code.Unavailable);

  if (connectErr.code === Code.FailedPrecondition) {
    const [detail] = connectErr.findDetails(InviteUnusableSchema);
    return describeInviteUnusable(detail?.state);
  }

  return describeConnectError(connectErr, {
    [Code.NotFound]: 'Convite não encontrado. Confira se o link está completo.',
    [Code.InvalidArgument]: 'Link de convite inválido.',
    [Code.Unavailable]: 'Não foi possível falar com o servidor agora. Tente de novo em instantes.',
  });
}

function describeInviteUnusable(state: InviteState | undefined): string {
  switch (state) {
    case InviteState.EXPIRED:
      return 'Esse convite expirou.';
    case InviteState.REVOKED:
      return 'Esse convite foi revogado.';
    case InviteState.USED_UP:
      return 'Esse convite já foi usado. Se você não foi quem usou, peça um novo link ao mestre.';
    default:
      return 'Esse convite não pode mais ser usado.';
  }
}

/**
 * The `/invite/error?reason=<code>` codes the server redirects to once the
 * sign-in-through-invite flow (docs/arquitetura.md#frontend-web) cannot
 * accept the invite: `expired`, `revoked`, `used_up`, `not_found`,
 * `invalid` and `unavailable` (the database failed while accepting).
 */
export function describeInviteErrorCode(reason: string | null): string {
  switch (reason) {
    case 'expired':
      return 'Esse convite expirou.';
    case 'revoked':
      return 'Esse convite foi revogado.';
    case 'used_up':
      return 'Esse convite já foi usado. Se você não foi quem usou, peça um novo link ao mestre.';
    case 'not_found':
      return 'Convite não encontrado. Confira se o link está completo.';
    case 'invalid':
      return 'Link de convite inválido.';
    case 'unavailable':
      return 'O servidor não conseguiu aceitar o convite agora. Abra o link de novo em alguns minutos.';
    default:
      return 'Não foi possível aceitar o convite.';
  }
}
