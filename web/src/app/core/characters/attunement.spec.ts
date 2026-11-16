import { create } from '@bufbuild/protobuf';
import { TestBed } from '@angular/core/testing';
import { describe, expect, it } from 'vitest';

import {
  AttunementRestriction,
  ConsumableKind,
  FullSheetSchema,
  ItemBonusSchema,
  SheetMagicItemSchema,
} from '../../../gen/meurpg/characters/v1/characters_pb';

import {
  AttunementClient,
  MagicCategories,
  type MagicItemsPage,
  attunementOf,
  magicCategoryIcon,
  magicEntryOf,
} from './attunement';

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

describe('attunementOf, the charges', () => {
  it('carries the charges of a magic item that holds some and nothing for the others', () => {
    const a = attunementOf(
      create(FullSheetSchema, {
        magicItems: [
          create(SheetMagicItemSchema, { id: 'w', chargesMax: 7, chargesLeft: 5 }),
          create(SheetMagicItemSchema, { id: 'r' }),
        ],
      }),
    );
    expect(a.entries[0].charges).toEqual({ max: 7, left: 5, state: 'ready' });
    expect(a.entries[1].charges).toBeUndefined();
  });
});

describe('attunementOf, the consumables', () => {
  it('carries the line of what "Usar" does for a magic item, and none for the others', () => {
    const a = attunementOf(
      create(FullSheetSchema, {
        magicItems: [
          create(SheetMagicItemSchema, { id: 'p', effectPt: 'Cura 2d4 + 2' }),
          create(SheetMagicItemSchema, {
            id: 's',
            spellKey: 'spell:fireball',
            spellNamePt: 'Bola de Fogo',
          }),
          create(SheetMagicItemSchema, { id: 'r' }),
        ],
      }),
    );
    expect(a.entries.map((e) => e.effectPt)).toEqual(['Cura 2d4 + 2', '', '']);
    // The scroll's spell is not read until the scroll has a screen of its own.
    expect(Object.keys(a.entries[1])).not.toContain('spellNamePt');
  });

  it('carries what the server says of a consumable, and none for an item that is not one', () => {
    const a = attunementOf(
      create(FullSheetSchema, {
        magicItems: [
          create(SheetMagicItemSchema, {
            id: 'p',
            consumable: {
              kind: ConsumableKind.POTION,
              costsAction: true,
              healDiceCount: 2,
              healDiceSides: 4,
              healBonus: 2,
            },
          }),
          create(SheetMagicItemSchema, {
            id: 'o',
            consumable: { kind: ConsumableKind.OIL },
          }),
          create(SheetMagicItemSchema, { id: 'r' }),
        ],
      }),
    );
    expect(a.entries.map((e) => e.consumable?.kind)).toEqual(['potion', 'oil', undefined]);
    expect(a.entries[0].consumable).toEqual({
      kind: 'potion',
      costsAction: true,
      heal: { count: 2, sides: 4, modifier: 2, text: '2d4 + 2' },
    });
  });
});

describe('magicEntryOf', () => {
  it('is the entry of one magic item, as the sheet lists it, for a page that reads the items alone', () => {
    const m = create(SheetMagicItemSchema, {
      id: 'p',
      key: 'item:potion-of-healing',
      namePt: 'Poção de cura',
      effectPt: 'Cura 2d4 + 2',
    });
    expect(magicEntryOf(m)).toEqual(
      attunementOf(create(FullSheetSchema, { magicItems: [m] })).entries[0],
    );
    expect(magicEntryOf(m)).toMatchObject({
      id: 'p',
      key: 'item:potion-of-healing',
      namePt: 'Poção de cura',
      kind: 'magic',
      effectPt: 'Cura 2d4 + 2',
    });
  });
});

describe('MagicCategories', () => {
  const page = (items: MagicItemsPage['items'], nextPageToken = ''): MagicItemsPage => ({
    items,
    total: items.length,
    nextPageToken,
  });
  const option = (key: string, category: string, variants: string[] = []) => ({
    key,
    namePt: key,
    category,
    rarity: 'rare',
    asks: false,
    restrictionPt: '',
    standalone: true,
    variants: variants.map((v) => ({
      key: v,
      namePt: v,
      rarity: 'rare',
      asks: false,
      restrictionPt: '',
    })),
  });

  it('draws no control or state: a ring and ammunition take the gem', () => {
    expect(magicCategoryIcon('ring')).toBe('diamond');
    expect(magicCategoryIcon('ammunition')).toBe('diamond');
  });

  it("reads every page once and gives a variant its family's icon", async () => {
    const calls: string[] = [];
    TestBed.configureTestingModule({
      providers: [
        {
          provide: AttunementClient,
          useValue: {
            listMagicItems: async (_c: string, _q: string, token: string) => {
              calls.push(token);
              return token === ''
                ? page([option('item:ring', 'armor')], 'p2')
                : page([option('item:armor', 'armor', ['item:armor-1'])]);
            },
          },
        },
      ],
    });
    const cats = TestBed.inject(MagicCategories);
    await cats.load('c1');
    await cats.load('c1');
    expect(calls).toEqual(['', 'p2']);
    expect(cats.iconOf('item:ring')).toBe('shield');
    expect(cats.iconOf('item:armor-1')).toBe('shield');
    expect(cats.iconOf('item:nope')).toBe('diamond');
  });

  it("draws the icon of the category of an item, a variant's being its family's, and the gem for what it does not know", async () => {
    TestBed.configureTestingModule({
      providers: [
        {
          provide: AttunementClient,
          useValue: {
            listMagicItems: async () =>
              page([option('item:potion-healing', 'potion', ['item:potion-healing-greater'])]),
          },
        },
      ],
    });
    const cats = TestBed.inject(MagicCategories);
    expect(cats.iconOf('item:potion-healing')).toBe('diamond');
    await cats.load('c1');
    expect(cats.iconOf('item:potion-healing')).toBe('science');
    expect(cats.iconOf('item:potion-healing-greater')).toBe('science');
    expect(cats.iconOf('item:nope')).toBe('diamond');
  });

  it('keeps the gem on every row when the read fails, and tries again next time', async () => {
    let fail = true;
    TestBed.configureTestingModule({
      providers: [
        {
          provide: AttunementClient,
          useValue: {
            listMagicItems: async () => {
              if (fail) {
                throw new Error('offline');
              }
              return page([option('item:ring', 'armor')]);
            },
          },
        },
      ],
    });
    const cats = TestBed.inject(MagicCategories);
    await cats.load('c1');
    expect(cats.iconOf('item:ring')).toBe('diamond');
    fail = false;
    await cats.load('c1');
    expect(cats.iconOf('item:ring')).toBe('shield');
  });
});
