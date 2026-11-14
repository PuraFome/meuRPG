import type { EffectDraft } from './effect-draft.types';

/** The required closed-list fields of the action, maneuver, resistance, spell and condition types, by name: whether each is still empty. */
export const CHOICE_EMPTY: Readonly<Record<string, (d: EffectDraft) => boolean>> = {
  event: (d) => d.event === '',
  economy: (d) => d.economy === '',
  applies: (d) => d.applies === '',
  except: (d) => d.except === '',
  while: (d) => d.while === '',
  spell: (d) => d.spell === '',
  ability: (d) => d.ability === '',
  conditions: (d) => d.conditions.length === 0,
  applies_to: (d) => d.appliesTo === '',
  spell_change: (d) => d.spellChange === '',
  rolls: (d) => d.rolls.length === 0,
  at_most: (d) => d.atMost <= 0,
  aura: (d) => !d.aura,
};
