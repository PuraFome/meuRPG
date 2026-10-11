import { askedWho, askTaskGroups } from './ask-roll';

describe('ask-roll', () => {
  it('has three groups, the saving throws named "Teste de resistência de ..."', () => {
    const groups = askTaskGroups([{ key: 'skill:stealth', name: 'Furtividade' }]);
    expect(groups.map((g) => g.label)).toEqual([
      'Perícias',
      'Habilidades',
      'Testes de resistência',
    ]);
    expect(groups[1].tasks[0]).toEqual({ key: 'ability:str', name: 'Teste de Força' });
    expect(groups[2].tasks.map((t) => t.key)).toEqual([
      'save:str',
      'save:dex',
      'save:con',
      'save:int',
      'save:wis',
      'save:cha',
    ]);
    expect(groups[2].tasks[2].name).toBe('Teste de resistência de Constituição');
  });

  it('is a group check only with two characters or more and the box on', () => {
    expect(askedWho([], 4, true)).toEqual({ characterIds: [], group: true });
    expect(askedWho([], 4, false).group).toBe(false);
    expect(askedWho(['a'], 4, true).group).toBe(false);
    expect(askedWho(['a', 'b'], 4, true)).toEqual({ characterIds: ['a', 'b'], group: true });
    expect(askedWho([], 1, true).group).toBe(false);
  });
});
