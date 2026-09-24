import type { AttributeKey, SkillEntry } from '../../core/models/character';

/** Ability modifier for a score, per the 5e formula floor((score - 10) / 2). */
export function abilityModifier(score: number): number {
  const safe = Number.isFinite(score) ? score : 10;
  return Math.floor((safe - 10) / 2);
}

/** Proficiency bonus by character level (2 at levels 1-4 up to 6 at 17-20). */
export function proficiencyBonusForLevel(level: number): number {
  const safe = Math.max(1, Math.min(20, Math.trunc(level) || 1));
  return 2 + Math.floor((safe - 1) / 4);
}

/** Total bonus for a proficiency-enabled check: ability mod + prof (+ prof again on expertise). */
export function proficiencyContribution(
  proficient: boolean,
  expertise: boolean,
  proficiencyBonus: number,
): number {
  if (expertise) return proficiencyBonus * 2;
  if (proficient) return proficiencyBonus;
  return 0;
}

/** Computed skill bonus (before any manual override). */
export function computedSkillBonus(
  entry: Pick<SkillEntry, 'ability' | 'proficient' | 'expertise'>,
  attributes: Record<string, number>,
  proficiencyBonus: number,
): number {
  const score = attributes[entry.ability] ?? 10;
  return (
    abilityModifier(score) +
    proficiencyContribution(entry.proficient, entry.expertise, proficiencyBonus)
  );
}

/** Skill bonus actually used: manual override when present, otherwise computed. */
export function effectiveSkillBonus(
  entry: SkillEntry,
  attributes: Record<string, number>,
  proficiencyBonus: number,
): number {
  if (entry.bonusOverride !== null && entry.bonusOverride !== undefined) {
    return entry.bonusOverride;
  }
  return computedSkillBonus(entry, attributes, proficiencyBonus);
}

/** Saving throw bonus for one ability. */
export function savingThrowBonus(
  ability: AttributeKey,
  proficient: boolean,
  attributes: Record<string, number>,
  proficiencyBonus: number,
): number {
  const score = attributes[ability] ?? 10;
  return abilityModifier(score) + (proficient ? proficiencyBonus : 0);
}

/**
 * Passive Perception = 10 + Perception bonus (ability mod + prof + expertise).
 * Accepts a partial Perception entry; defaults to a non-proficient check.
 */
export function passivePerception(
  attributes: Record<string, number>,
  proficiencyBonus: number,
  perception: Pick<SkillEntry, 'ability' | 'proficient' | 'expertise'> | null,
): number {
  const entry = perception ?? { ability: 'sab' as AttributeKey, proficient: false, expertise: false };
  return 10 + computedSkillBonus(entry, attributes, proficiencyBonus);
}

/** Spell save DC = 8 + proficiency bonus + spellcasting ability modifier. */
export function spellSaveDc(abilityMod: number, proficiencyBonus: number): number {
  return 8 + proficiencyBonus + abilityMod;
}

/** Spell attack bonus = proficiency bonus + spellcasting ability modifier. */
export function spellAttackBonus(abilityMod: number, proficiencyBonus: number): number {
  return proficiencyBonus + abilityMod;
}

/** Formats a modifier with an explicit sign: 2 -> "+2", -1 -> "-1", 0 -> "+0". */
export function formatModifier(value: number): string {
  return value >= 0 ? `+${value}` : `${value}`;
}