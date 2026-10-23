import { Code, ConnectError } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';

import {
  EncounterBlockedReason,
  EncounterBlockedSchema,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { zoneEndError, zoneMoveError } from './zone-errors';

function blocked(reason: EncounterBlockedReason, missingFt = 0): ConnectError {
  return new ConnectError('blocked', Code.FailedPrecondition, undefined, [
    { desc: EncounterBlockedSchema, value: create(EncounterBlockedSchema, { reason, missingFt }) },
  ]);
}

describe('the words of a refused zone move or end', () => {
  it('says how far past the range the square was, in meters, and the range', () => {
    expect(zoneMoveError(blocked(EncounterBlockedReason.TARGET_OUT_OF_REACH, 10))).toBe(
      'Longe demais: a área anda até 18 m do ponto atual. Faltam 3 m.',
    );
  });

  it('uses the words of the other combat refusals for the turn and the action', () => {
    expect(zoneMoveError(blocked(EncounterBlockedReason.NOT_YOUR_TURN))).toBe('Não é a sua vez.');
    expect(zoneMoveError(blocked(EncounterBlockedReason.ACTION_USED))).toBe(
      'Sua ação já foi usada neste turno.',
    );
  });

  it('says who may move a zone, and that a zone gone is gone', () => {
    expect(zoneMoveError(new ConnectError('no', Code.PermissionDenied))).toBe(
      'Só quem lançou a magia, na vez dela, pode mover a zona.',
    );
    expect(zoneMoveError(new ConnectError('no', Code.NotFound))).toBe(
      'Essa zona não existe mais, ou você não a vê. A tela foi atualizada.',
    );
    expect(zoneMoveError(new ConnectError('no', Code.InvalidArgument))).toMatch(/mover a zona/);
  });

  it('says only the master ends a zone', () => {
    expect(zoneEndError(new ConnectError('no', Code.PermissionDenied))).toBe(
      'Só o mestre encerra uma zona.',
    );
    expect(zoneEndError(new ConnectError('no', Code.NotFound))).toBe(
      'Essa zona não existe mais, ou você não a vê. A tela foi atualizada.',
    );
    expect(zoneEndError(new ConnectError('no', Code.Unavailable))).toMatch(/encerrar a zona/);
  });
});
