import type { FullSheetVm, ItemModifierLineVm, ItemModifierVm } from './character-sheet.types';

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

/** The source of the server's hint that says the idle shield adds nothing (it is also the key of the shield's own modifier). */
export const SHIELD_SOURCE = 'equipment:shield';

type ShieldReasonSheet = Pick<FullSheetVm, 'shieldIdle' | 'gear' | 'itemModifiers'>;

/** The combat column's note says why the shield is idle when the shield has no bonus line of its own to say it. */
export function shieldNoteShown(sheet: ShieldReasonSheet): boolean {
  return sheet.shieldIdle && !sheet.gear.some((g) => g.kind === 'shield' && g.bonus);
}

/** The combat column says why the shield is idle, in its note or in the shield's line: "Lembretes" then leaves the server's hint out. */
export function shieldReasonShown(sheet: ShieldReasonSheet): boolean {
  return (
    shieldNoteShown(sheet) ||
    (sheet.shieldIdle &&
      itemModifierLines(sheet.itemModifiers).some(
        (l) => l.sourceKey === SHIELD_SOURCE && !l.counts && l.reasonPt !== '',
      ))
  );
}
