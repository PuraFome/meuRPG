import { create } from '@bufbuild/protobuf';
import { describe, expect, it } from 'vitest';

import {
  AttunementRestriction,
  FullSheetSchema,
  ItemBonusSchema,
  SheetMagicItemSchema,
} from '../../../gen/meurpg/characters/v1/characters_pb';

import { attunementOf } from './attunement';

describe('attunementOf', () => {
  it('lists the pieces that name a catalog item or ask, then the magic items, and counts the attuned that ask', () => {
    const full = create(FullSheetSchema, {
      itemBonuses: [
        create(ItemBonusSchema, {
          key: 'equipment:longsword',
          bonus: 3,
          itemKey: 'item:defender',
          itemNamePt: 'Defensora',
          id: 'bonus:equipment:longsword',
          requiresAttunement: true,
          attuned: true,
        }),
        create(ItemBonusSchema, {
          key: 'equipment:shield',
          bonus: 1,
          id: 'bonus:equipment:shield',
        }),
      ],
      magicItems: [
        create(SheetMagicItemSchema, {
          id: 'm1',
          key: 'item:cloak-of-elvenkind',
          namePt: 'Manto élfico',
          requiresAttunement: true,
        }),
        create(SheetMagicItemSchema, {
          id: 'm2',
          key: 'item:ring-of-protection',
          namePt: 'Anel de proteção',
          requiresAttunement: true,
          attuned: true,
          restriction: AttunementRestriction.NOT_MET,
          restrictionPt: 'Só para um conjurador',
        }),
        create(SheetMagicItemSchema, {
          id: 'm3',
          key: 'item:bag-of-holding',
          namePt: 'Bolsa de carga',
        }),
      ],
    });
    const a = attunementOf(full);
    expect(a.entries.map((e) => e.id)).toEqual(['bonus:equipment:longsword', 'm1', 'm2', 'm3']);
    expect(a.attuned).toBe(2);
    expect(a.entries[0]).toMatchObject({
      kind: 'piece',
      namePt: 'Defensora',
      asks: true,
      attuned: true,
    });
    expect(a.entries[2]).toMatchObject({
      restriction: 'notMet',
      restrictionPt: 'Só para um conjurador',
    });
    expect(a.entries[3]).toMatchObject({ asks: false, restriction: 'none' });
  });
});
