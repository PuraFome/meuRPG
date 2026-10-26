import { create } from '@bufbuild/protobuf';

import { RollDieSchema } from '../../../gen/meurpg/rules/v1/rules_pb';
import { rollDiceFor, rollDieCost, rollDieField, sceneRollKind } from './roll-die';

const die = (key: string, applies: string, over: Record<string, unknown> = {}) =>
  create(RollDieSchema, {
    key,
    namePt: key,
    applies,
    sides: 8,
    resource: 'dados_de_vigor',
    ...over,
  });

describe('the table dice a roll offers', () => {
  const any = die('any-check', 'check');
  const stealth = die('stealth', 'check', { targets: ['skill.stealth'] });
  const dexterity = die('dex', 'check', { targets: ['check.dex'] });
  const wisSave = die('wis-save', 'save', { ability: 'wis' });
  const anySave = die('any-save', 'save');
  const init = die('init', 'initiative');
  const all = [any, stealth, dexterity, wisSave, anySave, init];
  const keys = (r: ReturnType<typeof sceneRollKind>) => rollDiceFor(all, r).map((d) => d.key);

  it('reads what a scene action rolls', () => {
    expect(sceneRollKind('skill:stealth')).toEqual({
      kind: 'check',
      skill: 'stealth',
      ability: 'dex',
    });
    expect(sceneRollKind('ability:str')).toEqual({ kind: 'check', ability: 'str' });
    expect(sceneRollKind('save:wis')).toEqual({ kind: 'save', ability: 'wis' });
    expect(sceneRollKind('other:x')).toBeNull();
  });

  it('offers a check die with no list on every check, and a limited one only where its list names the skill or the ability', () => {
    expect(keys(sceneRollKind('skill:stealth'))).toEqual(['any-check', 'stealth', 'dex']);
    expect(keys(sceneRollKind('skill:acrobatics'))).toEqual(['any-check', 'dex']);
    expect(keys(sceneRollKind('skill:arcana'))).toEqual(['any-check']);
    expect(keys(sceneRollKind('ability:dex'))).toEqual(['any-check', 'dex']);
  });

  it('offers a save die for any save, or only for its ability, and the initiative die on the initiative alone', () => {
    expect(keys({ kind: 'save', ability: 'wis' })).toEqual(['wis-save', 'any-save']);
    expect(keys({ kind: 'save', ability: 'con' })).toEqual(['any-save']);
    expect(keys({ kind: 'initiative' })).toEqual(['init']);
    expect(keys(null)).toEqual([]);
  });

  it('asks the face of the die on a physical roll and says what it costs', () => {
    expect(rollDieField(any)).toMatchObject({
      label: 'Resultado do d8 (any-check)',
      faces: 8,
      sign: 1,
    });
    expect(rollDieCost(any)).toBe('d8 · gasta 1 uso de dados de vigor');
    expect(rollDieCost(die('free', 'check', { resource: '' }))).toBe('d8');
  });
});
