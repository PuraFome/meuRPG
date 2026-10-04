import {
  LevelUpHitPointsMethod,
  type LevelUp,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { abilityLabel } from '../characters/character-labels';
import type { AbilityKey } from '../characters/characters.types';

/** One line of the master's "O que mudou": what the player chose, in words. */
export interface ChoiceRow {
  readonly label: string;
  readonly value: string;
}

const LIST = new Intl.ListFormat('pt-BR', { type: 'conjunction' });

/** The six abilities of an `AbilityScores`, in sheet order, with what was added to each. */
function increases(levelUp: LevelUp): { key: AbilityKey; by: number }[] {
  const a = levelUp.choices?.abilityIncrease;
  if (!a) {
    return [];
  }
  return (
    [
      ['str', a.strength],
      ['dex', a.dexterity],
      ['con', a.constitution],
      ['int', a.intelligence],
      ['wis', a.wisdom],
      ['cha', a.charisma],
    ] as const
  )
    .filter(([, by]) => by > 0)
    .map(([key, by]) => ({ key, by }));
}

/**
 * What the player chose in one level-up (MR-040), as the master reads it under "O que mudou":
 * the ability increase, how the hit points were decided, the subclass, the new cantrips,
 * spells, features, skills and expertise, by their Portuguese names (`names_pt`). A line with
 * nothing in it is left out. Only what the server stored: no number is worked out here.
 */
export function choiceRows(levelUp: LevelUp): ChoiceRow[] {
  const c = levelUp.choices;
  if (!c) {
    return [];
  }
  const names = (keys: readonly string[]) => LIST.format(keys.map((k) => levelUp.namesPt[k] ?? k));
  const rows: ChoiceRow[] = [];
  const up = increases(levelUp);
  if (up.length > 0) {
    rows.push({
      label: 'Atributos',
      value:
        up.length === 1
          ? `+${up[0].by} em ${abilityLabel(up[0].key)}`
          : LIST.format(up.map((u) => `+${u.by} em ${abilityLabel(u.key)}`)),
    });
  }
  const hp = c.hitPoints;
  if (hp && hp.value > 0) {
    const how =
      hp.method === LevelUpHitPointsMethod.ROLLED_IN_APP
        ? 'Rolado no app'
        : hp.method === LevelUpHitPointsMethod.ROLLED_PHYSICAL
          ? 'Dado físico'
          : 'Média';
    rows.push({ label: 'Pontos de vida', value: `${how}: ${hp.value}, mais o modificador de Constituição` });
  }
  if (c.subclassKey) {
    rows.push({ label: 'Subclasse', value: names([c.subclassKey]) });
  }
  const lists: [string, readonly string[]][] = [
    [c.cantripKeys.length === 1 ? 'Truque novo' : 'Truques novos', c.cantripKeys],
    [c.knownSpellKeys.length === 1 ? 'Magia nova' : 'Magias novas', c.knownSpellKeys],
    ['Magias preparadas novas', c.preparedSpellKeys],
    ['Opções de características', c.featureChoiceKeys],
    [c.skillProficiencyKeys.length === 1 ? 'Perícia nova' : 'Perícias novas', c.skillProficiencyKeys],
    ['Especialização', c.expertiseSkillKeys],
  ];
  for (const [label, keys] of lists) {
    if (keys.length > 0) {
      rows.push({ label, value: names(keys) });
    }
  }
  return rows;
}
