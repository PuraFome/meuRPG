import {
  Alignment,
  BasicSheet,
  Character,
  CharacterKind,
  CharacterState,
  CharacterStory,
  FullSheet,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { DerivedSheet } from '../../../gen/meurpg/rules/v1/rules_pb';
import { toCharacterSheetVm, toCharacterStoryInit, toStoryVm } from './character-sheet-source.live';

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

/** Just enough to be a valid `DerivedSheet` — every array present, no
 * values these tests care about (alignment and XP come from `FullSheet`,
 * not `DerivedSheet` — that is the whole point of the follow-up). */
function minimalDerivedSheet(): DerivedSheet {
  return {
    $typeName: 'meurpg.rules.v1.DerivedSheet',
    contentVersion: 'srd51@test',
    raceNamePt: 'Gnomo da Rocha',
    subraceNamePt: '',
    backgroundNamePt: 'Sábio',
    classes: [],
    totalLevel: 3,
    proficiencyBonus: 2,
    abilities: [],
    savingThrows: [],
    skills: [],
    passivePerception: 10,
    passiveInvestigation: 10,
    passiveInsight: 10,
    initiative: 0,
    armorClass: 10,
    armorClassDescription: '',
    hitPointsMax: 10,
    hitDice: [],
    speedWalkFt: 25,
    senses: [],
    spellcasting: [],
    spellSlots: [],
    spells: [],
    attacks: [],
    features: [],
    languages: [],
    hints: [],
    issues: [],
  };
}

function minimalFullSheet(overrides: Partial<FullSheet> = {}): FullSheet {
  return {
    $typeName: 'meurpg.characters.v1.FullSheet',
    baseScores: undefined,
    raceKey: 'race:gnome',
    subraceKey: 'subrace:rock-gnome',
    classes: [],
    background: { case: 'backgroundKey', value: 'background:acolyte' },
    skillProficiencyKeys: [],
    expertiseSkillKeys: [],
    extraAbilityBonuses: undefined,
    hitPoints: undefined,
    armorKey: '',
    shield: false,
    weaponKeys: [],
    cantripKeys: [],
    knownSpellKeys: [],
    preparedSpellKeys: [],
    equipment: [],
    coins: undefined,
    languages: [],
    toolProficiencies: [],
    experiencePoints: 0,
    alignment: Alignment.UNSPECIFIED,
    customFeaturesText: '',
    featureChoiceKeys: [],
    ...overrides,
  };
}

function characterWithFullSheet(full: FullSheet): Character {
  return {
    $typeName: 'meurpg.characters.v1.Character',
    id: 'char-1',
    campaignId: 'camp-1',
    kind: CharacterKind.PLAYER,
    state: CharacterState.DRAFT,
    name: 'Pensantus',
    playerUserId: 'user-1',
    playerDisplayName: 'Vinicius',
    sheet: { $typeName: 'meurpg.characters.v1.CharacterSheet', content: { case: 'full', value: full } },
    story: undefined,
    derived: minimalDerivedSheet(),
    revision: 1,
    sheetLockedAt: undefined,
    diedAt: undefined,
    createdAt: undefined,
    updatedAt: undefined,
    canEdit: true,
    canEditStory: true,
    canMarkDead: false,
    canAccessMasterNotes: false,
    storyEditingAllowed: false,
    canSetStoryEditing: false,
  };
}

describe('the sheet header shows alignment and XP, read from the stored FullSheet (integrator follow-up)', () => {
  it('maps a chosen alignment to its Portuguese label', () => {
    const vm = toCharacterSheetVm(
      characterWithFullSheet(minimalFullSheet({ alignment: Alignment.CHAOTIC_GOOD })),
    );
    expect(vm.alignmentLabel).toBe('Caótico e bom');
  });

  it('maps an unset alignment to an empty label — "nothing when unset"', () => {
    const vm = toCharacterSheetVm(
      characterWithFullSheet(minimalFullSheet({ alignment: Alignment.UNSPECIFIED })),
    );
    expect(vm.alignmentLabel).toBe('');
  });

  it('reads experience points straight from the FullSheet, 0 included', () => {
    const vm = toCharacterSheetVm(characterWithFullSheet(minimalFullSheet({ experiencePoints: 0 })));
    expect(vm.experiencePoints).toBe(0);

    const vmWithXp = toCharacterSheetVm(
      characterWithFullSheet(minimalFullSheet({ experiencePoints: 2700 })),
    );
    expect(vmWithXp.experiencePoints).toBe(2700);
  });

  it('has no alignment or XP for a BasicSheet NPC', () => {
    const basic: BasicSheet = {
      $typeName: 'meurpg.characters.v1.BasicSheet',
      hitPointsMax: 7,
      armorClass: 13,
      speedFt: 30,
      attackBonus: 3,
      damage: '1d6+1 perfurante',
      description: 'Um goblin arisco.',
    };
    const character: Character = {
      ...characterWithFullSheet(minimalFullSheet()),
      kind: CharacterKind.MINION,
      sheet: {
        $typeName: 'meurpg.characters.v1.CharacterSheet',
        content: { case: 'basic', value: basic },
      },
      derived: undefined,
    };

    const vm = toCharacterSheetVm(character);
    expect(vm.alignmentLabel).toBe('');
    expect(vm.experiencePoints).toBeNull();
  });
});
