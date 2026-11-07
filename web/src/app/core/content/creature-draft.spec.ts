import { create } from '@bufbuild/protobuf';

import {
  TableCreatureDamageModSchema,
  TableCreatureSchema,
} from '../../../gen/meurpg/rules/v1/table_content_pb';
import { emptyDamageMod } from './creature-damage-mods';
import {
  DICE_SIDES,
  MAX_ATTACKS,
  MAX_TRAITS,
  creaturePaths,
  creatureToDraft,
  draftToCreature,
  emptyAttack,
  emptyCreature,
  emptyTrait,
  unreadableCreaturePaths,
} from './creature-draft';

const mods = [
  create(TableCreatureDamageModSchema, {
    kind: 'resist',
    damageTypes: ['damage-type:slashing'],
    qualifier: 'nonmagical',
  }),
];

const stored = () =>
  create(TableCreatureSchema, {
    namePt: 'Lobo do Vale',
    baseKey: 'monster:wolf',
    hitPoints: 40,
    armorClass: 17,
    damageBonus: -2,
    hitPointsFormula: '4 + 4 * level()',
    armorClassFormula: '13 + prof()',
    damageBonusFormula: 'prof()',
    attackBonusFormula: 'mod("wis") + prof()',
    traits: [{ namePt: 'Elo com o guardião', textPt: 'Entende o dono.' }],
    attacks: [
      {
        namePt: 'Investida do Vale',
        melee: true,
        reachFt: 5,
        diceCount: 2,
        diceSides: 6,
        damageType: 'damage-type:bludgeoning',
        toHit: 'prof()',
        damageBonus: '1',
      },
      {
        namePt: 'Uivo cortante',
        melee: false,
        rangeFt: 30,
        longRangeFt: 120,
        diceCount: 1,
        diceSides: 8,
        damageType: 'damage-type:slashing',
      },
    ],
    damageMods: mods,
  });

