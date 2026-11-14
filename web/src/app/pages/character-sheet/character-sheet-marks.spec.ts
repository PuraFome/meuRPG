import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';
import { createRouterTransport } from '@connectrpc/connect';

import {
  CharacterSchema,
  CharacterService,
  FullSheetSchema,
  GetCharacterResponseSchema,
  ItemBonusSchema,
  UpdateCharacterResponseSchema,
  type UpdateCharacterRequest,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import {
  AttackSchema,
  DerivedSheetSchema,
  GearEntrySchema,
  GearKind,
  ItemModifierSchema,
} from '../../../gen/meurpg/rules/v1/rules_pb';
import { GearClient, bonusChipName } from '../../core/characters/gear';
import { CONNECT_TRANSPORT } from '../../core/connect/transport';
import { marksOf, sheetGear, weaponMarksOf } from './character-sheet-marks';
import { CharacterSheetSourceLive } from './character-sheet-source.live';

const gear = [
  create(GearEntrySchema, {
    key: 'equipment:dagger',
    namePt: 'Adaga',
    kind: GearKind.WEAPON,
    equipped: true,
    count: 1,
  }),
  create(GearEntrySchema, {
    key: 'equipment:mace',
    namePt: 'Maça',
    kind: GearKind.WEAPON,
    count: 1,
  }),
  create(GearEntrySchema, {
    key: 'equipment:shield',
    namePt: 'Escudo',
    kind: GearKind.SHIELD,
    equipped: true,
    count: 1,
  }),
];
const derived = create(DerivedSheetSchema, {
  gear,
  attacks: [create(AttackSchema, { key: 'equipment:dagger', silvered: true, magical: true })],
});
const full = create(FullSheetSchema, {
  weaponMarks: [
    { weaponKey: 'equipment:mace', adamantine: true },
    { weaponKey: 'equipment:dagger', silvered: false },
  ],
});

describe('the marks of the weapons on the sheet', () => {
  it('reads the three marks of an attack or of a mark', () => {
    expect(marksOf({ silvered: true, adamantine: false, magical: true })).toEqual({
      silvered: true,
      adamantine: false,
      magical: true,
    });
    expect(weaponMarksOf(full)).toEqual([
      { weaponKey: 'equipment:mace', silvered: false, adamantine: true, magical: false },
      { weaponKey: 'equipment:dagger', silvered: false, adamantine: false, magical: false },
    ]);
  });

  it('gives an equipped weapon the marks of its attack (the engine’s: the sheet’s or a table rule’s), a carried one the sheet’s mark, armor and shield none', () => {
    const rows = sheetGear(derived, full);
    expect(rows.find((g) => g.key === 'equipment:dagger')!.marks).toEqual({
      silvered: true,
      adamantine: false,
      magical: true,
    });
    expect(rows.find((g) => g.key === 'equipment:mace')!.marks).toEqual({
      silvered: false,
      adamantine: true,
      magical: false,
    });
    expect(rows.find((g) => g.key === 'equipment:shield')!.marks).toBeUndefined();
  });

  it('leaves a weapon with neither an attack nor a mark without marks', () => {
    const rows = sheetGear(derived, create(FullSheetSchema, {}));
    expect(rows.find((g) => g.key === 'equipment:mace')!.marks).toBeUndefined();
  });
});

describe('the combat’s read of the gear carries the marks too', () => {
  it('maps them the same way as the sheet', async () => {
    const transport = createRouterTransport(({ service }) => {
      service(CharacterService, {
        getCharacter: () =>
          create(GetCharacterResponseSchema, {
            character: create(CharacterSchema, {
              revision: 4,
              derived,
              sheet: { content: { case: 'full', value: full } },
            }),
          }),
      });
    });
    TestBed.configureTestingModule({
      providers: [{ provide: CONNECT_TRANSPORT, useValue: transport }],
    });
    const read = await TestBed.inject(GearClient).read('c1', 'ch1');
    expect(read.gear.map((g) => [g.key, g.marks])).toEqual([
      ['equipment:dagger', { silvered: true, adamantine: false, magical: true }],
      ['equipment:mace', { silvered: false, adamantine: true, magical: false }],
      ['equipment:shield', undefined],
    ]);
  });
});

describe('saving the marks goes over the whole sheet and its revision', () => {
  it('sends the full sheet with only the marks changed, the revision read and the name', async () => {
    const sent: UpdateCharacterRequest[] = [];
    const base = create(FullSheetSchema, {
      armorKey: 'equipment:chain',
      shield: true,
      equipment: [{ name: 'Corda', quantity: 1 }],
    });
    const transport = createRouterTransport(({ service }) => {
      service(CharacterService, {
        updateCharacter: (req) => {
          sent.push(req);
          return create(UpdateCharacterResponseSchema, {
            character: create(CharacterSchema, { id: 'ch1', revision: 8, name: 'Fulano' }),
          });
        },
      });
    });
    TestBed.configureTestingModule({
      providers: [{ provide: CONNECT_TRANSPORT, useValue: transport }, CharacterSheetSourceLive],
    });
    await TestBed.inject(CharacterSheetSourceLive).saveGear('c1', 'ch1', 7, 'Fulano', base, {
      marks: [{ weaponKey: 'equipment:dagger', silvered: true, adamantine: false, magical: false }],
    });
    expect(sent).toHaveLength(1);
    expect(sent[0].revision).toBe(7);
    expect(sent[0].name).toBe('Fulano');
    const content = sent[0].sheet!.content;
    expect(content.case).toBe('full');
    const value = content.value as typeof base;
    expect(value.armorKey).toBe('equipment:chain');
    expect(value.shield).toBe(true);
    expect(value.equipment.map((e) => e.name)).toEqual(['Corda']);
    expect(value.weaponMarks.map((m) => [m.weaponKey, m.silvered])).toEqual([
      ['equipment:dagger', true],
    ]);
    expect(base.weaponMarks).toEqual([]); // the sheet on screen is not changed by the call
  });
});

describe('the magic bonus of a piece of gear', () => {
  const bonuses = [create(ItemBonusSchema, { key: 'equipment:mace', bonus: 2 })];
  const mod = (sourceKey: string, counts: boolean, reasonPt = '') =>
    create(ItemModifierSchema, { sourceKey, counts, target: 'attack', value: 2, reasonPt });
  const rowsWith = (mods: ReturnType<typeof mod>[]) =>
    sheetGear(
      create(DerivedSheetSchema, { gear, itemModifiers: mods }),
      create(FullSheetSchema, { itemBonuses: bonuses }),
    ).find((g) => g.key === 'equipment:mace')!;

  it('counts when every modifier of the piece counts', () => {
    const row = rowsWith([mod('equipment:mace', true), mod('equipment:mace', true)]);
    expect([row.bonus, row.bonusCounts]).toEqual([2, true]);
  });

  it('does not count when one modifier of the piece does not', () => {
    const row = rowsWith([mod('equipment:mace', true), mod('equipment:mace', false)]);
    expect([row.bonus, row.bonusCounts]).toEqual([2, false]);
  });

  it('gives the reason of the first modifier that does not count, and none when all count', () => {
    const row = rowsWith([
      mod('equipment:mace', true),
      mod('equipment:mace', false, 'Pede sintonização.'),
      mod('equipment:mace', false, 'Outro motivo.'),
    ]);
    expect(row.bonusReason).toBe('Pede sintonização.');
    expect(rowsWith([mod('equipment:mace', true)]).bonusReason).toBeUndefined();
    expect(rowsWith([mod('equipment:dagger', false, 'x')]).bonusReason).toBeUndefined();
  });

  it('counts when the piece has a bonus but no modifier, and ignores other pieces’ modifiers', () => {
    const row = rowsWith([mod('equipment:dagger', false)]);
    expect([row.bonus, row.bonusCounts]).toEqual([2, true]);
  });

  it('leaves a piece without a stored bonus plain', () => {
    const dagger = sheetGear(derived, create(FullSheetSchema, { itemBonuses: bonuses })).find(
      (g) => g.key === 'equipment:dagger',
    )!;
    expect(dagger.bonus).toBeUndefined();
    expect(dagger.bonusCounts).toBeUndefined();
  });

  it('gives several copies of one key the same bonus on their one row', () => {
    const two = create(DerivedSheetSchema, {
      gear: [create(GearEntrySchema, { key: 'equipment:mace', kind: GearKind.WEAPON, count: 2 })],
    });
    const rows = sheetGear(two, create(FullSheetSchema, { itemBonuses: bonuses }));
    expect(rows).toHaveLength(1);
    expect([rows[0].count, rows[0].bonus]).toEqual([2, 2]);
  });

  it('names the chip with and without the words', () => {
    expect(bonusChipName(1, true)).toBe('Bônus mágico +1, já somado');
    expect(bonusChipName(3, false)).toBe('Bônus mágico +3, não conta agora');
    expect(bonusChipName(1, true, true)).toBe('Bônus mágico, já somado');
    expect(bonusChipName(1, false, true)).toBe('Bônus mágico, não conta agora');
  });

  it('is read by the combat’s gear client too', async () => {
    const transport = createRouterTransport(({ service }) => {
      service(CharacterService, {
        getCharacter: () =>
          create(GetCharacterResponseSchema, {
            character: create(CharacterSchema, {
              derived: create(DerivedSheetSchema, {
                gear,
                itemModifiers: [mod('equipment:mace', false)],
              }),
              sheet: {
                content: { case: 'full', value: create(FullSheetSchema, { itemBonuses: bonuses }) },
              },
            }),
          }),
      });
    });
    TestBed.configureTestingModule({
      providers: [{ provide: CONNECT_TRANSPORT, useValue: transport }],
    });
    const read = await TestBed.inject(GearClient).read('c1', 'ch1');
    const mace = read.gear.find((g) => g.key === 'equipment:mace')!;
    expect([mace.bonus, mace.bonusCounts]).toEqual([2, false]);
  });
});

describe('saving the gear change over the whole sheet', () => {
  async function save(base: ReturnType<typeof create<typeof FullSheetSchema>>, change: object) {
    const sent: UpdateCharacterRequest[] = [];
    const transport = createRouterTransport(({ service }) => {
      service(CharacterService, {
        updateCharacter: (req) => {
          sent.push(req);
          return create(UpdateCharacterResponseSchema, {
            character: create(CharacterSchema, { id: 'ch1', revision: 8, name: 'Fulano' }),
          });
        },
      });
    });
    TestBed.configureTestingModule({
      providers: [{ provide: CONNECT_TRANSPORT, useValue: transport }, CharacterSheetSourceLive],
    });
    await TestBed.inject(CharacterSheetSourceLive).saveGear('c1', 'ch1', 7, 'Fulano', base, change);
    expect(sent).toHaveLength(1);
    return sent[0].sheet!.content.value as typeof base;
  }
  const entry = {
    key: 'equipment:mace',
    bonus: 1,
    requiresAttunement: true,
    attuned: true,
    itemKey: 'magic:mace',
    id: 'i1',
  };

  it('sets the bonus on an existing entry and keeps the rest of it', async () => {
    const base = create(FullSheetSchema, { itemBonuses: [entry] });
    const value = await save(base, { bonus: { key: 'equipment:mace', bonus: 3 } });
    expect(value.itemBonuses).toHaveLength(1);
    expect(value.itemBonuses[0]).toMatchObject({ ...entry, bonus: 3 });
    expect(base.itemBonuses[0].bonus).toBe(1);
  });

  it('appends an entry for a key that has none', async () => {
    const value = await save(create(FullSheetSchema, { itemBonuses: [entry] }), {
      bonus: { key: 'equipment:dagger', bonus: 2 },
    });
    expect(value.itemBonuses.map((b) => [b.key, b.bonus])).toEqual([
      ['equipment:mace', 1],
      ['equipment:dagger', 2],
    ]);
  });

  it('removes the entry when the bonus is 0', async () => {
    const value = await save(create(FullSheetSchema, { itemBonuses: [entry] }), {
      bonus: { key: 'equipment:mace', bonus: 0 },
    });
    expect(value.itemBonuses).toEqual([]);
  });

  it('removes the whole entry of an attuned piece on "Nenhum": its attunement goes with it', async () => {
    const other = { ...entry, key: 'equipment:dagger', id: 'i2', itemKey: 'magic:dagger' };
    const value = await save(create(FullSheetSchema, { itemBonuses: [entry, other] }), {
      bonus: { key: 'equipment:mace', bonus: 0 },
    });
    expect(value.itemBonuses).toHaveLength(1);
    expect(value.itemBonuses[0]).toMatchObject({
      key: 'equipment:dagger',
      attuned: true,
      requiresAttunement: true,
      itemKey: 'magic:dagger',
    });
    expect(value.itemBonuses.some((b) => b.itemKey === 'magic:mace' || b.id === 'i1')).toBe(false);
  });

  it('sends the marks and the bonus in one request', async () => {
    const value = await save(create(FullSheetSchema, {}), {
      marks: [{ weaponKey: 'equipment:mace', silvered: true, adamantine: false, magical: false }],
      bonus: { key: 'equipment:mace', bonus: 1 },
    });
    expect(value.weaponMarks.map((m) => m.weaponKey)).toEqual(['equipment:mace']);
    expect(value.itemBonuses.map((b) => [b.key, b.bonus])).toEqual([['equipment:mace', 1]]);
  });
});
