import type { Combatant } from '../../../gen/meurpg/play/v1/combat_pb';

/**
 * The 15 conditions of the SRD that the master marks on a combatant (RN-22,
 * E6-29): labels only, the app applies no effect. The keys are the server's
 * (`condition:poisoned`) and the names the same Portuguese as the effects
 * content (`names_pt.json`), in alphabetical order, which is the order the
 * dialog lists them. "Derrubado" is the prone condition; "Caído" is the
 * 0-hit-point state of a player's character (question 43).
 */
export interface ConditionInfo {
  readonly key: string;
  readonly name: string;
}

export const CONDITIONS: readonly ConditionInfo[] = [
  { key: 'condition:grappled', name: 'Agarrado' },
  { key: 'condition:frightened', name: 'Amedrontado' },
  { key: 'condition:stunned', name: 'Atordoado' },
  { key: 'condition:blinded', name: 'Cego' },
  { key: 'condition:prone', name: 'Derrubado' },
  { key: 'condition:charmed', name: 'Enfeitiçado' },
  { key: 'condition:poisoned', name: 'Envenenado' },
  { key: 'condition:exhaustion', name: 'Exaustão' },
  { key: 'condition:restrained', name: 'Impedido' },
  { key: 'condition:incapacitated', name: 'Incapacitado' },
  { key: 'condition:unconscious', name: 'Inconsciente' },
  { key: 'condition:invisible', name: 'Invisível' },
  { key: 'condition:paralyzed', name: 'Paralisado' },
  { key: 'condition:petrified', name: 'Petrificado' },
  { key: 'condition:deafened', name: 'Surdo' },
];

/** A condition's Portuguese name by key; empty for a key this app doesn't know. */
export function conditionName(key: string): string {
  return CONDITIONS.find((c) => c.key === key)?.name ?? '';
}

/** The names written on a combatant's tags, in the order the master marked
 * them. The server sends them already (`condition_names_pt`); the keys are
 * the fallback for an older answer. */
export function conditionTags(c: Pick<Combatant, 'conditions' | 'conditionNamesPt'>): string[] {
  return c.conditionNamesPt.length > 0
    ? [...c.conditionNamesPt]
    : c.conditions.map(conditionName).filter(Boolean);
}

/** The keys now in the dialog's boxes against the ones the combatant has:
 * whether "Salvar condições" has anything to save. */
export function sameKeys(a: readonly string[], b: readonly string[]): boolean {
  return a.length === b.length && [...a].sort().join('|') === [...b].sort().join('|');
}

/** "Envenenado e Derrubado", "Cego, Surdo e Impedido": names read aloud. */
export function listNames(names: readonly string[]): string {
  if (names.length <= 1) {
    return names.join('');
  }
  return `${names.slice(0, -1).join(', ')} e ${names[names.length - 1]}`;
}
