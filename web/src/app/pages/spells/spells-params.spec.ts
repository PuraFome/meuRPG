import { convertToParamMap } from '@angular/router';

import { NO_FILTER } from '../../core/spells/spells-filter';
import { filterFromParams, filterParams, sameFilters } from './spells-params';

describe('the spells link', () => {
  const link = (params: Record<string, string>) => convertToParamMap(params);

  it('reads the filters of the link, cleaned', () => {
    const f = filterFromParams(
      link({
        q: 'a'.repeat(120),
        class: 'class:wizard',
        levels: '3,x,1,3,10',
        school: 'school:nope',
        mine: '1',
      }),
      true,
    );
    expect(f).toEqual({
      ...NO_FILTER,
      query: 'a'.repeat(100),
      classKey: 'class:wizard',
      levels: [1, 3],
      schoolKey: '',
      onlyMine: true,
    });
  });

  it('turns "mine" off for a person with no character, and reads an empty link as no filter', () => {
    expect(filterFromParams(link({ mine: '1' }), false).onlyMine).toBe(false);
    expect(filterFromParams(link({}), true)).toEqual(NO_FILTER);
  });

  it('writes only the filters that are on, and null for the others', () => {
    expect(filterParams({ ...NO_FILTER, query: ' maos ', levels: [0, 2], onlyMine: true })).toEqual(
      {
        q: 'maos',
        class: null,
        levels: '0,2',
        school: null,
        mine: '1',
      },
    );
    expect(filterParams(NO_FILTER)).toEqual({
      q: null,
      class: null,
      levels: null,
      school: null,
      mine: null,
    });
  });

  it('compares the link with the screen by the name as trimmed, and the levels in order', () => {
    expect(sameFilters({ ...NO_FILTER, query: 'a' }, { ...NO_FILTER, query: ' a ' })).toBe(true);
    expect(sameFilters({ ...NO_FILTER, levels: [1] }, { ...NO_FILTER, levels: [2] })).toBe(false);
    expect(sameFilters({ ...NO_FILTER, onlyMine: true }, NO_FILTER)).toBe(false);
    expect(sameFilters({ ...NO_FILTER, classKey: 'x' }, NO_FILTER)).toBe(false);
    expect(sameFilters({ ...NO_FILTER, schoolKey: 'x' }, NO_FILTER)).toBe(false);
  });
});
