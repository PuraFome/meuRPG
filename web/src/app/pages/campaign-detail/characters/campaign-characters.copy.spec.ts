import { characterRowSub, stateTagClass } from './campaign-characters.copy';
import { CampaignCharacterListItemVm } from './campaign-characters.types';

function item(overrides: Partial<CampaignCharacterListItemVm>): CampaignCharacterListItemVm {
  return {
    id: 'c1',
    name: 'Pensantus',
    kind: 'player',
    state: 'draft',
    classSummary: 'Mago 3',
    playerDisplayName: 'Vinicius',
    ...overrides,
  };
}

describe('campaign characters copy', () => {
  it('tells the master whose character it is', () => {
    expect(characterRowSub(item({}), true)).toBe('Mago 3, de Vinicius');
  });

  it('never says "Sem nome" for a player with no display name', () => {
    expect(characterRowSub(item({ playerDisplayName: null }), true)).toBe('Mago 3, de um jogador sem nome');
  });

  it('shows a player only the class of their own character', () => {
    expect(characterRowSub(item({}), false)).toBe('Mago 3');
  });

  it('says nothing about the player of an NPC, and nothing at all for a basic sheet', () => {
    expect(characterRowSub(item({ kind: 'enemy', classSummary: 'Guerreiro 2', playerDisplayName: null }), true)).toBe(
      'Guerreiro 2',
    );
    expect(characterRowSub(item({ kind: 'minion', classSummary: '', playerDisplayName: null }), true)).toBe('');
  });

  it('gives pending the warning tone and dead the danger tone', () => {
    expect(stateTagClass('pending')).toBe('mr-tag--pending');
    expect(stateTagClass('dead')).toBe('mr-tag--danger');
    expect(stateTagClass('draft')).toBe('');
    expect(stateTagClass('locked')).toBe('');
  });
});
