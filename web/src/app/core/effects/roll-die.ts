import type { RollDie } from '../../../gen/meurpg/rules/v1/rules_pb';
import type { ExtraDieField } from './effects';

/** The roll a table die may join: an ability check (a skill or an ability), a saving throw of an ability, or the initiative. */
export type RollDieKind =
  | { readonly kind: 'check'; readonly skill?: string; readonly ability?: string }
  | { readonly kind: 'save'; readonly ability: string }
  | { readonly kind: 'initiative' };

/** The ability of each SRD skill, to tell a skill check apart from a die limited to the checks of an ability ("check.dex"). */
const SKILL_ABILITY: Readonly<Record<string, string>> = {
  athletics: 'str',
  acrobatics: 'dex',
  'sleight-of-hand': 'dex',
  stealth: 'dex',
  arcana: 'int',
  history: 'int',
  investigation: 'int',
  nature: 'int',
  religion: 'int',
  'animal-handling': 'wis',
  insight: 'wis',
  medicine: 'wis',
  perception: 'wis',
  survival: 'wis',
  deception: 'cha',
  intimidation: 'cha',
  performance: 'cha',
  persuasion: 'cha',
};

/** What a scene action rolls ("skill:stealth", "ability:str", "save:wis"), as the roll a die may join. */
export function sceneRollKind(actionKey: string): RollDieKind | null {
  const [kind, name = ''] = actionKey.split(':');
  switch (kind) {
    case 'skill':
      return { kind: 'check', skill: name, ability: SKILL_ABILITY[name] };
    case 'ability':
      return { kind: 'check', ability: name };
    case 'save':
      return { kind: 'save', ability: name };
    default:
      return null;
  }
}

function fits(die: RollDie, roll: RollDieKind): boolean {
  if (die.applies !== roll.kind) {
    return false;
  }
  if (roll.kind === 'check') {
    // No list: every ability check. A list names the skills ("skill.stealth") and abilities ("check.dex") it is limited to.
    return (
      die.targets.length === 0 ||
      die.targets.some(
        (t) =>
          (roll.skill !== undefined && t === `skill.${roll.skill}`) ||
          (roll.ability !== undefined && t === `check.${roll.ability}`),
      )
    );
  }
  return roll.kind === 'initiative' || die.ability === '' || die.ability === roll.ability;
}

/** The table dice the roll offers: only a die whose `applies` and limits fit it. */
export function rollDiceFor(
  dice: readonly RollDie[],
  roll: RollDieKind | null,
): readonly RollDie[] {
  return roll ? dice.filter((d) => fits(d, roll)) : [];
}

/** The field a physical roll asks for the die's face: always the last one of the roll's extra dice. */
export function rollDieField(die: RollDie): ExtraDieField {
  return {
    label: `Resultado do d${die.sides} (${die.namePt})`,
    name: die.namePt,
    faces: die.sides,
    sign: 1,
  };
}

/** "d8 · gasta 1 uso de dados de superioridade": what the tick says it costs. */
export function rollDieCost(die: RollDie): string {
  return die.resource === ''
    ? `d${die.sides}`
    : `d${die.sides} · gasta 1 uso de ${die.resource.replace(/_/g, ' ')}`;
}
