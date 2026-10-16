import type { ParamMap } from '@angular/router';

import { NO_FILTER, type SpellFilter, SCHOOLS } from '../../core/spells/spells-filter';

/** The query parameters: the filters survive a reload, and the open spell is a step of the browser's history. */
export const PARAMS = {
  query: 'q',
  classKey: 'class',
  levels: 'levels',
  school: 'school',
  mine: 'mine',
  spell: 'spell',
} as const;

/** The longest search the link keeps. */
const QUERY_MAX = 100;

/** The filters in the link, so a reload or a shared link brings the same search. `hasMine`: the person has a character, so "mine" can be on. */
export function filterFromParams(q: ParamMap, hasMine: boolean): SpellFilter {
  const levels = (q.get(PARAMS.levels) ?? '')
    .split(',')
    .filter((x) => /^[0-9]$/.test(x))
    .map(Number);
  const school = q.get(PARAMS.school) ?? '';
  return {
    ...NO_FILTER,
    query: (q.get(PARAMS.query) ?? '').slice(0, QUERY_MAX),
    classKey: q.get(PARAMS.classKey) ?? '',
    levels: [...new Set(levels)].sort((a, b) => a - b),
    schoolKey: SCHOOLS.some((s) => s.key === school) ? school : '',
    onlyMine: q.get(PARAMS.mine) === '1' && hasMine,
  };
}

/** The link's words for the filters. */
export function filterParams(f: SpellFilter): Record<string, string | null> {
  return {
    [PARAMS.query]: f.query.trim() || null,
    [PARAMS.classKey]: f.classKey || null,
    [PARAMS.levels]: f.levels.length > 0 ? f.levels.join(',') : null,
    [PARAMS.school]: f.schoolKey || null,
    [PARAMS.mine]: f.onlyMine ? '1' : null,
  };
}

/** Whether the filters the link names are the ones on screen (the name is compared as the link would carry it, trimmed). */
export function sameFilters(fromUrl: SpellFilter, now: SpellFilter): boolean {
  return (
    fromUrl.query === now.query.trim() &&
    fromUrl.classKey === now.classKey &&
    fromUrl.schoolKey === now.schoolKey &&
    fromUrl.onlyMine === now.onlyMine &&
    fromUrl.levels.join(',') === now.levels.join(',')
  );
}
