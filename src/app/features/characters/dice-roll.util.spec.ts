import { describe, it, expect } from 'vitest';
import { roll4d6DropLowest, rollAttributeSet } from './dice-roll.util';

/** Create a deterministic random source from a sequence of [0,1) values. */
function sequenceRandom(values: number[]): () => number {
  let i = 0;
  return () => {
    const v = values[i];
    i = (i + 1) % values.length;
    return v;
  };
}

describe('roll4d6DropLowest', () => {
  it('drops the lowest of four dice', () => {
    // Die values depend on random(): Math.floor(random() * 6) + 1.
    // To get dice [1, 2, 3, 4], pass random values:
    //   0/6 → die 1,  1/6 → die 2,  2/6 → die 3,  3/6 → die 4
    const random = sequenceRandom([0 / 6, 1 / 6, 2 / 6, 3 / 6]);
    // Sorted rolls: [1, 2, 3, 4] → drop 1 → sum 2+3+4 = 9
    expect(roll4d6DropLowest(random)).toBe(9);
  });

  it('returns 3 when every die is the minimum (1)', () => {
    // random() = 0 → die = 1 for all four rolls
    expect(roll4d6DropLowest(() => 0)).toBe(3); // 1+1+1 = 3
  });

  it('returns 18 when every die is the maximum (6)', () => {
    // random() ≈ 0.999... → die = 6 for all four rolls
    const random = sequenceRandom([0.999, 0.999, 0.999, 0.999]);
    expect(roll4d6DropLowest(random)).toBe(18); // 6+6+6 = 18
  });
});

describe('rollAttributeSet', () => {
  it('returns exactly 6 values each in 3-18, sorted descending', () => {
    const random = sequenceRandom([0 / 6, 1 / 6, 2 / 6, 3 / 6]);
    const set = rollAttributeSet(random);
    expect(set).toHaveLength(6);
    for (const score of set) {
      expect(score).toBeGreaterThanOrEqual(3);
      expect(score).toBeLessThanOrEqual(18);
    }
    // Check sorted descending
    for (let i = 1; i < set.length; i++) {
      expect(set[i]).toBeLessThanOrEqual(set[i - 1]);
    }
  });

  it('all-zero random returns [3,3,3,3,3,3]', () => {
    const set = rollAttributeSet(() => 0);
    expect(set).toEqual([3, 3, 3, 3, 3, 3]);
  });

  it('every value stays in 3-18 across 10 000 random rolls', () => {
    for (let i = 0; i < 10_000; i++) {
      const score = roll4d6DropLowest();
      expect(score).toBeGreaterThanOrEqual(3);
      expect(score).toBeLessThanOrEqual(18);
    }
  });
});