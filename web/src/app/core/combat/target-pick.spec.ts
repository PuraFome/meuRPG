import { create } from '@bufbuild/protobuf';

import {
  CoverDegree,
  CoverSource,
  TargetInReachSchema,
} from '../../../gen/meurpg/play/v1/combat_pb';
import type { TargetRule } from './cast-flow';
import {
  attackPickRows,
  castPickRows,
  counts,
  enough,
  farNote,
  pickWords,
  reasonOf,
  tapped,
} from './target-pick';

const target = (over: Record<string, unknown> = {}) =>
  create(TargetInReachSchema, { combatantId: 'g1', label: 'Goblin 1', ...over });
const SINGLE: TargetRule = { kind: 'single', max: 1, min: 1 };
const DARTS: TargetRule = { kind: 'darts', max: 3, min: 1 };
const MULTI: TargetRule = { kind: 'multi', max: 2, min: 1 };

describe('target-pick: the reasons', () => {
  it('has none for a target the server offers', () => {
    expect(reasonOf(target())).toBe('');
  });

  it('says "fora de alcance" for a target the reach leaves out', () => {
    expect(reasonOf(target({ tooFar: true }))).toBe('fora de alcance');
  });

  it('says "cobertura total" for the cover of the map, and the master’s mark in its own words', () => {
    expect(
      reasonOf(
        target({ untargetable: true, cover: CoverDegree.TOTAL, coverSource: CoverSource.MAP }),
      ),
    ).toBe('cobertura total');
    expect(
      reasonOf(
        target({ untargetable: true, cover: CoverDegree.TOTAL, coverSource: CoverSource.MARK }),
      ),
    ).toContain('não pode ser alvo');
  });

  it('tags a target past the normal range and within the long one, and nothing else', () => {
    const dagger = { rangeFt: 20, longRangeFt: 60 };
    expect(farNote(target({ distanceFt: 40 }), dagger)).toBe('Longe: desvantagem');
    expect(farNote(target({ distanceFt: 15 }), dagger)).toBe('');
    expect(farNote(target({ distanceFt: 40 }), { rangeFt: 20, longRangeFt: 0 })).toBe('');
    expect(farNote(target(), dagger)).toBe('');
  });
});

describe('target-pick: the rows', () => {
  it('keeps the order of the server, dims the ones that cannot be chosen and says the catch of a far one', () => {
    const rows = attackPickRows(
      [
        target({ combatantId: 'a', label: 'Goblin 1' }),
        target({ combatantId: 'b', label: 'Goblin 2', distanceFt: 40 }),
        target({ combatantId: 'c', label: 'Goblin 3', tooFar: true, distanceFt: 90 }),
      ],
      { rangeFt: 20, longRangeFt: 60 },
    );
    expect(rows.map((r) => [r.id, r.reason, r.note])).toEqual([
      ['a', '', ''],
      ['b', '', 'Longe: desvantagem'],
      ['c', 'fora de alcance', ''],
    ]);
  });

  it('leaves the caster out of a spell’s rows unless it is a heal, and then calls it "(você)"', () => {
    const list = [target({ combatantId: 'me', label: 'Pensantus' }), target()];
    expect(castPickRows(list, 'me', false).map((r) => r.id)).toEqual(['g1']);
    expect(castPickRows(list, 'me', true).map((r) => r.label)).toEqual([
      'Pensantus (você)',
      'Goblin 1',
    ]);
  });
});

describe('target-pick: the taps', () => {
  it('replaces a single target', () => {
    expect(tapped(SINGLE, ['a'], 'b')).toEqual(['b']);
  });

  it('adds one dart for each tap, up to the total, and a target may take several', () => {
    let picks: string[] = [];
    for (const id of ['a', 'a', 'b', 'c']) {
      picks = tapped(DARTS, picks, id);
    }
    expect(picks).toEqual(['a', 'a', 'b']);
    expect([...counts(picks)]).toEqual([
      ['a', 2],
      ['b', 1],
    ]);
  });

  it('toggles several targets up to the limit', () => {
    expect(tapped(MULTI, ['a'], 'b')).toEqual(['a', 'b']);
    expect(tapped(MULTI, ['a', 'b'], 'c')).toEqual(['a', 'b']);
    expect(tapped(MULTI, ['a', 'b'], 'a')).toEqual(['b']);
  });

  it('is enough with one target, or with every dart placed', () => {
    expect(enough(SINGLE, [])).toBe(false);
    expect(enough(SINGLE, ['a'])).toBe(true);
    expect(enough(DARTS, ['a', 'a'])).toBe(false);
    expect(enough(DARTS, ['a', 'a', 'b'])).toBe(true);
  });
});

describe('target-pick: the words', () => {
  const labels = new Map([['a', 'Goblin 1']]);
  const base = {
    name: 'Raio de Fogo',
    labels,
    detail: '+5 para acertar · 1d10 de fogo',
    foe: true,
  };

  it('asks for the target, then names it with the numbers', () => {
    expect(pickWords({ ...base, rule: SINGLE, picks: [] })).toEqual({
      title: 'Escolha o alvo · Raio de Fogo',
      hint: 'Toque num inimigo com anel.',
    });
    const named = pickWords({ ...base, rule: SINGLE, picks: ['a'] });
    expect(named.title).toBe('Raio de Fogo em Goblin 1');
    expect(named.hint).toContain('+5 para acertar');
    expect(named.hint).toContain('toque em outro anel para trocar');
  });

  it('says "criatura" for a spell that is not an attack', () => {
    expect(pickWords({ ...base, foe: false, rule: SINGLE, picks: [] }).hint).toBe(
      'Toque numa criatura com anel.',
    );
  });

  it('counts the darts and the rays', () => {
    expect(pickWords({ ...base, name: 'Mísseis Mágicos', rule: DARTS, picks: ['a', 'a'] })).toEqual(
      {
        title: 'Mísseis Mágicos · 2 de 3 dardos',
        hint: 'Cada toque põe um dardo; um alvo pode levar vários.',
      },
    );
    expect(pickWords({ ...base, rule: { ...DARTS, rays: true }, picks: [] }).title).toContain(
      '0 de 3 raios',
    );
  });
});

describe('a target the acted-for character does not see', () => {
  it('is listed with the reason "não vê este alvo"', () => {
    const row = attackPickRows(
      [{ combatantId: 'c', label: 'Cultista', notSeen: true, untargetable: true } as never],
      { rangeFt: 5, longRangeFt: 0 },
    )[0];
    expect(row.reason).toBe('não vê este alvo');
  });
});
