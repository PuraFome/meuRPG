import { EffectMenuVm, draftToEffect, emptyEffect, missingRequired } from './effect-draft';
import type { EffectDraft } from './effect-draft';
import { menuResponse, stateMenu } from './content-testing-menu';

/** A menu that reads exactly the fields it is given, so a case can ask for any of them. */
function menuReading(names: readonly string[]): EffectMenuVm {
  const res = menuResponse();
  res.types = [{ ...res.types[0], type: 'any', fields: names.map((name) => ({ name })) } as never];
  return new EffectMenuVm(res);
}

const ALL = [
  'target',
  'mode',
  'value',
  'when',
  'tags',
  'proficiency',
  'level',
  'roll',
  'targets',
  'sense',
  'range_ft',
  'resource',
  'max',
  'recharge',
  'choice',
  'count',
  'from',
  'economy',
  'applies',
  'ability',
  'spells',
  'text_pt',
  'ignore_cover',
  'range_multiplier',
  'no_long_range_disadvantage',
  'cost',
  'cost_max',
  'value_per_point',
  'heal',
  'temp_hp',
  'event',
  'restore',
  'heal_percent_max',
  'unknown_to_the_app',
];

const filled = (): EffectDraft => ({
  ...emptyEffect('any'),
  target: 'attack_roll',
  mode: 'add',
  value: ' 2 ',
  when: ' ao atacar ',
  tags: ' against:magic, , about:items ',
  proficiency: 'skill:arcana',
  level: '3',
  roll: 'advantage',
  targets: ['a', 'b'],
  sense: 'darkvision',
  rangeM: '18',
  resource: ' vigilia ',
  max: ' 3 ',
  recharge: 'dawn',
  choice: 'skill',
  count: 2,
  from: ['skill:arcana'],
  economy: 'bonus',
  applies: 'weapon',
  ability: 'strength',
  spells: ['spell:light'],
  textPt: ' Texto {value} ',
  ignoreCover: true,
  rangeMultiplier: 2,
  noLongRangeDisadvantage: true,
  cost: ' 2 ',
  costMax: ' prof() ',
  valuePerPoint: ' 1d6 ',
  heal: ' 1d6+2 ',
  tempHp: ' level() ',
  event: 'long_rest',
  restore: ' reserva_de_teste ',
  healPercentMax: 50,
});

describe('the effect a request sends, field by field', () => {
  it('sends every field the type reads, trimmed, and nothing the type does not read', () => {
    const sent = draftToEffect(filled(), menuReading(ALL));
    expect(sent).toEqual({
      type: 'any',
      target: 'attack_roll',
      mode: 'add',
      value: '2',
      when: 'ao atacar',
      tags: ['against:magic', 'about:items'],
      proficiency: 'skill:arcana',
      level: '3',
      roll: 'advantage',
      targets: ['a', 'b'],
      sense: 'darkvision',
      rangeFt: 60,
      resource: 'vigilia',
      max: '3',
      recharge: 'dawn',
      choice: 'skill',
      count: 2,
      from: ['skill:arcana'],
      economy: 'bonus',
      applies: 'weapon',
      ability: 'strength',
      spells: ['spell:light'],
      textPt: 'Texto {value}',
      ignoreCover: true,
      rangeMultiplier: 2,
      noLongRangeDisadvantage: true,
      cost: '2',
      costMax: 'prof()',
      valuePerPoint: '1d6',
      heal: '1d6+2',
      tempHp: 'level()',
      event: 'long_rest',
      restore: 'reserva_de_teste',
      healPercentMax: 50,
    });
    expect(draftToEffect(filled(), menuReading(['value', 'count']))).toEqual({
      type: 'any',
      value: '2',
      count: 2,
    });
  });

  it('sends nothing of an empty field, an empty list, a zero count or a switch that is off', () => {
    expect(draftToEffect(emptyEffect('any'), menuReading(ALL))).toEqual({ type: 'any' });
    expect(
      draftToEffect(
        { ...emptyEffect('any'), value: '  ', tags: ' , ', rangeM: 'abc' },
        menuReading(ALL),
      ),
    ).toEqual({ type: 'any' });
  });

  it('copies the lists, so the draft keeps its own', () => {
    const d = filled();
    const sent = draftToEffect(d, menuReading(['targets', 'from', 'spells']));
    expect(sent.targets).toEqual(d.targets);
    expect(sent.targets).not.toBe(d.targets);
    expect(sent.from).not.toBe(d.from);
    expect(sent.spells).not.toBe(d.spells);
  });

  it('sends "Só onde já tem proficiência" only with the level Especialização on a skill, and only when on', () => {
    const menu = menuReading(['proficiency', 'level', 'only_proficient']);
    const expertise = {
      ...emptyEffect('any'),
      proficiency: 'skill:perception',
      level: 'expertise',
      onlyProficient: true,
    };
    expect(draftToEffect(expertise, menu)).toEqual({
      type: 'any',
      proficiency: 'skill:perception',
      level: 'expertise',
      onlyProficient: true,
    });
    expect(draftToEffect({ ...expertise, onlyProficient: false }, menu).onlyProficient).toBe(
      undefined,
    );
    // Another level or a target that is not a skill: the switch stays in the draft, the server never hears of it.
    expect(draftToEffect({ ...expertise, level: 'full' }, menu).onlyProficient).toBe(undefined);
    expect(draftToEffect({ ...expertise, proficiency: 'save.wis' }, menu).onlyProficient).toBe(
      undefined,
    );
    expect(draftToEffect({ ...expertise, proficiency: 'skill:*' }, menu).onlyProficient).toBe(true);
    // A type that does not read it never sends it.
    expect(draftToEffect(expertise, menuReading(['proficiency', 'level'])).onlyProficient).toBe(
      undefined,
    );
  });

  it('writes nothing for the name of a field that is also a name of Object', () => {
    expect(draftToEffect(emptyEffect('any'), menuReading(['constructor', 'toString']))).toEqual({
      type: 'any',
    });
  });
});

describe('the state and weapon effects of the menu', () => {
  it('sends a state with its name, duration and ways to end, and nothing it does not read', () => {
    const d: EffectDraft = {
      ...emptyEffect('state'),
      stateName: ' postura ',
      durationRounds: '10',
      endsOn: ['dismiss', 'long_rest'],
      economy: 'bonus_action',
      attackBonus: 3,
    };
    const e = draftToEffect(d, stateMenu());
    expect(e.type).toBe('state');
    expect(e.stateName).toBe('postura');
    expect(e.durationRounds).toBe('10');
    expect(e.endsOn).toEqual(['dismiss', 'long_rest']);
    expect(e.economy).toBe('bonus_action');
    expect(e.attackBonus).toBeUndefined();
  });

  it('sends a weapon rule with signed bonuses and the reach in feet', () => {
    const d: EffectDraft = {
      ...emptyEffect('weapon_rule'),
      weapon: 'melee',
      attackBonus: -2,
      damageBonus: 1,
      reachM: '3',
      while: 'postura',
    };
    const e = draftToEffect(d, stateMenu());
    expect(e.attackBonus).toBe(-2);
    expect(e.damageBonus).toBe(1);
    expect(e.reachFt).toBe(10);
    expect(e.while).toBe('postura');
  });

  it('asks for the name of a state before the server does', () => {
    expect(missingRequired(emptyEffect('state'), stateMenu())).toContain('state_name');
  });
});
