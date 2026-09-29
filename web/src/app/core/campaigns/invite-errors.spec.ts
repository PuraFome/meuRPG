import { Code, ConnectError } from '@connectrpc/connect';

import { InviteState, InviteUnusableSchema } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { describeAcceptInviteError, describeInviteErrorCode } from './invite-errors';

function unusable(state: InviteState): ConnectError {
  return new ConnectError('convite não serve mais', Code.FailedPrecondition, undefined, [
    { desc: InviteUnusableSchema, value: { state } },
  ]);
}

describe('describeAcceptInviteError', () => {
  it('reads the InviteUnusable detail to say exactly why: expired', () => {
    expect(describeAcceptInviteError(unusable(InviteState.EXPIRED))).toBe('Esse convite expirou.');
  });

  it('reads the InviteUnusable detail to say exactly why: revoked', () => {
    expect(describeAcceptInviteError(unusable(InviteState.REVOKED))).toBe(
      'Esse convite foi revogado.',
    );
  });

  it('reads the InviteUnusable detail to say exactly why: used up', () => {
    expect(describeAcceptInviteError(unusable(InviteState.USED_UP))).toContain('já foi usado');
  });

  it('maps not_found to a clear message', () => {
    const err = new ConnectError('no invite has this token', Code.NotFound);
    expect(describeAcceptInviteError(err)).toContain('Convite não encontrado');
  });

  it('maps invalid_argument (empty token) to a clear message', () => {
    const err = new ConnectError('token is empty', Code.InvalidArgument);
    expect(describeAcceptInviteError(err)).toContain('Link de convite inválido');
  });

  it('maps unavailable, and any unrecognized failure, to a "try again" message', () => {
    expect(describeAcceptInviteError(new ConnectError('down', Code.Unavailable))).toContain(
      'Tente de novo',
    );
    expect(describeAcceptInviteError(new TypeError('Failed to fetch'))).toContain('Tente de novo');
  });
});

describe('describeInviteErrorCode', () => {
  it.each([
    ['expired', 'expirou'],
    ['revoked', 'revogado'],
    ['used_up', 'já foi usado'],
    ['not_found', 'não encontrado'],
    ['invalid', 'inválido'],
  ])('maps motivo=%s to a message containing %j', (motivo, fragment) => {
    expect(describeInviteErrorCode(motivo)).toContain(fragment);
  });

  it('falls back to a generic message for an unknown or missing motivo', () => {
    expect(describeInviteErrorCode('something-new')).toBe('Não foi possível aceitar o convite.');
    expect(describeInviteErrorCode(null)).toBe('Não foi possível aceitar o convite.');
  });
});
