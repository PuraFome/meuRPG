import { create } from '@bufbuild/protobuf';

import {
  LevelUpChoicesSchema,
  LevelUpHitPointsMethod,
  LevelUpSchema,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { choiceRows } from './levelup-changes';

const levelUp = (choices: object) =>
  create(LevelUpSchema, {
    characterName: 'Pensantus',
    toLevel: 4,
    namesPt: {
      'spell:light': 'Luz',
      'spell:misty-step': 'Passo Nebuloso',
      'spell:mirror-image': 'Reflexos',
      'sub:x': 'Escola X',
    },
    choices: create(LevelUpChoicesSchema, choices),
  });

describe('choiceRows: the master\'s "O que mudou"', () => {
  it('words the ability, the hit points and the spells by their Portuguese names', () => {
    const rows = choiceRows(
      levelUp({
        abilityIncrease: { intelligence: 2 },
        hitPoints: { method: LevelUpHitPointsMethod.AVERAGE, value: 4 },
        cantripKeys: ['spell:light'],
        knownSpellKeys: ['spell:misty-step', 'spell:mirror-image'],
        preparedSpellKeys: ['spell:misty-step'],
      }),
    );
    expect(rows).toEqual([
      { label: 'Habilidades', value: '+2 em Inteligência' },
      { label: 'Pontos de vida', value: 'Média: 4, mais o modificador de Constituição' },
      { label: 'Truque novo', value: 'Luz' },
      { label: 'Magias novas', value: 'Passo Nebuloso e Reflexos' },
      { label: 'Magias preparadas novas', value: 'Passo Nebuloso' },
    ]);
  });

  it('says how the die was rolled, and +1 in two', () => {
    const rows = choiceRows(
      levelUp({
        abilityIncrease: { strength: 1, wisdom: 1 },
        hitPoints: { method: LevelUpHitPointsMethod.ROLLED_PHYSICAL, value: 5 },
        subclassKey: 'sub:x',
      }),
    );
    expect(rows[0].value).toBe('+1 em Força e +1 em Sabedoria');
    expect(rows[1].value).toContain('Dado físico: 5');
    expect(rows[2]).toEqual({ label: 'Subclasse', value: 'Escola X' });
  });

  it('leaves out what was not chosen', () => {
    expect(
      choiceRows(
        levelUp({ hitPoints: { method: LevelUpHitPointsMethod.ROLLED_IN_APP, value: 3 } }),
      ).map((r) => r.label),
    ).toEqual(['Pontos de vida']);
  });

  it('names the feat the player took in place of the increase', () => {
    const l = levelUp({ featKey: 'feat:atleta@mesa', abilityIncrease: { dexterity: 1 } });
    l.namesPt['feat:atleta@mesa'] = 'Atleta';
    expect(choiceRows(l).map((r) => [r.label, r.value])).toEqual([
      ['Habilidades', '+1 em Destreza'],
      ['Talento', 'Atleta'],
    ]);
  });
});
