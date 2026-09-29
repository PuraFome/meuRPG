import { CharacterStory } from '../../../gen/meurpg/characters/v1/characters_pb';
import { toCharacterStoryInit, toStoryVm } from './character-sheet-source.live';

describe('story round-trips load → save unchanged (integrator fix, phase 2b)', () => {
  it('a fully populated CharacterStory survives toStoryVm → toCharacterStoryInit', () => {
    const loaded: CharacterStory = {
      $typeName: 'meurpg.characters.v1.CharacterStory',
      personality: {
        $typeName: 'meurpg.characters.v1.Personality',
        traits: 'Fala sozinho quando pensa.',
        ideals: 'Conhecimento acima de tudo.',
        bonds: 'Deve à sua guilda.',
        flaws: 'Curioso demais para o próprio bem.',
      },
      appearance: {
        $typeName: 'meurpg.characters.v1.Appearance',
        age: '112 anos',
        height: '1,05 m',
        weight: '20 kg',
        eyes: 'castanhos',
        skin: 'bronzeada',
        hair: 'grisalho, curto',
        description: 'Sempre com um livro debaixo do braço.',
      },
      backstory: 'Cresceu numa vila de gnomos nas colinas.',
      allies: 'A Guilda dos Arcanistas.',
    };

    const vm = toStoryVm(loaded);
    const wireInit = toCharacterStoryInit(vm);

    // Every field the server sent comes back exactly as it was — nothing
    // the story form doesn't show (there is nothing it doesn't show) gets
    // silently dropped or blanked.
    expect(wireInit).toEqual({
      personality: {
        traits: loaded.personality!.traits,
        ideals: loaded.personality!.ideals,
        bonds: loaded.personality!.bonds,
        flaws: loaded.personality!.flaws,
      },
      appearance: {
        age: loaded.appearance!.age,
        height: loaded.appearance!.height,
        weight: loaded.appearance!.weight,
        eyes: loaded.appearance!.eyes,
        skin: loaded.appearance!.skin,
        hair: loaded.appearance!.hair,
        description: loaded.appearance!.description,
      },
      backstory: loaded.backstory,
      allies: loaded.allies,
    });
  });

  it('an unset story maps to all-empty fields, never undefined', () => {
    const vm = toStoryVm(undefined);
    expect(vm).toEqual({
      personality: { traits: '', ideals: '', bonds: '', flaws: '' },
      appearance: { age: '', height: '', weight: '', eyes: '', skin: '', hair: '', description: '' },
      backstory: '',
      allies: '',
    });
  });
});
