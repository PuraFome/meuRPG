import { ABILITY_KEYS } from '../characters/characters.types';
import {
  STANDARD_ARRAY,
  RollDie,
  emptyPlacement,
  freeCount,
  holderOf,
  place,
  rollAbilityScore,
  rollAbilityScores,
  rollDie,
  standardArrayResults,
} from './dice';

/** A die that returns the given faces in order. */
function fixed(...faces: number[]): RollDie {
  let i = 0;
  return () => faces[i++ % faces.length];
}

describe('rollDie', () => {
  it('only returns faces of the die, and every face turns up', () => {
    const seen = new Set<number>();
    for (let i = 0; i < 600; i++) {
      const face = rollDie(6);
      expect(Number.isInteger(face)).toBe(true);
      expect(face).toBeGreaterThanOrEqual(1);
      expect(face).toBeLessThanOrEqual(6);
      seen.add(face);
    }
    expect(seen.size).toBe(6);
  });

  it('works for the biggest hit die', () => {
    for (let i = 0; i < 200; i++) {
      expect(rollDie(12)).toBeLessThanOrEqual(12);
    }
  });
});

describe('rollAbilityScore (4d6, drop lowest)', () => {
  it('adds the three highest dice and strikes the lowest', () => {
    expect(rollAbilityScore(fixed(6, 6, 5, 2))).toEqual({
      total: 17,
      dice: [6, 6, 5, 2],
      dropped: 3,
    });
  });

  it('strikes the lowest wherever it fell', () => {
    const result = rollAbilityScore(fixed(1, 4, 4, 3));
    expect(result.total).toBe(11);
    expect(result.dropped).toBe(0);
  });

  it('strikes only one die on a tie', () => {
    const result = rollAbilityScore(fixed(2, 2, 2, 2));
    expect(result.total).toBe(6);
    expect(result.dropped).toBe(0);
  });

  it('never goes below 3 or above 18', () => {
    expect(rollAbilityScore(fixed(1, 1, 1, 1)).total).toBe(3);
    expect(rollAbilityScore(fixed(6, 6, 6, 6)).total).toBe(18);
  });
});

describe('rollAbilityScores', () => {
  it('rolls six scores, best first', () => {
    // Six sets of four dice: 12, 15, 9, 18, 6 and 14.
    const results = rollAbilityScores(
      fixed(
        4,
        4,
        4,
        1,
        /**/ 6,
        5,
        4,
        1,
        /**/ 3,
        3,
        3,
        3,
        /**/ 6,
        6,
        6,
        1,
        /**/ 2,
        2,
        2,
        2,
        /**/ 5,
        5,
        4,
        4,
      ),
    );
    expect(results.map((r) => r.total)).toEqual([18, 15, 14, 12, 9, 6]);
  });
});

describe('the standard array', () => {
  it('is 15, 14, 13, 12, 10 and 8, with no dice', () => {
    expect(STANDARD_ARRAY).toEqual([15, 14, 13, 12, 10, 8]);
    expect(standardArrayResults().map((r) => r.total)).toEqual([15, 14, 13, 12, 10, 8]);
    expect(standardArrayResults().every((r) => r.dice.length === 0)).toBe(true);
  });
});

describe('placing results', () => {
  it('starts with every ability free', () => {
    const p = emptyPlacement();
    expect(freeCount(p)).toBe(6);
    expect(holderOf(p, 0)).toBeNull();
  });

  it('puts a result on an ability', () => {
    const p = place(emptyPlacement(), 2, 'dex');
    expect(p.dex).toBe(2);
    expect(holderOf(p, 2)).toBe('dex');
    expect(freeCount(p)).toBe(5);
  });

  it('moves a result when its ability is empty: the old one is freed', () => {
    let p = place(emptyPlacement(), 0, 'str');
    p = place(p, 0, 'cha');
    expect(p.str).toBeNull();
    expect(p.cha).toBe(0);
  });

  it('swaps two placed results', () => {
    let p = place(emptyPlacement(), 0, 'str');
    p = place(p, 1, 'dex');
    p = place(p, 1, 'str');
    expect(p.str).toBe(1);
    expect(p.dex).toBe(0);
  });

  it('replaces a result with a free one, freeing the old one', () => {
    let p = place(emptyPlacement(), 0, 'str');
    p = place(p, 3, 'str');
    expect(p.str).toBe(3);
    expect(holderOf(p, 0)).toBeNull();
  });

  it('never gives one result to two abilities', () => {
    let p = emptyPlacement();
    ABILITY_KEYS.forEach((key, i) => (p = place(p, i, key)));
    p = place(p, 0, 'int');
    const held = ABILITY_KEYS.map((key) => p[key]);
    expect(new Set(held).size).toBe(6);
  });
});
