import { Role, XpMode } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { campaignLead, roleTag, xpModeSentence } from './campaign-copy';

describe('campaign copy', () => {
  it('says each XP mode as a phrase, and nothing for an unset one', () => {
    expect(xpModeSentence(XpMode.ENEMIES)).toBe('XP por inimigos derrotados');
    expect(xpModeSentence(XpMode.GOLD)).toBe('XP por ouro');
    expect(xpModeSentence(XpMode.MILESTONES)).toBe('XP por marcos');
    expect(xpModeSentence(XpMode.UNSPECIFIED)).toBeNull();
  });

  it('tags the role with one capitalised word', () => {
    expect(roleTag(Role.MASTER)).toBe('Mestre');
    expect(roleTag(Role.PLAYER)).toBe('Jogador');
  });

  it('writes the lead with the role and, when set, the XP mode', () => {
    expect(campaignLead({ myRole: Role.MASTER, xpMode: XpMode.ENEMIES })).toBe(
      'Você é mestre nesta campanha. XP por inimigos derrotados.',
    );
    expect(campaignLead({ myRole: Role.PLAYER, xpMode: XpMode.UNSPECIFIED })).toBe(
      'Você é jogador nesta campanha.',
    );
  });
});
