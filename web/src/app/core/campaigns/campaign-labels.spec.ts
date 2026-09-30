import { InviteState, Role, XpMode } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { displayNameOrFallback, inviteStateLabel, roleLabel, xpModeLabel } from './campaign-labels';

describe('roleLabel', () => {
  it('shows "mestre" for the master and "jogador" for a player', () => {
    expect(roleLabel(Role.MASTER)).toBe('mestre');
    expect(roleLabel(Role.PLAYER)).toBe('jogador');
  });
});

describe('xpModeLabel', () => {
  it('names each XP mode in Portuguese', () => {
    expect(xpModeLabel(XpMode.ENEMIES)).toBe('Por inimigos derrotados');
    expect(xpModeLabel(XpMode.GOLD)).toBe('Por ouro');
    expect(xpModeLabel(XpMode.MILESTONES)).toBe('Por marcos');
  });
});

describe('inviteStateLabel', () => {
  it('maps every InviteState to its Portuguese label', () => {
    expect(inviteStateLabel(InviteState.ACTIVE)).toBe('ativo');
    expect(inviteStateLabel(InviteState.USED_UP)).toBe('usado');
    expect(inviteStateLabel(InviteState.EXPIRED)).toBe('expirado');
    expect(inviteStateLabel(InviteState.REVOKED)).toBe('revogado');
  });
});

describe('displayNameOrFallback', () => {
  it('returns the trimmed name when one was chosen', () => {
    expect(displayNameOrFallback('  Vinicius  ')).toBe('Vinicius');
  });

  it('falls back to "Sem nome" — never an e-mail — when empty', () => {
    expect(displayNameOrFallback('')).toBe('Sem nome');
    expect(displayNameOrFallback('   ')).toBe('Sem nome');
  });
});
