import {
  canLower,
  canRaise,
  costOf,
  missingDie,
  pointsSpent,
  resultOfSet,
  typedInRange,
} from './ability-methods';

const costs = [0, 1, 2, 3, 4, 5, 7, 9];

describe('ability methods', () => {
  it("adds the server's costs: 15, 14, 13, 10, 10 and 8 is 25 of 27", () => {
    const scores = { str: 10, dex: 14, con: 13, int: 8, wis: 15, cha: 10 };
    expect(costOf(14, costs, 8)).toBe(7);
    expect(costOf(7, costs, 8)).toBeNull();
    expect(costOf(16, costs, 8)).toBeNull();
    expect(pointsSpent(scores, costs, 8)).toBe(2 + 7 + 5 + 0 + 9 + 2);
  });

  it('lets a score go up only with the points left, and down only inside the table', () => {
    expect(canRaise(8, 27, costs, 8)).toBe(true);
    expect(canRaise(13, 1, costs, 8)).toBe(false);
    expect(canRaise(13, 2, costs, 8)).toBe(true);
    expect(canRaise(15, 27, costs, 8)).toBe(false);
    expect(canLower(8, costs, 8)).toBe(false);
    expect(canLower(9, costs, 8)).toBe(true);
  });

  it('strikes the first of the lowest dice of a stored set', () => {
    expect(resultOfSet({ dice: [6, 5, 5, 2], total: 16 })).toEqual({
      total: 16,
      dice: [6, 5, 5, 2],
      dropped: 3,
    });
    expect(resultOfSet({ dice: [3, 3, 4, 5], total: 12 }).dropped).toBe(0);
  });

  it('checks a typed value against the range', () => {
    expect(typedInRange(3, 3, 18)).toBe(true);
    expect(typedInRange(18, 3, 18)).toBe(true);
    expect(typedInRange(19, 3, 18)).toBe(false);
    expect(typedInRange(2, 3, 18)).toBe(false);
    expect(typedInRange(10.5, 3, 18)).toBe(false);
    expect(typedInRange(null, 3, 18)).toBe(false);
  });

  it('says which die is missing, or out of 1 to 6', () => {
    const full = Array.from({ length: 6 }, () => ['1', '2', '3', '4']);
    expect(missingDie(full)).toBe('');
    const missing = full.map((r) => [...r]);
    missing[5][3] = '';
    expect(missingDie(missing)).toBe('Falta o quarto dado da rolagem 6.');
    missing[0][0] = '7';
    expect(missingDie(missing)).toBe('O primeiro dado da rolagem 1 vai de 1 a 6.');
  });
});
