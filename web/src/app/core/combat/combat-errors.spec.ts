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

describe('the reasons slice 6.5c added', () => {
  const said = (reason: EncounterBlockedReason, more: object = {}) => blockedMessage({ reason, ...more } as never);

  it('says why a spell, an action or a death save was refused', () => {
    expect(said(EncounterBlockedReason.NO_SLOT, { minLevel: 2 })).toBe('Não há espaço de 2º\u00a0círculo ou maior livre.');
    expect(said(EncounterBlockedReason.NO_SLOT)).toBe('Não há espaço de magia livre.');
    expect(said(EncounterBlockedReason.NO_USES, { recharge: 1 })).toBe('Sem usos: volta num descanso curto.');
    expect(said(EncounterBlockedReason.BONUS_ACTION_USED)).toMatch(/ação bônus/);
    expect(said(EncounterBlockedReason.ATTACKS_USED)).toMatch(/ataques/);
    expect(said(EncounterBlockedReason.REACTION_USED)).toMatch(/reação/);
    expect(said(EncounterBlockedReason.DEATH_SAVE_DUE)).toMatch(/teste contra a morte/);
    expect(said(EncounterBlockedReason.DEATH_SAVE_NOT_DUE)).toMatch(/Não há teste/);
    expect(said(EncounterBlockedReason.NOT_DYING)).toMatch(/três testes/);
    expect(said(EncounterBlockedReason.NOT_AWAITING_REACTION)).toMatch(/não espera/);
  });
});
