import { masterTabItems, resolveTab } from './master-tabs';

describe('the master tabs', () => {
  it('takes the tab the address names', () => {
    expect(resolveTab('grupo', true)).toBe('grupo');
    expect(resolveTab('mapa', true)).toBe('mapa');
    expect(resolveTab('cena', false)).toBe('cena');
  });

  it('opens on the scene while one is open and on the map otherwise', () => {
    expect(resolveTab(null, true)).toBe('cena');
    expect(resolveTab(null, false)).toBe('mapa');
  });

  it('ignores a name that is not a tab', () => {
    expect(resolveTab('combate', true)).toBe('cena');
    expect(resolveTab('', false)).toBe('mapa');
  });

  it('counts what waits, in words too, and draws no count for nothing', () => {
    const none = masterTabItems({ newRolls: 0, castsWaiting: 0 });
    expect(none.map((t) => t.label)).toEqual(['Cena', 'Mapa', 'Grupo']);
    expect(none.every((t) => !t.badge)).toBe(true);
    const some = masterTabItems({ newRolls: 2, castsWaiting: 1 });
    expect(some[0]).toMatchObject({ badge: '2', tone: 'wait', words: '2 rolagens novas' });
    expect(some[1].badge).toBeUndefined();
    expect(some[2]).toMatchObject({ badge: '1', tone: 'wait', words: '1 conjuração esperando' });
  });
});
