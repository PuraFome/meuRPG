import { signal } from '@angular/core';

import { LevelUpHitPointsRule } from '../../../gen/meurpg/characters/v1/characters_pb';
import type { PickSet } from './levelup-draft-types';
import {
  adoptedHitPoints,
  anyPicked,
  featureOptionKeys,
  flip,
  keepOnly,
  nextFeatures,
  offeredAmong,
  subclassPrepares,
  trim,
} from './levelup-picks';
import { fighterOptions } from './levelup-testing';

const set = (...keys: string[]): PickSet => signal<ReadonlySet<string>>(new Set(keys));

describe('the pick sets', () => {
  it('flips a key in, out, replaces it when there is one place and refuses a full list', () => {
    const picks = set('a');
    flip(picks, 'b', 2);
    expect([...picks()]).toEqual(['a', 'b']);
    flip(picks, 'c', 2);
    expect([...picks()]).toEqual(['a', 'b']);
    flip(picks, 'a', 2);
    expect([...picks()]).toEqual(['b']);
    flip(picks, 'z', 1);
    expect([...picks()]).toEqual(['z']);
  });

  it('trims to the first picks and keeps the set itself when nothing goes', () => {
    const picks = set('a', 'b', 'c');
    const before = picks();
    trim(picks, 5);
    expect(picks()).toBe(before);
    trim(picks, 2);
    expect([...picks()]).toEqual(['a', 'b']);
  });

  it('keeps only what is allowed, and the set itself when nothing goes', () => {
    const picks = set('a', 'b');
    const before = picks();
    keepOnly(picks, () => true);
    expect(picks()).toBe(before);
    keepOnly(picks, (k) => k === 'b');
    expect([...picks()]).toEqual(['b']);
  });

  it('keeps the picks that are among the items offered', () => {
    expect([...offeredAmong(new Set(['a', 'x']), [{ key: 'a' }, { key: 'b' }])]).toEqual(['a']);
  });

  it('takes the feature options by the choose of their group', () => {
    const group = ['a', 'b', 'c'];
    expect([...nextFeatures(new Set(['a']), 'a', group, 2)]).toEqual([]);
    expect([...nextFeatures(new Set(['a']), 'b', group, 2)]).toEqual(['a', 'b']);
    const full = new Set(['a', 'b']);
    expect(nextFeatures(full, 'c', group, 2)).toBe(full);
    expect([...nextFeatures(new Set(['a', 'x']), 'b', group, 1)]).toEqual(['x', 'b']);
  });
});

describe('what a subclass brings', () => {
  it('lists the feature options of a subclass and whether it prepares spells', () => {
    const o = fighterOptions({
      subclasses: [
        {
          key: 'sub:a',
          prepares: true,
          featureChoices: [{ choose: 1, options: [{ key: 'o:1' }, { key: 'o:2' }] }],
        },
        { key: 'sub:b' },
      ] as never,
    });
    expect([...featureOptionKeys(o, 'sub:a')]).toEqual(['o:1', 'o:2']);
    expect([...featureOptionKeys(o, 'sub:b')]).toEqual([]);
    expect([...featureOptionKeys(o, 'none')]).toEqual([]);
    expect(subclassPrepares(o, 'sub:a')).toBe(true);
    expect(subclassPrepares(o, 'sub:b')).toBe(false);
    expect(subclassPrepares(o, 'none')).toBe(false);
  });
});

describe('what a re-read of the sheet keeps of the hit points', () => {
  const was = fighterOptions();
  const roll = { kind: 'physical', value: 8 } as const;

  it('lets the rule of the table decide the card when it has one', () => {
    const average = fighterOptions({ hitPointsRule: LevelUpHitPointsRule.AVERAGE_ONLY });
    expect(adoptedHitPoints(average, was, 'roll', roll)).toEqual({ card: 'average', rolled: null });
    const only = fighterOptions({ hitPointsRule: LevelUpHitPointsRule.ROLL_ONLY });
    expect(adoptedHitPoints(only, was, 'average', null)).toEqual({ card: 'roll', rolled: null });
    expect(adoptedHitPoints(only, was, 'average', roll).rolled).toEqual(roll);
  });

  it('keeps a typed roll for the same class and level and a die it fits only', () => {
    expect(adoptedHitPoints(was, was, 'roll', roll).rolled).toEqual(roll);
    expect(adoptedHitPoints(was, was, 'roll', { kind: 'physical', value: 11 }).rolled).toBeNull();
    expect(adoptedHitPoints(was, was, 'roll', { kind: 'physical', value: 0 }).rolled).toBeNull();
    expect(adoptedHitPoints(was, was, 'roll', { kind: 'physical', value: 1.5 }).rolled).toBeNull();
    const other = fighterOptions({ toLevel: 6 });
    expect(adoptedHitPoints(other, was, 'roll', roll).rolled).toBeNull();
  });

  it('keeps an app roll only when it is the one the server kept', () => {
    const kept = fighterOptions({ keptHitPointRoll: 6, keptHitPointRollClassKey: 'class:fighter' });
    const app = { kind: 'app', value: 6 } as const;
    expect(adoptedHitPoints(kept, was, 'roll', app).rolled).toEqual(app);
    expect(adoptedHitPoints(kept, was, 'roll', { kind: 'app', value: 5 }).rolled).toBeNull();
    expect(adoptedHitPoints(kept, was, 'average', app).rolled).toBeNull();
  });
});

describe('anyPicked', () => {
  const none = {
    abilityKeys: () => [],
    featKey: () => '',
    hpCard: () => 'average' as const,
    subclassKey: () => '',
    lateDrafts: () => new Map(),
    newDrafts: () => new Map(),
    instrument: () => '',
    cantrips: () => new Set<string>(),
    spells: () => new Set<string>(),
    prepared: () => new Set<string>(),
    features: () => new Set<string>(),
    skills: () => new Set<string>(),
    expertise: () => new Set<string>(),
  };

  it('is false for the defaults and true for any pick beyond them', () => {
    expect(anyPicked(none, 'average')).toBe(false);
    expect(anyPicked({ ...none, hpCard: () => 'roll' }, 'average')).toBe(true);
    expect(anyPicked({ ...none, hpCard: () => 'roll' }, 'roll')).toBe(false);
    expect(anyPicked({ ...none, featKey: () => 'f' }, 'average')).toBe(true);
    expect(anyPicked({ ...none, instrument: () => 'i' }, 'average')).toBe(true);
    expect(anyPicked({ ...none, expertise: () => new Set(['x']) }, 'average')).toBe(true);
    expect(anyPicked({ ...none, newDrafts: () => new Map([['a', 1]]) }, 'average')).toBe(true);
  });
});
