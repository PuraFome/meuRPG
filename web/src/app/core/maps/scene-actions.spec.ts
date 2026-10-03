import {
  CHECK_KINDS,
  actionSubtitle,
  actionTitle,
  checkKindLabel,
  checkOptions,
  parseDc,
} from './scene-actions';

describe('scene actions', () => {
  it('reads the DC field: empty is none, 1 to 30 is a DC, anything else is refused', () => {
    expect(parseDc('')).toBe(0);
    expect(parseDc('  ')).toBe(0);
    expect(parseDc('1')).toBe(1);
    expect(parseDc(' 30 ')).toBe(30);
    for (const bad of ['0', '31', '-3', '12.5', 'doze', '1e1', '0012x']) {
      expect(parseDc(bad), bad).toBeNull();
    }
  });

  it('lists the six abilities as checks and as saves, and the skills as given', () => {
    const skills = [{ key: 'skill:arcana', label: 'Arcanismo' }];
    expect(checkOptions('skill', skills)).toEqual(skills);
    const abilities = checkOptions('ability', skills);
    expect(abilities.map((o) => o.key)).toEqual([
      'ability:str', 'ability:dex', 'ability:con', 'ability:int', 'ability:wis', 'ability:cha',
    ]);
    expect(abilities[0].label).toBe('Força');
    expect(checkOptions('save', skills).map((o) => o.key)[4]).toBe('save:wis');
  });

  it('names an action by its own name, or by the check when it has none', () => {
    const named = { name: 'Procurar pistas', checkName: 'Investigação', key: 'skill:investigation' };
    const bare = { name: '', checkName: 'Percepção', key: 'skill:perception' };
    expect(actionTitle(named)).toBe('Procurar pistas');
    expect(actionSubtitle(named)).toBe('Investigação');
    expect(actionTitle(bare)).toBe('Percepção');
    expect(actionSubtitle(bare)).toBe('Perícia');
    expect(actionSubtitle({ name: '', checkName: 'Teste de Força', key: 'ability:str' })).toBe('Teste de atributo');
    expect(checkKindLabel('save:wis')).toBe('Salvaguarda');
    expect(CHECK_KINDS.map((k) => k.label)).toEqual(['Perícia', 'Teste de atributo', 'Salvaguarda']);
  });
});
