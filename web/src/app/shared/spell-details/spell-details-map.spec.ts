import { create } from '@bufbuild/protobuf';

import {
  CastingTimeUnit,
  SpellDetailsSchema,
  SpellDurationKind,
  SpellDurationUnit,
  SpellRangeKind,
} from '../../../gen/meurpg/rules/v1/rules_pb';
import { spellDetailsFromGen } from './spell-details-map';

describe('spellDetailsFromGen', () => {
  it('turns the generated message into the sheet\'s gen-free view (Sono)', () => {
    const vm = spellDetailsFromGen(
      create(SpellDetailsSchema, {
        spell: { key: 'spell:sleep', name: 'Sleep', namePt: 'Sono', level: 1, schoolNamePt: 'Encantamento' },
        castingTime: { amount: 1, unit: CastingTimeUnit.ACTION, raw: '1 action' },
        range: { kind: SpellRangeKind.RANGED, distanceFt: 90, raw: '90 feet' },
        components: { verbal: true, somatic: true, material: true, materialText: 'a pinch of fine sand' },
        duration: { kind: SpellDurationKind.TIMED, amount: 1, unit: SpellDurationUnit.MINUTE, raw: '1 minute' },
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
});
