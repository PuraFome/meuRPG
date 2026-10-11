import { ABILITY_CHOICES, abilityMissing, needsAbility } from './ability-choice';

describe('ability choice', () => {
  it('lists the six abilities with the animal of the spell', () => {
    expect(ABILITY_CHOICES.map((a) => `${a.key} ${a.name} ${a.hint}`)).toEqual([
      'str Força Touro',
      'dex Destreza Gato',
      'con Constituição Urso',
      'int Inteligência Raposa',
      'wis Sabedoria Coruja',
      'cha Carisma Águia',
    ]);
  });

  it('asks only for Aprimorar Habilidade, and until an ability is chosen', () => {
    expect(needsAbility('spell:enhance-ability')).toBe(true);
    expect(needsAbility('spell:bless')).toBe(false);
    expect(abilityMissing('spell:enhance-ability', '')).toBe('Escolha a habilidade.');
    expect(abilityMissing('spell:enhance-ability', 'foo')).toBe('Escolha a habilidade.');
    expect(abilityMissing('spell:enhance-ability', 'cha')).toBe('');
    expect(abilityMissing('spell:bless', '')).toBe('');
  });
});
