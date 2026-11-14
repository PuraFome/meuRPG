import type { ItemModifierLineVm, ItemModifierVm } from './character-sheet.types';

function targetWords(targets: readonly string[]): string {
  if (targets.includes('ac')) {
    return 'na CA';
  }
  const attack = targets.some((t) => t.startsWith('attack:'));
  const damage = targets.some((t) => t.startsWith('damage:'));
  if (attack && damage) {
    return 'no ataque e no dano';
  }
  return attack ? 'no ataque' : 'no dano';
}

/** One line per piece, in the order of the server's entries: the bonus, the name and where it is added. */
export function itemModifierLines(modifiers: readonly ItemModifierVm[]): ItemModifierLineVm[] {
  const keys = [...new Set(modifiers.map((m) => m.sourceKey))];
  return keys.map((key) => {
    const own = modifiers.filter((m) => m.sourceKey === key);
    return {
      sourceKey: key,
      value: own[0].value,
      namePt: own[0].sourceNamePt,
      targetPt: targetWords(own.map((m) => m.target)),
      counts: own.every((m) => m.counts),
      reasonPt: own.find((m) => !m.counts)?.reasonPt ?? '',
    };
  });
}
