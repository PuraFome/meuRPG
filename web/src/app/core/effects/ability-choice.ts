/** The spell whose cast names an ability (the server rejects it without one). */
export const ENHANCE_ABILITY_KEY = 'spell:enhance-ability';

/** One ability Enhance Ability can be cast for: its key, its name and the animal that goes with it. */
export interface AbilityChoice {
  readonly key: string;
  readonly name: string;
  readonly hint: string;
}

export const ABILITY_CHOICES: readonly AbilityChoice[] = [
  { key: 'str', name: 'Força', hint: 'Touro' },
  { key: 'dex', name: 'Destreza', hint: 'Gato' },
  { key: 'con', name: 'Constituição', hint: 'Urso' },
  { key: 'int', name: 'Inteligência', hint: 'Raposa' },
  { key: 'wis', name: 'Sabedoria', hint: 'Coruja' },
  { key: 'cha', name: 'Carisma', hint: 'Águia' },
];

/**
 * The Portuguese name of an ability: the server's when it sent one, else the name of its key ("con"), else the key itself.
 * The server's zone fields (`save_ability`, `EffectSavePrompt.ability`) carry the key and may leave the name empty, so the
 * client names it from the one table of the six abilities.
 */
export function abilityName(key: string, namePt: string): string {
  return namePt || ABILITY_CHOICES.find((a) => a.key === key)?.name || key;
}

/** Whether casting this spell asks which ability it is for. */
export function needsAbility(spellKey: string): boolean {
  return spellKey === ENHANCE_ABILITY_KEY;
}

/** Why the cast cannot go yet, when the spell asks for an ability and none is chosen; `''` otherwise. */
export function abilityMissing(spellKey: string, abilityKey: string): string {
  return needsAbility(spellKey) && !ABILITY_CHOICES.some((a) => a.key === abilityKey)
    ? 'Escolha a habilidade.'
    : '';
}
