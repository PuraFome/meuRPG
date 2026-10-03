import { Code, ConnectError } from '@connectrpc/connect';

import { EncounterBlockedReason } from '../../../gen/meurpg/play/v1/combat_pb';
import { blockedMessage, combatErrorMessage } from './combat-errors';

describe('combat errors', () => {
  it('says how far a refused move was, in meters', () => {
    expect(
      blockedMessage({ reason: EncounterBlockedReason.TOO_FAR, missingFt: 5 } as never),
    ).toBe('Longe demais: faltam 1,5 m');
    expect(blockedMessage({ reason: EncounterBlockedReason.SQUARE_OCCUPIED } as never)).toMatch(/Ocupado/);
  });

  it('speaks by code when there is no typed detail', () => {
    expect(combatErrorMessage(new ConnectError('x', Code.Aborted))).toMatch(/mudou/);
    expect(combatErrorMessage(new ConnectError('x', Code.NotFound))).toMatch(/não existe mais/);
    expect(combatErrorMessage(new Error('network'))).toMatch(/servidor/);
  });
});
