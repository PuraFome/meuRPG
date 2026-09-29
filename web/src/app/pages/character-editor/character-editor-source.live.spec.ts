import {
  Alignment,
  FullSheet,
  HitPointsMethod,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { mergeFullSheetInit, toFormFullSheet, toFullSheetInit } from './character-editor-source.live';

/**
 * A fully populated `FullSheet` — every field set, including the two the
 * editor form has no UI for (`coins`, `feature_choice_keys`) — for the
 * integrator's no-data-loss fix (phase 2b): loading it, then saving without
 * touching anything, must send back exactly what was loaded.
 *
 * One class only: the MVP editor's own, already-documented scope limit
 * (one class; more is multiclassing) is not what this test is about — it
 * covers everything the editor's fields actually claim to edit.
 */
function fullyPopulatedFullSheet(): FullSheet {
  return {
    $typeName: 'meurpg.characters.v1.FullSheet',
    baseScores: {
      $typeName: 'meurpg.rules.v1.AbilityScores',
      strength: 8,
      dexterity: 14,
      constitution: 16,
      intelligence: 18,
      wisdom: 12,
      charisma: 10,
    },
    raceKey: 'race:gnome',
    subraceKey: 'subrace:rock-gnome',
    classes: [
      {
        $typeName: 'meurpg.characters.v1.ClassLevel',
        classKey: 'class:wizard',
        level: 3,
        subclass: { case: 'subclassKey', value: 'subclass:evocation' },
      },
    ],
    background: {
      case: 'backgroundKey',
      value: 'background:acolyte',
    },
    skillProficiencyKeys: ['skill:arcana', 'skill:history', 'skill:investigation'],
    expertiseSkillKeys: ['skill:arcana'],
    extraAbilityBonuses: {
      $typeName: 'meurpg.rules.v1.AbilityScores',
      strength: 0,
      dexterity: 0,
      constitution: 1,
      intelligence: 2,
      wisdom: 0,
      charisma: 0,
    },
    hitPoints: {
      $typeName: 'meurpg.characters.v1.HitPoints',
      method: HitPointsMethod.ROLLED,
      rolls: [4, 6, 2],
    },
    armorKey: '',
    shield: false,
    weaponKeys: ['equipment:quarterstaff', 'equipment:dagger'],
    cantripKeys: ['spell:fire-bolt', 'spell:ray-of-frost', 'spell:minor-illusion'],
    knownSpellKeys: [
      'spell:magic-missile',
      'spell:burning-hands',
      'spell:shield',
      'spell:mage-armor',
    ],
    preparedSpellKeys: ['spell:magic-missile', 'spell:shield', 'spell:mage-armor'],
    equipment: [
      { $typeName: 'meurpg.characters.v1.Item', name: 'Grimório', quantity: 1 },
      { $typeName: 'meurpg.characters.v1.Item', name: 'Tocha', quantity: 3 },
    ],
    // No UI collects this — the field this test exists to protect.
    coins: {
      $typeName: 'meurpg.characters.v1.Coins',
      copper: 5,
      silver: 12,
      electrum: 0,
      gold: 30,
      platinum: 1,
    },
    languages: ['Anão', 'Élfico'],
    toolProficiencies: ['Ferramentas de cartógrafo'],
    experiencePoints: 2700,
    alignment: Alignment.NEUTRAL_GOOD,
    customFeaturesText: 'Sabe um truque de cartas que sempre erra.',
    // No UI collects this either — the plan §4 gap this test also protects.
    featureChoiceKeys: ['feature:fighter-fighting-style-defense'],
  };
}

describe('FullSheet round-trips load → save unchanged (integrator fix, phase 2b)', () => {
  it('a fully populated FullSheet survives toFormFullSheet → mergeFullSheetInit, coins and feature_choice_keys included', () => {
    const loaded = fullyPopulatedFullSheet();

    const form = toFormFullSheet('Pensantus', loaded);
    const merged = mergeFullSheetInit(loaded, form);

    expect(merged).toEqual({
      baseScores: {
        strength: loaded.baseScores!.strength,
        dexterity: loaded.baseScores!.dexterity,
        constitution: loaded.baseScores!.constitution,
        intelligence: loaded.baseScores!.intelligence,
        wisdom: loaded.baseScores!.wisdom,
        charisma: loaded.baseScores!.charisma,
      },
      raceKey: loaded.raceKey,
      subraceKey: loaded.subraceKey,
      classes: [
        {
          classKey: loaded.classes[0].classKey,
          level: loaded.classes[0].level,
          subclass: loaded.classes[0].subclass,
        },
      ],
      background: loaded.background,
      skillProficiencyKeys: loaded.skillProficiencyKeys,
      expertiseSkillKeys: loaded.expertiseSkillKeys,
      extraAbilityBonuses: {
        strength: loaded.extraAbilityBonuses!.strength,
        dexterity: loaded.extraAbilityBonuses!.dexterity,
        constitution: loaded.extraAbilityBonuses!.constitution,
        intelligence: loaded.extraAbilityBonuses!.intelligence,
        wisdom: loaded.extraAbilityBonuses!.wisdom,
        charisma: loaded.extraAbilityBonuses!.charisma,
      },
      hitPoints: { method: loaded.hitPoints!.method, rolls: loaded.hitPoints!.rolls },
      armorKey: loaded.armorKey,
      shield: loaded.shield,
      weaponKeys: loaded.weaponKeys,
      cantripKeys: loaded.cantripKeys,
      knownSpellKeys: loaded.knownSpellKeys,
      preparedSpellKeys: loaded.preparedSpellKeys,
      equipment: [
        { name: 'Grimório', quantity: 1 },
        { name: 'Tocha', quantity: 3 },
      ],
      languages: loaded.languages,
      toolProficiencies: loaded.toolProficiencies,
      experiencePoints: loaded.experiencePoints,
      alignment: loaded.alignment,
      customFeaturesText: loaded.customFeaturesText,
      // The whole point: fields the form has no UI for come back exactly
      // as loaded, not wiped to a zero/empty default.
      coins: loaded.coins,
      featureChoiceKeys: loaded.featureChoiceKeys,
    });
  });

  it('a custom background with exactly two granted skills round-trips too', () => {
    const loaded: FullSheet = {
      ...fullyPopulatedFullSheet(),
      background: {
        case: 'customBackground',
        value: {
          $typeName: 'meurpg.characters.v1.CustomBackground',
          name: 'Sábio',
          skillKeys: ['skill:arcana', 'skill:history'],
        },
      },
    };

    const form = toFormFullSheet('Pensantus', loaded);
    expect(form.background).toBe('custom');
    expect(form.customBackgroundName).toBe('Sábio');
    expect(form.customBackgroundSkills).toEqual(['skill:arcana', 'skill:history']);

    const merged = mergeFullSheetInit(loaded, form);
    // `background` is always rebuilt from the form (there is no "not shown
    // by the form" part of it to preserve), so it comes back as a plain
    // init shape, not the branded message `loaded` carried.
    expect(merged.background).toEqual({
      case: 'customBackground',
      value: { name: 'Sábio', skillKeys: ['skill:arcana', 'skill:history'] },
    });
  });

  it('CreateCharacter (no loaded message) is exactly toFullSheetInit — nothing to merge yet', () => {
    const form = toFormFullSheet('Pensantus', fullyPopulatedFullSheet());
    expect(mergeFullSheetInit(undefined, form)).toEqual(toFullSheetInit(form));
  });

  it('sends no hit points rolls on the wire for the "average" method, even if some were typed', () => {
    const loaded = fullyPopulatedFullSheet(); // hitPointsMethod: 'rolled' by fixture
    const form = toFormFullSheet('Pensantus', loaded);

    const stillRolled = toFullSheetInit(form);
    expect(stillRolled.hitPoints).toEqual({ method: loaded.hitPoints!.method, rolls: [4, 6, 2] });

    const switchedToAverage = toFullSheetInit({ ...form, hitPointsMethod: 'average' });
    expect(switchedToAverage.hitPoints.rolls).toEqual([]);
  });
});
