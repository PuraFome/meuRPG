import type { GearVm, MarksVm } from '../characters/gear';
import { attackMarksOf, magicBonusOf } from './combat-options';

type AttackRow = Parameters<typeof magicBonusOf>[0];

/** The bonus chip a fold row draws: solid when it counts, dashed when it does not (the shield behind a two-handed weapon). */
function bonusDrawnOn(g: GearVm, shieldIdle: boolean): '' | 'counts' | 'idle' {
  if (!g.bonus) {
    return '';
  }
  return g.bonusCounts !== false && !(g.kind === 'shield' && g.equipped && shieldIdle)
    ? 'counts'
    : 'idle';
}

/**
 * What the chips on the combat's "Ataques" tab draw, for the legend under them: the marks of each attack row (a counting +N
 * takes the place of "Mágica") and of each piece in the fold (the armor is not listed), and whether a +N counts now. A
 * shield behind a two-handed weapon draws its bonus dashed.
 */
export function legendOfRows(
  rows: readonly (AttackRow & { readonly attack?: Parameters<typeof attackMarksOf>[0] })[],
  gear: readonly GearVm[],
  shieldIdle: boolean,
): { marks: (MarksVm | null)[]; bonus: '' | 'counts' | 'idle' } {
  const marks: (MarksVm | null)[] = [];
  let counts = false;
  let idle = false;
  for (const a of rows) {
    const bonus = magicBonusOf(a);
    marks.push(attackMarksOf(a.attack, bonus));
    counts ||= bonus > 0;
  }
  for (const g of gear.filter((p) => p.kind !== 'armor')) {
    const drawn = bonusDrawnOn(g, shieldIdle);
    marks.push(g.marks && drawn === 'counts' ? { ...g.marks, magical: false } : (g.marks ?? null));
    counts ||= drawn === 'counts';
    idle ||= drawn === 'idle';
  }
  return { marks, bonus: counts ? 'counts' : idle ? 'idle' : '' };
}