describe('the creature form (#578)', () => {
  it('turns a stored creature into a draft and back to the same body, formulas as typed', () => {
    const draft = creatureToDraft(stored());
    expect(draft.hitPoints).toBe('40');
    expect(draft.hitPointsFormula).toBe('4 + 4 * level()');
    expect(draft.attacks[0].reachM).toBe('1,5');
    const init = draftToCreature(draft);
    expect(init).toMatchObject({
      namePt: 'Lobo do Vale',
      baseKey: 'monster:wolf',
      hitPoints: 40,
      armorClass: 17,
      damageBonus: -2,
      hitPointsFormula: '4 + 4 * level()',
      armorClassFormula: '13 + prof()',
      damageBonusFormula: 'prof()',
      attackBonusFormula: 'mod("wis") + prof()',
      traits: [{ namePt: 'Elo com o guardião', textPt: 'Entende o dono.' }],
    });
    expect(init.attacks).toEqual([
      {
        namePt: 'Investida do Vale',
        melee: true,
        reachFt: 5,
        rangeFt: 0,
        longRangeFt: 0,
        diceCount: 2,
        diceSides: 6,
        damageType: 'damage-type:bludgeoning',
        damageBonus: '1',
        toHit: 'prof()',
      },
      {
        namePt: 'Uivo cortante',
        melee: false,
        reachFt: 0,
        rangeFt: 30,
        longRangeFt: 120,
        diceCount: 1,
        diceSides: 8,
        damageType: 'damage-type:slashing',
        damageBonus: '',
        toHit: '',
      },
    ]);
  });

  it('turns the resistances, immunities and vulnerabilities into rows and back, as the entry has them', () => {
    const draft = creatureToDraft(stored());
    expect(draft.damageMods).toHaveLength(1);
    expect(draft.damageMods[0]).toMatchObject({
      kind: 'resist',
      types: ['damage-type:slashing'],
      qualifier: 'nonmagical',
    });
    const init = draftToCreature({ ...draft, name: 'Outro nome', hitPoints: '41' });
    expect(init.damageMods).toEqual([
      { kind: 'resist', damageTypes: ['damage-type:slashing'], qualifier: 'nonmagical' },
    ]);
    // A row the master edits is what goes out, and a new creature has none.
    draft.damageMods[0].types.push('damage-type:piercing');
    draft.damageMods.push({ ...emptyDamageMod(), kind: 'vulnerable', types: ['damage-type:fire'] });
    expect(draftToCreature(draft).damageMods).toHaveLength(2);
    expect(draftToCreature(draft).damageMods?.[0]).toMatchObject({
      damageTypes: ['damage-type:slashing', 'damage-type:piercing'],
    });
    expect(draftToCreature(emptyCreature()).damageMods).toEqual([]);
  });

  it('lists the paths of the rows of resistances', () => {
    const paths = creaturePaths(creatureToDraft(stored()));
    expect(paths).toEqual(
      expect.arrayContaining([
        'table_creature.damage_mods',
        'table_creature.damage_mods[0].kind',
        'table_creature.damage_mods[0].damage_types',
        'table_creature.damage_mods[0].qualifier',
      ]),
    );
    expect(paths).not.toContain('table_creature.damage_mods[1].kind');
  });

  it('sends empty numbers as 0 (the base keeps its own) and trims names, texts and formulas', () => {
    const d = {
      ...emptyCreature(),
      name: '  Lobo  ',
      baseKey: 'monster:wolf',
      hitPointsFormula: ' prof() ',
      traits: [{ ...emptyTrait(), name: ' Faro ', text: ' Sente o cheiro. ' }],
    };
    expect(draftToCreature(d)).toMatchObject({
      namePt: 'Lobo',
      hitPoints: 0,
      armorClass: 0,
      damageBonus: 0,
      hitPointsFormula: 'prof()',
      traits: [{ namePt: 'Faro', textPt: 'Sente o cheiro.' }],
    });
  });

  it('writes distances in feet from metres and sends only the reach of a melee attack or the range of a ranged one', () => {
    const melee = { ...emptyAttack(), reachM: '3', rangeM: '9', longRangeM: '18' };
    expect(draftToCreature({ ...emptyCreature(), attacks: [melee] }).attacks?.[0]).toMatchObject({
      reachFt: 10,
      rangeFt: 0,
      longRangeFt: 0,
    });
    const ranged = { ...melee, melee: false };
    expect(draftToCreature({ ...emptyCreature(), attacks: [ranged] }).attacks?.[0]).toMatchObject({
      reachFt: 0,
      rangeFt: 30,
      longRangeFt: 60,
    });
    // An empty reach is the server's 0 (1,5 m).
    expect(
      draftToCreature({ ...emptyCreature(), attacks: [emptyAttack()] }).attacks?.[0],
    ).toMatchObject({ reachFt: 0 });
  });

  it('shows feet as metres with the decimal comma', () => {
    const c = create(TableCreatureSchema, {
      attacks: [{ namePt: 'A', melee: false, rangeFt: 25, longRangeFt: 75 }],
    });
    const a = creatureToDraft(c).attacks[0];
    expect(a.rangeM).toBe('7,5');
    expect(a.longRangeM).toBe('22,5');
    expect(a.reachM).toBe('');
  });

  it('names the number fields it cannot read, in the rows that hold them', () => {
    const d = {
      ...emptyCreature(),
      hitPoints: '12x',
      damageBonus: '+3',
      attacks: [
        { ...emptyAttack(), diceCount: 'dois', reachM: '1,5' },
        { ...emptyAttack(), melee: false, rangeM: 'longe', longRangeM: '9' },
      ],
    };
    expect(unreadableCreaturePaths(d)).toEqual([
      'table_creature.hit_points',
      'table_creature.attacks[0].dice_count',
      'table_creature.attacks[1].range_ft',
    ]);
    expect(unreadableCreaturePaths(creatureToDraft(stored()))).toEqual([]);
  });

  it('lists the paths of the inputs it draws, with those of every trait and attack', () => {
    const paths = creaturePaths(creatureToDraft(stored()));
    expect(paths).toEqual(
      expect.arrayContaining([
        'table_creature.name_pt',
        'table_creature.base_key',
        'table_creature.hit_points_formula',
        'table_creature.attack_bonus_formula',
        'table_creature.traits',
        'table_creature.traits[0].name_pt',
        'table_creature.traits[0].text_pt',
        'table_creature.attacks',
        'table_creature.attacks[1].long_range_ft',
        'table_creature.attacks[1].damage_type',
        'table_creature.attacks[1].to_hit',
      ]),
    );
    expect(paths).not.toContain('table_creature.attacks[2].name_pt');
    // no element carries a whole row: a refusal of one climbs to its list
    expect(paths).not.toContain('table_creature.traits[0]');
    expect(paths).not.toContain('table_creature.attacks[0]');
  });

  it('has the limits of the server', () => {
    expect(MAX_ATTACKS).toBe(8);
    expect(MAX_TRAITS).toBe(10);
    // the dice that exist at a table, the d100 among them
    expect(DICE_SIDES).toEqual(['4', '6', '8', '10', '12', '20', '100']);
  });
});
