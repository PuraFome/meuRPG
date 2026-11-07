import { create } from '@bufbuild/protobuf';

import {
  CastingTimeUnit,
  SpellDurationKind,
  SpellDurationUnit,
} from '../../../gen/meurpg/rules/v1/rules_pb';
import {
  TableSummonReplaces,
  TableSummonRole,
  TableSummonSchema,
  TableSummonTurnMode,
} from '../../../gen/meurpg/rules/v1/table_content_pb';
import { emptyEffect } from './effect-draft';
import { emptyFeature } from './feature-draft';
import {
  draftToSummon,
  emptySummon,
  followResource,
  resourcesOf,
  summonBlockedPaths,
  summonPaths,
  summonToDraft,
} from './summon-draft';
import {
  durationOf,
  durationOptions,
  durationValue,
  timeOf,
  timeOptions,
  timeValue,
} from './summon-options';
import { creatureNameOf, rememberCreatureName, summonLine } from './summon-names';

const stored = () =>
  create(TableSummonSchema, {
    creatureKeys: ['creature:lobo-do-vale@mesa', 'monster:wolf'],
    count: 'mod("cha") + 1',
    countPerSlotLevel: 0,
    turnMode: TableSummonTurnMode.OWNER_ROLL_GROUP,
    role: TableSummonRole.COMPANION,
    maxLive: 2,
    replaces: TableSummonReplaces.SAME_SUMMON,
    resourceKey: 'companheiro',
    resourceCost: 1,
    castingTime: { unit: CastingTimeUnit.BONUS_ACTION, amount: 1 },
    duration: {
      kind: SpellDurationKind.TIMED,
      amount: 1,
      unit: SpellDurationUnit.HOUR,
      upTo: true,
    },
    concentration: true,
  });

describe('the summon draft (#578)', () => {
  it('turns a stored summon into a draft and back to the same message', () => {
    const s = stored();
    expect(draftToSummon(summonToDraft(s))).toEqual(s);
  });

  it('keeps what the block does not edit: the role, the cost, the count per slot level and an unknown enum value', () => {
    const s = create(TableSummonSchema, {
      creatureKeys: ['monster:wolf'],
      count: '1',
      role: TableSummonRole.MOUNT,
      countPerSlotLevel: 3,
      resourceCost: 5,
      turnMode: 9 as TableSummonTurnMode,
      replaces: 7 as TableSummonReplaces,
      resourceKey: 'x',
    });
    const back = draftToSummon(summonToDraft(s));
    expect(back.role).toBe(TableSummonRole.MOUNT);
    expect(back.countPerSlotLevel).toBe(3);
    expect(back.resourceCost).toBe(5);
    expect(back.turnMode).toBe(9);
    expect(back.replaces).toBe(7);
  });

  it('keeps an empty time as empty and an empty duration as empty when nothing touches them', () => {
    const s = create(TableSummonSchema, {
      creatureKeys: ['monster:wolf'],
      count: '1',
      resourceKey: 'x',
    });
    const back = draftToSummon(summonToDraft(s));
    expect(back.castingTime).toBeUndefined();
    expect(back.duration).toBeUndefined();
    expect(timeValue(summonToDraft(s).time)).toBe(timeValue(emptySummon().time));
  });

  it('sends the formula as typed and the most at a time as a number; text that is no number is refused by the server (-1)', () => {
    const d = { ...summonToDraft(stored()), count: ' 1 + slot() ', maxLive: '12' };
    expect(draftToSummon(d).count).toBe(' 1 + slot() ');
    expect(draftToSummon(d).maxLive).toBe(12);
    expect(draftToSummon({ ...d, maxLive: '' }).maxLive).toBe(0);
    expect(draftToSummon({ ...d, maxLive: '1,5' }).maxLive).toBe(-1);
    expect(draftToSummon({ ...d, maxLive: '-2' }).maxLive).toBe(-1);
  });

  it('a new summon costs 1 use and starts at the defaults of the board', () => {
    const s = draftToSummon(emptySummon('companheiro'));
    expect(s.resourceCost).toBe(1);
    expect(s.resourceKey).toBe('companheiro');
    expect(s.turnMode).toBe(TableSummonTurnMode.OWN_INITIATIVE);
    expect(s.replaces).toBe(TableSummonReplaces.NONE);
    expect(s.castingTime?.unit).toBe(CastingTimeUnit.ACTION);
    expect(s.duration?.kind).toBe(SpellDurationKind.INSTANTANEOUS);
    expect(s.concentration).toBe(false);
  });

  it('names the paths of the inputs it draws, under the feature', () => {
    expect(summonPaths('table_race.traits[0]')).toContain('table_race.traits[0].summon.max_live');
    expect(summonPaths('table_class_feature')).toContain(
      'table_class_feature.summon.creature_keys',
    );
  });
});

