import {
  Ability,
  type AbilityScores,
  type FeatPrerequisite,
  type FeatUnmet,
  FeatUnmetKind,
} from '../../../gen/meurpg/rules/v1/rules_pb';
import { ABILITY_LABELS } from '../characters/character-labels';
import type { AbilityKey } from '../characters/characters.types';

/**
 * What a feat asks of a character and what a character lacks for it, in Portuguese (MR-025). The server decides who
 * qualifies and lists what is unmet; this only writes the prerequisite and the codes as sentences. A key the app has
 * no name for (a proficiency, a race) is named by `nameOf`, from the content catalog.
 */

export type KeyName = (key: string) => string;

const FIELDS: readonly { field: keyof AbilityScores & string; key: AbilityKey }[] = [
  { field: 'strength', key: 'str' },
  { field: 'dexterity', key: 'dex' },
  { field: 'constitution', key: 'con' },
  { field: 'intelligence', key: 'int' },
  { field: 'wisdom', key: 'wis' },
  { field: 'charisma', key: 'cha' },
];

const WIRE_KEY: Readonly<Record<number, AbilityKey>> = {
  [Ability.STRENGTH]: 'str',
  [Ability.DEXTERITY]: 'dex',
  [Ability.CONSTITUTION]: 'con',
  [Ability.INTELLIGENCE]: 'int',
  [Ability.WISDOM]: 'wis',
  [Ability.CHARISMA]: 'cha',
};

/** "Força", for an ability of the wire. */
export function abilityName(ability: Ability): string {
  const key = WIRE_KEY[ability];
  return key ? ABILITY_LABELS[key] : '';
}

/** "Força 13" for each ability with a score in `scores`, in the sheet's order. */
function scoreList(scores: AbilityScores | undefined): string[] {
  return FIELDS.flatMap(({ field, key }) => {
    const value = scores?.[field] as number | undefined;
    return value ? [`${ABILITY_LABELS[key]} ${value}`] : [];
  });
}

function listWith(items: readonly string[], word: 'e' | 'ou'): string {
  return items.length <= 1
    ? (items[0] ?? '')
    : `${items.slice(0, -1).join(', ')} ${word} ${items[items.length - 1]}`;
}

/** The conditions of a prerequisite, one phrase each ("Força 13 e Destreza 13", "proficiência em Armadura média"); none when it asks nothing. */
export function prerequisiteParts(p: FeatPrerequisite | undefined, nameOf: KeyName): string[] {
  if (!p) {
    return [];
  }
  const parts: string[] = [];
  const all = scoreList(p.minimums);
  if (all.length > 0) {
    parts.push(listWith(all, 'e'));
  }
  const any = scoreList(p.anyOf);
  if (any.length > 0) {
    parts.push(any.length === 1 ? any[0] : `${listWith(any, 'ou')} (uma delas)`);
  }
  if (p.proficiencyKey) {
    parts.push(`proficiência em ${nameOf(p.proficiencyKey)}`);
  }
  if (p.spellcasting) {
    parts.push('poder conjurar ao menos uma magia');
  }
  if (p.raceKey) {
    parts.push(`ser ${nameOf(p.raceKey)}`);
  }
  if (p.level > 0) {
    parts.push(`nível ${p.level} ou mais`);
  }
  return parts;
}

/** "Força 13, nível 4 ou mais"; empty when the feat asks nothing. */
export function prerequisiteSupport(p: FeatPrerequisite | undefined, nameOf: KeyName): string {
  return prerequisiteParts(p, nameOf).join(', ');
}

/** What a character lacks for a feat: "Precisa de Força 13.", from the server's `unmet`. */
export function unmetText(u: FeatUnmet, nameOf: KeyName): string {
  const needs = u.abilities.map((a) => `${abilityName(a.ability)} ${a.minimum}`);
  switch (u.kind) {
    case FeatUnmetKind.ABILITY_MINIMUM:
      return `Precisa de ${listWith(needs, 'e')}.`;
    case FeatUnmetKind.ABILITY_ANY_OF:
      return `Precisa de ${listWith(needs, 'ou')} (basta uma).`;
    case FeatUnmetKind.PROFICIENCY:
      return `Precisa de proficiência em ${nameOf(u.key)}.`;
    case FeatUnmetKind.SPELLCASTING:
      return 'Precisa poder conjurar ao menos uma magia.';
    case FeatUnmetKind.RACE:
      return `Precisa ser ${nameOf(u.key)}.`;
    case FeatUnmetKind.LEVEL:
      return `Precisa do nível ${u.value} ou mais.`;
    default:
      return 'Não atende ao pré-requisito.';
  }
}
