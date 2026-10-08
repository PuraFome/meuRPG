import { SpellRangeKind, type SpellDetails } from '../../../gen/meurpg/rules/v1/rules_pb';
import { joinDots, tight } from '../format/text';
import { damageDice } from './cast-flow';
import { poolDice } from './hp-effects';
import { metersText } from '../units';

/**
 * The line under a spell's name in "O que você pode fazer" (E8-02): where it
 * reaches and what it rolls, from the same details the cast sheet's subtitle reads:
 * "Alcance 27 m · 5d8 PV de criaturas", "Alcance 36 m · 8d6 de fogo". Empty until
 * the details are read; a cantrip's dice are the caster's (`Attack.spellDice`). Nothing is written by hand per spell.
 */
export function spellSummary(details: SpellDetails | null | undefined, cantripDice = ''): string {
  if (!details) {
    return '';
  }
  const parts: string[] = [];
  const range = details.range;
  if (range?.kind === SpellRangeKind.RANGED && range.distanceFt > 0) {
    parts.push(`Alcance ${metersText(range.distanceFt)}`);
  } else if (range?.kind === SpellRangeKind.TOUCH) {
    parts.push('Toque');
  } else if (range?.kind === SpellRangeKind.SELF) {
    parts.push('Pessoal');
  }
  const level = details.spell?.level ?? 0;
  const pool = poolDice(details, level);
  const dice = damageDice(details, level, cantripDice);
  const type = details.damage[0]?.damageTypePt ?? '';
  const heal = details.healBySlotLevel[level];
  if (pool) {
    parts.push(`${pool.count}d${pool.sides} PV de criaturas`);
  } else if (dice) {
    parts.push(type ? `${dice} de ${type}` : dice);
  } else if (heal) {
    parts.push(`cura ${heal.replace('MOD', 'mod.')}`);
  }
  return tight(joinDots(parts));
}
