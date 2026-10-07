import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import {
  EncounterBlockedReason,
  EncounterBlockedSchema,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { needsTwoDice, trapErrorMessage } from './trap-errors';

function blocked(reason: EncounterBlockedReason): ConnectError {
  const err = new ConnectError('x', Code.FailedPrecondition);
  return Object.assign(err, { findDetails: () => [create(EncounterBlockedSchema, { reason })] });
}

describe('trapErrorMessage', () => {
  it('says each refusal of the search by its reason, never by the message', () => {
    expect(trapErrorMessage(blocked(EncounterBlockedReason.ACTION_USED))).toBe(
      'Você já usou a sua ação neste turno.',
    );
    expect(trapErrorMessage(blocked(EncounterBlockedReason.NOT_YOUR_TURN))).toBe(
      'Procurar numa luta só na sua vez.',
    );
    expect(trapErrorMessage(blocked(EncounterBlockedReason.TRAP_NOT_ON_MAP))).toContain(
      'não está no mapa',
    );
    expect(trapErrorMessage(blocked(EncounterBlockedReason.TRAP_SEARCH_NOT_NOW))).toContain(
      'o combate não está na vez de ninguém',
    );
    expect(trapErrorMessage(blocked(EncounterBlockedReason.TRAP_NOT_ARMED))).toContain(
      'já disparou ou foi desarmada',
    );
  });

  it('knows the second die is wanted', () => {
    expect(needsTwoDice(blocked(EncounterBlockedReason.SEARCH_NEEDS_TWO_DICE))).toBe(true);
    expect(needsTwoDice(blocked(EncounterBlockedReason.ACTION_USED))).toBe(false);
  });

  it('falls back to the code', () => {
    expect(trapErrorMessage(new ConnectError('x', Code.PermissionDenied))).toBe(
      'Só o mestre da campanha faz isso.',
    );
  });
});
