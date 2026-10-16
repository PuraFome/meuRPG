import { create } from '@bufbuild/protobuf';

import {
  CastingTimeUnit,
  SpellAttackType,
  SpellDetailsSchema,
  SpellDurationKind,
  SpellDurationUnit,
  SpellRangeKind,
} from '../../../gen/meurpg/rules/v1/rules_pb';
import { spellDetailsFromGen } from './spell-details-map';

describe('spellDetailsFromGen', () => {
  it("turns the generated message into the sheet's gen-free view (Sono)", () => {
    const vm = spellDetailsFromGen(
      create(SpellDetailsSchema, {
        spell: {
          key: 'spell:sleep',
          name: 'Sleep',
          namePt: 'Sono',
          level: 1,
          schoolNamePt: 'Encantamento',
        },
        castingTime: { amount: 1, unit: CastingTimeUnit.ACTION, raw: '1 action' },
        range: { kind: SpellRangeKind.RANGED, distanceFt: 90, raw: '90 feet' },
        components: {
          verbal: true,
          somatic: true,
          material: true,
          materialText: 'a pinch of fine sand',
        },
        duration: {
          kind: SpellDurationKind.TIMED,
          amount: 1,
          unit: SpellDurationUnit.MINUTE,
          raw: '1 minute',
        },
        description: ['This spell sends creatures into a magical slumber.'],
        higherLevel: ['When you cast this spell using a spell slot of 2nd level or higher...'],
      }),
    );
    expect(vm).toMatchObject({
      key: 'spell:sleep',
      namePt: 'Sono',
      nameEn: 'Sleep',
      level: 1,
      castingTime: { amount: 1, unit: 'action' },
      range: { kind: 'ranged', distanceFt: 90 },
      components: { verbal: true, somatic: true, material: true },
      duration: { kind: 'timed', amount: 1, unit: 'minute' },
    });
    expect(vm.description).toEqual(['This spell sends creatures into a magical slumber.']);
    expect(vm.higherLevel).toHaveLength(1);
  });

  it("carries the target, the attack, the damage and whether it is the table's (E10-11)", () => {
    const vm = spellDetailsFromGen(
      create(SpellDetailsSchema, {
        spell: {
          key: 'spell:lamina-de-nanquim@mesa',
          namePt: 'Lâmina de Nanquim',
          level: 1,
          classKeys: ['class:wizard'],
          archived: true,
        },
        target: { labelPt: 'Uma criatura' },
        attackType: SpellAttackType.RANGED,
        damage: [{ damageTypePt: 'necrótico', bySlotLevel: { 2: '3d8', 1: '2d8' } }],
      }),
    );
    expect(vm).toMatchObject({
      table: true,
      archived: true,
      classKeys: ['class:wizard'],
      targetLabel: 'Uma criatura',
      attack: 'ranged',
      damage: [{ dice: '2d8', typePt: 'necrótico' }],
    });
    expect(
      spellDetailsFromGen(create(SpellDetailsSchema, { spell: { key: 'spell:shield' } })).table,
    ).toBe(false);
  });
});

describe('spellDetailsFromGen: defaults and every enum', () => {
  it('reads an empty message as an empty, no-attack view', () => {
    const vm = spellDetailsFromGen(create(SpellDetailsSchema, {}));
    expect(vm).toMatchObject({
      key: '',
      namePt: '',
      nameEn: '',
      level: 0,
      ritual: false,
      concentration: false,
      castingTime: { amount: 0, unit: '', trigger: '', raw: '' },
      range: { kind: '', distanceFt: 0, raw: '' },
      components: { verbal: false, somatic: false, material: false, materialText: '' },
      duration: { kind: '', amount: 0, unit: '', upTo: false, concentration: false, raw: '' },
      classKeys: [],
      archived: false,
      table: false,
      targetLabel: '',
      attack: 'none',
      damage: [],
    });
  });

  it('maps every casting unit, range kind, duration kind and unit', () => {
    const units = [
      [CastingTimeUnit.UNSPECIFIED, ''],
      [CastingTimeUnit.ACTION, 'action'],
      [CastingTimeUnit.BONUS_ACTION, 'bonus_action'],
      [CastingTimeUnit.REACTION, 'reaction'],
      [CastingTimeUnit.MINUTE, 'minute'],
      [CastingTimeUnit.HOUR, 'hour'],
    ] as const;
    for (const [unit, key] of units) {
      const vm = spellDetailsFromGen(create(SpellDetailsSchema, { castingTime: { unit } }));
      expect(vm.castingTime.unit).toBe(key);
    }
    const ranges = [
      [SpellRangeKind.SELF, 'self'],
      [SpellRangeKind.TOUCH, 'touch'],
      [SpellRangeKind.RANGED, 'ranged'],
      [SpellRangeKind.SIGHT, 'sight'],
      [SpellRangeKind.UNLIMITED, 'unlimited'],
      [SpellRangeKind.SPECIAL, 'special'],
    ] as const;
    for (const [kind, key] of ranges) {
      expect(spellDetailsFromGen(create(SpellDetailsSchema, { range: { kind } })).range.kind).toBe(
        key,
      );
    }
    const kinds = [
      [SpellDurationKind.INSTANTANEOUS, 'instantaneous'],
      [SpellDurationKind.TIMED, 'timed'],
      [SpellDurationKind.UNTIL_DISPELLED, 'until_dispelled'],
      [SpellDurationKind.SPECIAL, 'special'],
    ] as const;
    for (const [kind, key] of kinds) {
      expect(
        spellDetailsFromGen(create(SpellDetailsSchema, { duration: { kind } })).duration.kind,
      ).toBe(key);
    }
    const durUnits = [
      [SpellDurationUnit.ROUND, 'round'],
      [SpellDurationUnit.MINUTE, 'minute'],
      [SpellDurationUnit.HOUR, 'hour'],
      [SpellDurationUnit.DAY, 'day'],
    ] as const;
    for (const [unit, key] of durUnits) {
      expect(
        spellDetailsFromGen(create(SpellDetailsSchema, { duration: { unit } })).duration.unit,
      ).toBe(key);
    }
  });

  it('reads the attack type, the table flag and the damage of the lowest level', () => {
    expect(
      spellDetailsFromGen(create(SpellDetailsSchema, { attackType: SpellAttackType.MELEE })).attack,
    ).toBe('melee');
    expect(
      spellDetailsFromGen(create(SpellDetailsSchema, { attackType: SpellAttackType.RANGED }))
        .attack,
    ).toBe('ranged');
    const vm = spellDetailsFromGen(
      create(SpellDetailsSchema, {
        spell: { key: 'spell:x@mesa', classKeys: ['c'], archived: true },
        target: { labelPt: 'uma criatura' },
        damage: [
          { damageTypePt: 'fogo', bySlotLevel: { 3: '8d6', 2: '6d6' } },
          { damageTypePt: 'gelo', byCharacterLevel: { 5: '1d8', 11: '2d8' } },
          { damageTypePt: 'nada' },
        ],
      }),
    );
    expect(vm.table).toBe(true);
    expect(vm.archived).toBe(true);
    expect(vm.classKeys).toEqual(['c']);
    expect(vm.targetLabel).toBe('uma criatura');
    expect(vm.damage).toEqual([
      { dice: '6d6', typePt: 'fogo' },
      { dice: '1d8', typePt: 'gelo' },
    ]);
  });
});
