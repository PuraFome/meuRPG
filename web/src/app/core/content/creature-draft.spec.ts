import { create } from '@bufbuild/protobuf';

import {
  TableCreatureDamageModSchema,
  TableCreatureSchema,
} from '../../../gen/meurpg/rules/v1/table_content_pb';
import {
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

  it('keeps the resistances, immunities and vulnerabilities as the entry has them, with no row for them', () => {
    const draft = creatureToDraft(stored());
    expect(draft.kept?.damageMods).toHaveLength(1);
    const init = draftToCreature({ ...draft, name: 'Outro nome', hitPoints: '41' });
    expect(init.damageMods).toHaveLength(1);
    expect(init.damageMods?.[0]).toMatchObject({
      kind: 'resist',
      damageTypes: ['damage-type:slashing'],
      qualifier: 'nonmagical',
    });
    // A new creature has none.
    expect(draftToCreature(emptyCreature()).damageMods).toEqual([]);
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
  });

  it('has the limits of the server', () => {
    expect(MAX_ATTACKS).toBe(8);
    expect(MAX_TRAITS).toBe(10);
  });
});
