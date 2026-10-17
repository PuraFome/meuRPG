import type { SpellDraft } from './spell-types';

/** What the spell editor tells the master about damage the combat cannot apply. It only warns: the entry still saves. */
export const UNAPPLIED_DAMAGE_PT =
  'O app não sabe aplicar este dano: não há ataque, teste de resistência nem área. Escolha um ataque ou um teste de resistência em “Mecânica”, ou deixe o alvo como área.';

/**
 * Whether the spell has damage but nothing to apply it with: no attack to roll, no saving throw to ask for and no area to
 * pick creatures in. A spell on the caster alone is left out.
 */
export function unappliedDamage(d: SpellDraft): boolean {
  const hasDamage = d.dice.trim() !== '';
  const hasWay = d.mechanic === 'attack' || d.mechanic === 'save';
  const hasTargets = d.target === 'area' || d.target === 'self';
  return hasDamage && !hasWay && !hasTargets;
}
