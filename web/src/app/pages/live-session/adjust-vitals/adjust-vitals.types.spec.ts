import { brisaVitals, pensantusVitals } from '../testing';
import { changeBetween, draftFrom, sameChange } from './adjust-vitals.types';

describe('changeBetween', () => {
  it('is null when nothing changed', () => {
    const v = pensantusVitals();
    expect(changeBetween(v, draftFrom(v))).toBeNull();
  });

  it('sends only what changed, as absolute values', () => {
    const v = pensantusVitals();
    const draft = { ...draftFrom(v), hitPointsCurrent: 12, slotsUsed: { 1: 3, 2: 0 } };
    expect(changeBetween(v, draft)).toEqual({
      hitPointsCurrent: 12,
      spellSlotsUsed: [{ level: 1, used: 3 }],
    });
  });

  it('covers temporary HP, hit dice and pact slots', () => {
    const v = brisaVitals({ pactSlots: { slotLevel: 2, total: 2, used: 0 } });
    const draft = { ...draftFrom(v), hitPointsTemporary: 0, hitDiceUsed: 2, pactSlotsUsed: 1 };
    expect(changeBetween(v, draft)).toEqual({
      hitPointsTemporary: 0,
      pactSlotsUsed: 1,
      hitDiceUsed: 2,
    });
  });
});

describe('sameChange', () => {
  it('tells a retry (same numbers) from a new correction', () => {
    expect(sameChange({ hitPointsCurrent: 12 }, { hitPointsCurrent: 12 })).toBe(true);
    expect(sameChange({ hitPointsCurrent: 12 }, { hitPointsCurrent: 11 })).toBe(false);
  });
});
