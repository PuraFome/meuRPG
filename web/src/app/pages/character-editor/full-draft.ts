import type { CharacterFormValue, ExtraClassValue } from './character-editor.types';
import type { FullForm, FullFormRaw } from './full-form';
import { MAX_TOTAL_LEVEL } from './class-blocks';
import { validRoll } from './hit-points-preview';
import {
  type EditorField,
  FULL_SHEET_FIELDS,
  UNPLACED_RESULTS_FIELD,
  invalidFields,
} from './editor-labels';

/** Everything the draft of a full sheet is made of besides the form's own controls. */
export interface DraftParts {
  readonly form: FullFormRaw;
  readonly customBackgroundSkills: ReadonlySet<string>;
  readonly customBackgroundProficiencies: readonly string[];
  readonly extraClasses: readonly ExtraClassValue[];
  readonly chosenSkills: ReadonlySet<string>;
  readonly expertiseSkills: ReadonlySet<string>;
  readonly hitPointsRolls: readonly number[];
  /** How many rolls the levels need: the ones past that are dropped, not sent stale. */
  readonly rollsNeeded: number;
  readonly isCaster: boolean;
  readonly cantrips: string[];
  readonly spellsKnown: string[];
  readonly spellsPrepared: string[];
  readonly featureChoiceKeys: readonly string[];
  readonly featureChoiceText: Readonly<Record<string, string>>;
  /** The master's editor sends the feats; a player's save keeps the stored ones (the server refuses a change), so a
   * player's draft has none here. */
  readonly feats: {
    readonly keys: readonly string[];
    readonly slots: Readonly<Record<string, string>>;
  } | null;
}

/** The "Outro" background's own fields: only a custom background sends them. The skills: "fewer than 2 shows an
 * issue" — the server reports that; the editor just sends what was chosen, even 0 or 1, rather than guessing or
 * blocking submission over it. The tools or languages, the feature and the equipment the same way. */
function customBackgroundOf(d: DraftParts) {
  const v = d.form;
  const custom = v.background === 'custom';
  const skills = Array.from(d.customBackgroundSkills);
  const pair: [string, string] | null =
    custom && skills.length === 2 ? [skills[0], skills[1]] : null;
  return {
    customBackgroundSkills: pair,
    customBackgroundProficiencies: custom ? [...d.customBackgroundProficiencies] : [],
    customBackgroundFeatureName: custom ? v.customBackgroundFeatureName : '',
    customBackgroundFeatureText: custom ? v.customBackgroundFeatureText : '',
    customBackgroundEquipment: custom ? v.customBackgroundEquipment : '',
  };
}

/** The full sheet the draft stands for, as the request carries it. */
export function buildFullValue(d: DraftParts): CharacterFormValue {
  const v = d.form;
  return {
    name: v.name,
    race: v.race,
    subrace: v.subrace,
    className: v.className,
    subclassName: v.subclassName,
    customSubclassName: v.customSubclassName,
    level: v.level,
    background: v.background,
    customBackgroundName: v.customBackgroundName,
    ...customBackgroundOf(d),
    // A blank block never gets here: `classProblems` stops the save first.
    extraClasses: d.extraClasses.filter((c) => c.classKey !== ''),
    skillProficiencies: Array.from(d.chosenSkills),
    expertiseSkillKeys: Array.from(d.expertiseSkills),
    abilities: v.abilities,
    extraAbilityBonuses: v.extraAbilityBonuses,
    hitPointsMethod: v.hitPointsMethod,
    // Only as many rolls as the current level needs — a roll left over
    // from a higher level typed earlier is dropped, not sent stale.
    hitPointsRolls: d.hitPointsRolls.slice(0, d.rollsNeeded),
    isCaster: d.isCaster,
    // Never send a stale pick: if the class or level changed after a
    // spell was chosen and it dropped off the (class-filtered) catalog
    // list, it never reaches the server — no typed key ever could get
    // here in the first place (integrator fix).
    cantrips: d.cantrips,
    spellsKnown: d.spellsKnown,
    spellsPrepared: d.spellsPrepared,
    armor: v.armor,
    shield: v.shield,
    weapons: v.weaponKeys,
    equipmentText: v.equipmentText,
    languagesText: v.languagesText,
    toolProficienciesText: v.toolProficienciesText,
    experiencePoints: v.experiencePoints,
    challengeRating: v.challengeRating,
    xpValue: v.xpValue,
    portraitImageId: v.portraitImageId,
    alignment: v.alignment,
    customFeaturesText: v.customFeaturesText,
    featureChoiceKeys: [...d.featureChoiceKeys],
    featureChoiceText: { ...d.featureChoiceText },
    cuttingWordsAsk: v.cuttingWordsAsk,
    ...(d.feats ? { featKeys: [...d.feats.keys], featSlots: { ...d.feats.slots } } : {}),
  };
}

