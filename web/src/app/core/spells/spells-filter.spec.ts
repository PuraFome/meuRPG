import { NO_FILTER, PAGE_SIZE, activeFilters, filterChips, isFiltered, toListRequest, toggleLevel } from './spells-filter';

describe('the "Magias" filters (MR-045)', () => {
  it('maps the filters to the ListSpells request, and nothing is filtered in the browser', () => {
    const req = toListRequest(
      'camp-1',
      { query: '  maos ', classKey: 'class:wizard', levels: [0, 1], schoolKey: 'school:evocation', onlyMine: true },
      'char-1',
    );
    expect(req).toEqual({
      campaignId: 'camp-1',
      query: 'maos',
      classKey: 'class:wizard',
      levels: [0, 1],
      schoolKeys: ['school:evocation'],
      characterId: 'char-1',
      pageSize: PAGE_SIZE,
      pageToken: '',
    });
  });

  it('sends no filter for an empty one, and the character only with "Só as que posso aprender"', () => {
    expect(toListRequest('c', NO_FILTER, 'char-1')).toMatchObject({ query: '', classKey: '', levels: [], schoolKeys: [], characterId: '' });
    expect(toListRequest('c', { ...NO_FILTER, onlyMine: true }, null).characterId).toBe('');
    expect(toListRequest('c', { ...NO_FILTER, onlyMine: true }, 'char-1').characterId).toBe('char-1');
  });

  it('carries the page token of the next page', () => {
    expect(toListRequest('c', NO_FILTER, null, 'tok').pageToken).toBe('tok');
  });

  it('counts the filters beyond the name: the number in "Filtros (2)"', () => {
    expect(activeFilters({ ...NO_FILTER, query: 'maos' })).toBe(0);
    expect(activeFilters({ ...NO_FILTER, classKey: 'class:wizard', onlyMine: true })).toBe(2);
    expect(activeFilters({ ...NO_FILTER, levels: [1, 2, 3], schoolKey: 'school:illusion' })).toBe(2);
    expect(isFiltered({ ...NO_FILTER, query: ' ' })).toBe(false);
    expect(isFiltered({ ...NO_FILTER, query: 'a' })).toBe(true);
    expect(isFiltered({ ...NO_FILTER, onlyMine: true })).toBe(true);
  });

  it('toggles a circle on and off, keeping them in order', () => {
    expect(toggleLevel([], 2)).toEqual([2]);
    expect(toggleLevel([2], 0)).toEqual([0, 2]);
    expect(toggleLevel([0, 2], 2)).toEqual([0]);
  });

  it('asks the most the server gives in a page (400)', () => {
    expect(PAGE_SIZE).toBe(400);
  });

  it('draws no class chip until the class name is known', () => {
    expect(filterChips({ ...NO_FILTER, classKey: 'class:wizard' }, () => '')).toEqual([]);
  });

  it('writes one chip per filter that is on', () => {
    const chips = filterChips(
      { query: 'x', classKey: 'class:wizard', levels: [0, 2], schoolKey: 'school:evocation', onlyMine: true },
      (k) => (k === 'class:wizard' ? 'Mago' : ''),
    );
    expect(chips).toEqual([
      { id: 'class', label: 'Mago' },
      { id: 'level', label: 'Truque, 2º nível' },
      { id: 'school', label: 'Evocação' },
      { id: 'mine', label: 'Só as que posso aprender' },
    ]);
    expect(filterChips(NO_FILTER, () => '')).toEqual([]);
  });
});