describe('the resources a summon spends', () => {
  const resource = (name: string, max: string) => ({
    ...emptyEffect('resource'),
    resource: name,
    max,
  });

  it('lists the resource effects of the feature: its name and the uses when it has one, the resource name when it has several', () => {
    const one = {
      ...emptyFeature(),
      name: 'Companheiro do Vale',
      effects: [resource('companheiro', '1')],
    };
    expect(resourcesOf(one)).toEqual([{ key: 'companheiro', label: 'Companheiro do Vale (1)' }]);
    const two = {
      ...one,
      effects: [resource('a', '2'), resource('b', 'prof()'), emptyEffect('note')],
    };
    expect(resourcesOf(two).map((r) => r.label)).toEqual(['a (2)', 'b (prof())']);
    expect(resourcesOf({ ...one, effects: [resource('  ', '1')] })).toEqual([]);
  });

  it('blocks the save while the switch is on with no resource, or with one the feature no longer has', () => {
    const f = { ...emptyFeature(), summon: emptySummon('') };
    expect(summonBlockedPaths('table_race.traits[0]', f)).toEqual([
      'table_race.traits[0].summon.resource_key',
    ]);
    const ok = {
      ...f,
      effects: [resource('companheiro', '1')],
      summon: emptySummon('companheiro'),
    };
    expect(summonBlockedPaths('t', ok)).toEqual([]);
    expect(summonBlockedPaths('t', { ...ok, summon: emptySummon('outro') })).toHaveLength(1);
    expect(summonBlockedPaths('t', { ...ok, summon: null })).toEqual([]);
  });

  it('follows the name of its one resource when the master edits it', () => {
    const f = {
      ...emptyFeature(),
      effects: [resource('companheiro2', '1')],
      summon: emptySummon('companheiro'),
    };
    expect(followResource(f).summon?.resourceKey).toBe('companheiro2');
    const two = { ...f, effects: [resource('a', '1'), resource('b', '1')] };
    expect(followResource(two)).toBe(two);
    expect(followResource({ ...f, summon: null }).summon).toBeNull();
  });
});

describe('the time and duration lists', () => {
  it('offers the presets and adds a stored value they do not have', () => {
    expect(timeOptions({ unit: CastingTimeUnit.ACTION, amount: 1, triggerPt: '' })[0].label).toBe(
      '1 ação',
    );
    const odd = timeOptions({ unit: CastingTimeUnit.MINUTE, amount: 3, triggerPt: '' });
    expect(odd[odd.length - 1].label).toBe('3 minutos');
    const dur = durationOptions({
      kind: SpellDurationKind.TIMED,
      amount: 3,
      unit: SpellDurationUnit.ROUND,
      upTo: false,
    });
    expect(dur[dur.length - 1].label).toBe('3 rodadas');
  });

  it('reads a value back, keeping a stored trigger only while the unit stays', () => {
    const reaction = { unit: CastingTimeUnit.REACTION, amount: 1, triggerPt: 'quando sofre dano' };
    expect(timeOf(timeValue(reaction), reaction).triggerPt).toBe('quando sofre dano');
    expect(timeOf(`${CastingTimeUnit.HOUR}:1`, reaction)).toEqual({
      unit: CastingTimeUnit.HOUR,
      amount: 1,
      triggerPt: '',
    });
    const custom = {
      kind: SpellDurationKind.TIMED,
      amount: 3,
      unit: SpellDurationUnit.ROUND,
      upTo: true,
    };
    expect(durationOf(durationValue(custom), custom)).toBe(custom);
    expect(durationOf(String(SpellDurationKind.INSTANTANEOUS), custom).kind).toBe(
      SpellDurationKind.INSTANTANEOUS,
    );
  });
});

describe('the names of the creatures a summon lists', () => {
  const nameOf = (k: string) => (k === 'creature:lobo-do-vale@mesa' ? 'Lobo do Vale' : k);

  it('takes a table creature from the catalog, an SRD one from what the bestiary said, else from its slug', () => {
    expect(creatureNameOf('creature:lobo-do-vale@mesa', nameOf)).toBe('Lobo do Vale');
    expect(creatureNameOf('monster:giant-wolf-spider', nameOf)).toBe('Giant wolf spider');
    rememberCreatureName('monster:giant-wolf-spider', 'Aranha-lobo gigante');
    expect(creatureNameOf('monster:giant-wolf-spider', nameOf)).toBe('Aranha-lobo gigante');
  });

  it('says "Invoca: Lobo do Vale, Unheard of." and nothing for a feature with no summon', () => {
    const unheard = create(TableSummonSchema, {
      creatureKeys: ['creature:lobo-do-vale@mesa', 'monster:unheard-of'],
    });
    expect(summonLine(unheard, nameOf)).toBe('Invoca: Lobo do Vale, Unheard of.');
    expect(summonLine(undefined, nameOf)).toBe('');
    expect(summonLine({ creatureKeys: [] }, nameOf)).toBe('');
  });
});