/** What the class blocks still lack, in words ("Classe 2", "o nível total vai até 20"): not controls of the form, so
 * the page adds them to the invalid fields while they stand. */
export function classProblemsOf(
  extraClasses: readonly ExtraClassValue[],
  totalLevelProblem: string,
): string[] {
  const problems: string[] = [];
  extraClasses.forEach((c, i) => {
    if (c.classKey === '') problems.push(`Classe ${i + 2}`);
    if (!Number.isInteger(c.level) || c.level < 1 || c.level > MAX_TOTAL_LEVEL)
      problems.push(`Nível da classe ${i + 2}`);
  });
  if (totalLevelProblem) problems.push('o nível total vai até 20');
  return problems;
}

/** With "Rolado", the levels whose roll is empty or does not fit that level's die: the form keeps no control for
 * them, so the page adds them to the invalid fields while they stand. */
export function rollProblemsOf(
  rolled: boolean,
  hitDie: number,
  needed: number,
  dice: readonly number[],
  rolls: readonly number[],
): string[] {
  if (!rolled || hitDie === 0) {
    return [];
  }
  const problems: string[] = [];
  for (let i = 0; i < needed; i++) {
    if (validRoll(rolls[i], dice[i] || hitDie) === null) {
      problems.push(`Dado de vida do nível ${i + 2}`);
    }
  }
  return problems;
}

/** The full sheet's invalid fields right now, plus the placing that is not finished (it has no control of its own:
 * see `abilitiesIncomplete`). */
export function invalidFullFieldsOf(
  form: FullForm,
  classProblems: readonly string[],
  rollProblems: readonly string[],
  placing: { readonly incomplete: boolean; readonly problem: string },
): EditorField[] {
  const fields: EditorField[] = [
    ...invalidFields(form, FULL_SHEET_FIELDS),
    ...classProblems.map((label) => ({ path: 'classes', label, step: 'basico' as const })),
    ...rollProblems.map((label) => ({
      path: 'hitPointsRolls',
      label,
      step: 'atributos' as const,
    })),
  ];
  if (!placing.incomplete) {
    return fields;
  }
  return [
    ...fields,
    placing.problem
      ? { ...UNPLACED_RESULTS_FIELD, label: placing.problem }
      : UNPLACED_RESULTS_FIELD,
  ];
}

/** Whether the server is asked to derive the draft: while the "Pontos de vida" box (rolled, with a die to roll), the
 * "Magias" step or the master's feats are on screen. */
export function serverDraftNeeded(on: {
  readonly ready: boolean;
  readonly rolledBox: boolean;
  readonly caster: boolean;
  readonly feats: boolean;
}): boolean {
  return on.ready && (on.rolledBox || on.caster || on.feats);
}

/** What the draft stores of the choices after the server answered: the picks no choice offers any more go, and so do
 * the texts of a choice that is not offered; `null` where nothing changes (so no signal is set). */
export function choicesAfterAnswer(
  answer: {
    readonly notOffered: readonly string[];
    readonly groups: readonly { readonly choices: readonly { readonly key: string }[] }[];
  },
  keys: readonly string[],
  texts: Readonly<Record<string, string>>,
): { keys: string[] | null; texts: Record<string, string> | null } {
  const gone = new Set(answer.notOffered);
  const live = new Set(answer.groups.flatMap((g) => g.choices.map((c) => c.key)));
  const kept = Object.entries(texts).filter(([k]) => live.has(k.slice(0, k.lastIndexOf('#'))));
  return {
    keys: gone.size > 0 ? keys.filter((k) => !gone.has(k)) : null,
    texts: kept.length !== Object.keys(texts).length ? Object.fromEntries(kept) : null,
  };
}
