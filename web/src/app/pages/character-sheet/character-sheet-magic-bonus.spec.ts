import { create } from '@bufbuild/protobuf';

import { CharacterSchema, FullSheetSchema } from '../../../gen/meurpg/characters/v1/characters_pb';
import {
  AttackKind,
  AttackSchema,
  DerivedSheetSchema,
} from '../../../gen/meurpg/rules/v1/rules_pb';
import { toCharacterSheetVm } from './character-sheet-map';
import { FullSheetVm } from './character-sheet.types';

describe('the magic bonus of the sheet’s attacks', () => {
  function attacksOf(...attacks: ReturnType<typeof create<typeof AttackSchema>>[]) {
    const vm = toCharacterSheetVm(
      create(CharacterSchema, {
        sheet: { content: { case: 'full', value: create(FullSheetSchema, {}) } },
        derived: create(DerivedSheetSchema, { attacks }),
      }),
    );
    return (vm.sheet as FullSheetVm).attacks;
  }

  it('reads the server’s value on each attack and defaults to 0', () => {
    const rows = attacksOf(
      create(AttackSchema, { key: 'equipment:rapier', kind: AttackKind.WEAPON, magicBonus: 2 }),
      create(AttackSchema, { key: 'equipment:dagger', kind: AttackKind.WEAPON }),
      create(AttackSchema, { key: 'spell:fire-bolt', kind: AttackKind.SPELL }),
    );
    expect(rows.map((a) => a.magicBonus)).toEqual([2, 0, 0]);
  });
});
