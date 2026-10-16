import { FormBuilder, ValidatorFn, Validators } from '@angular/forms';

import { MAX_TOTAL_LEVEL } from './class-blocks';
import type {
  AlignmentKey,
  CharacterFormValue,
  CuttingWordsAskKey,
  HitPointsMethod,
} from './character-editor.types';

// The server's limits on the text and the numbers of a sheet (`characters.proto`): the form refuses a value over them
// before the request, so the save never comes back with an `invalid_argument` the person could have seen.
const NAME_MAX = 80;
const BACKGROUND_NAME_MAX = 40;
const FEATURE_TEXT_MAX = 1000;
const BACKGROUND_EQUIPMENT_MAX = 500;
const EQUIPMENT_TEXT_MAX = 4000;
const PROFICIENCIES_TEXT_MAX = 2000;
const CUSTOM_FEATURES_TEXT_MAX = 5000;
const EXPERIENCE_MAX = 1000000;
const ABILITY_SCORE_MAX = 30;
const MANUAL_BONUS_LIMIT = 10;

/** One typed ability score. */
const score = (): [number, ValidatorFn[]] => [
  10,
  [Validators.required, Validators.min(1), Validators.max(ABILITY_SCORE_MAX)],
];

/** One manual bonus on an ability score. */
const bonus = (): [number, ValidatorFn[]] => [
  0,
  [Validators.required, Validators.min(-MANUAL_BONUS_LIMIT), Validators.max(MANUAL_BONUS_LIMIT)],
];

/** The form of a full sheet (player, enemy, boss): the controls the stepper's steps write. What is not a text, a
 * number or a choice of one (the picks, the class blocks, the rolls) lives in signals next to it. */
export function createFullForm(fb: FormBuilder) {
  return fb.nonNullable.group({
    name: ['', [Validators.required, Validators.maxLength(NAME_MAX)]],
    race: ['', Validators.required],
    subrace: [''],
    className: ['', Validators.required],
    subclassName: [''],
    customSubclassName: [''],
    level: [1, [Validators.required, Validators.min(1), Validators.max(MAX_TOTAL_LEVEL)]],
    background: ['', Validators.required],
    customBackgroundName: ['', Validators.maxLength(BACKGROUND_NAME_MAX)],
    // The "Outro" background's feature (a name and a text) and its equipment, in the player's words.
    customBackgroundFeatureName: ['', Validators.maxLength(BACKGROUND_NAME_MAX)],
    customBackgroundFeatureText: ['', Validators.maxLength(FEATURE_TEXT_MAX)],
    customBackgroundEquipment: ['', Validators.maxLength(BACKGROUND_EQUIPMENT_MAX)],
    /** A content key from the catalog, or `''` for "Sem armadura" — never
     * typed (integrator fix). */
    armor: [''],
    shield: [false],
    /** Content keys, `<mat-select multiple>` — never typed. */
    weaponKeys: [[] as string[]],
    equipmentText: ['', Validators.maxLength(EQUIPMENT_TEXT_MAX)],
    languagesText: ['', Validators.maxLength(PROFICIENCIES_TEXT_MAX)],
    toolProficienciesText: ['', Validators.maxLength(PROFICIENCIES_TEXT_MAX)],
    experiencePoints: [0, [Validators.required, Validators.min(0), Validators.max(EXPERIENCE_MAX)]],
    // An enemy's or boss's ND and the XP it gives when defeated (E7-11); a
    // player's stay empty and 0 and are never shown.
    challengeRating: [''],
    xpValue: [0, [Validators.required, Validators.min(0), Validators.max(EXPERIENCE_MAX)]],
    // An enemy's or boss's portrait (MR-031): a gallery image's ID, or empty.
    portraitImageId: [''],
    alignment: ['' as AlignmentKey],
    customFeaturesText: ['', Validators.maxLength(CUSTOM_FEATURES_TEXT_MAX)],
    cuttingWordsAsk: ['only-attacks' as CuttingWordsAskKey],
    hitPointsMethod: ['average' as HitPointsMethod],
    abilities: fb.nonNullable.group({
      str: score(),
      dex: score(),
      con: score(),
      int: score(),
      wis: score(),
      cha: score(),
    }),
    extraAbilityBonuses: fb.nonNullable.group({
      str: bonus(),
      dex: bonus(),
      con: bonus(),
      int: bonus(),
      wis: bonus(),
      cha: bonus(),
    }),
  });
}

export type FullForm = ReturnType<typeof createFullForm>;
export type FullFormRaw = ReturnType<FullForm['getRawValue']>;

/** The form's value for a saved sheet (an edit). */
export function fullFormValueOf(full: CharacterFormValue): FullFormRaw {
  return {
    name: full.name,
    race: full.race,
    subrace: full.subrace,
    className: full.className,
    subclassName: full.subclassName,
    customSubclassName: full.customSubclassName,
    level: full.level,
    background: full.background,
    customBackgroundName: full.customBackgroundName,
    customBackgroundFeatureName: full.customBackgroundFeatureName,
    customBackgroundFeatureText: full.customBackgroundFeatureText,
    customBackgroundEquipment: full.customBackgroundEquipment,
    armor: full.armor,
    shield: full.shield,
    weaponKeys: full.weapons,
    equipmentText: full.equipmentText,
    languagesText: full.languagesText,
    toolProficienciesText: full.toolProficienciesText,
    experiencePoints: full.experiencePoints,
    challengeRating: full.challengeRating,
    xpValue: full.xpValue,
    portraitImageId: full.portraitImageId,
    alignment: full.alignment,
    customFeaturesText: full.customFeaturesText,
    cuttingWordsAsk: full.cuttingWordsAsk,
    hitPointsMethod: full.hitPointsMethod,
    abilities: full.abilities,
    extraAbilityBonuses: full.extraAbilityBonuses,
  };
}
