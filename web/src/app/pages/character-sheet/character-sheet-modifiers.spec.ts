import { attackMarksFor } from '../../core/characters/gear';
import type { ItemModifierVm } from './character-sheet.types';
import { countingAttackBonus, itemModifierLines } from './character-sheet-modifiers';

function mod(
  target: string,
  sourceKey: string,
  over: Partial<ItemModifierVm> = {},
): ItemModifierVm {
  return {
    target,
    value: 1,
    sourceKey,
    sourceNamePt: sourceKey,
    counts: true,
    reasonPt: '',
    ...over,
  };
}

describe('itemModifierLines', () => {
  it("makes one line of a weapon's two entries, in the order of the entries", () => {
    const lines = itemModifierLines([
      mod('ac', 'armor'),
      mod('attack:sword', 'sword', { value: 2, sourceNamePt: 'Espada longa' }),
      mod('damage:sword', 'sword', { value: 2, sourceNamePt: 'Espada longa' }),
    ]);
    expect(lines.map((l) => [l.sourceKey, l.value, l.namePt, l.targetPt])).toEqual([
      ['armor', 1, 'armor', 'na CA'],
      ['sword', 2, 'Espada longa', 'no ataque e no dano'],
    ]);
  });

  it('keeps the reason of a piece that does not count', () => {
    const [line] = itemModifierLines([mod('ac', 'shield', { counts: false, reasonPt: 'Parado.' })]);
    expect(line).toMatchObject({ counts: false, reasonPt: 'Parado.' });
  });

  it('is empty without entries', () => {
    expect(itemModifierLines([])).toEqual([]);
  });
});

describe('countingAttackBonus and attackMarksFor', () => {
  const marks = { silvered: true, adamantine: false, magical: true };

  it('reads the bonus of a counting attack entry only', () => {
    expect(countingAttackBonus([mod('attack:bow', 'bow', { value: 3 })], 'bow')).toBe(3);
    expect(countingAttackBonus([mod('attack:bow', 'bow', { counts: false })], 'bow')).toBe(0);
    expect(countingAttackBonus([mod('attack:bow', 'bow')], 'sword')).toBe(0);
  });

  it('swaps "Mágica" for the +N, and keeps it without one', () => {
    expect(attackMarksFor(marks, 1)).toEqual({ silvered: true, adamantine: false, magical: false });
    expect(attackMarksFor(marks, 0)).toBe(marks);
    expect(attackMarksFor(undefined, 1)).toBeUndefined();
  });
});
