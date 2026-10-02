import { abilityModifier, hitPointsPreview, validRoll } from './hit-points-preview';

describe('abilityModifier', () => {
  it('rounds down', () => {
    expect([8, 9, 10, 11, 12, 16, 17].map(abilityModifier)).toEqual([-1, -1, 0, 0, 1, 3, 3]);
  });
});

describe('validRoll', () => {
  it('accepts a face of the die and nothing else', () => {
    expect(validRoll(8, 12)).toBe(8);
    expect(validRoll(13, 12)).toBeNull();
    expect(validRoll(0, 12)).toBeNull();
    expect(validRoll(2.5, 12)).toBeNull();
    expect(validRoll(NaN, 12)).toBeNull();
    expect(validRoll(undefined, 12)).toBeNull();
  });
});

describe('hitPointsPreview', () => {
  // The E6-21 example: Barbarian 3 (d12), CON 16 (+3), level 2 rolled an 8.
  const preview = hitPointsPreview(12, 3, 16, [8]);

  it('takes the die maximum at level 1', () => {
    expect(preview.constitutionModifier).toBe(3);
    expect(preview.firstLevel).toBe(15);
  });

  it('adds the Constitution modifier to each roll', () => {
    expect(preview.levels).toEqual([
      { level: 2, roll: 8, gain: 11 },
      { level: 3, roll: null, gain: null },
    ]);
    expect(preview.total).toBe(26);
  });

  it('says which levels are missing and the range they allow', () => {
    expect(preview.missing).toEqual([3]);
    expect(preview.min).toBe(30);
    expect(preview.max).toBe(41);
  });

  it('is settled once every level has a roll', () => {
    const done = hitPointsPreview(12, 3, 16, [8, 5]);
    expect(done.missing).toEqual([]);
    expect(done.total).toBe(34);
    expect(done.min).toBe(34);
    expect(done.max).toBe(34);
  });

  it('never lets a level add less than 1, like the server', () => {
    const frail = hitPointsPreview(6, 2, 3, [1]); // CON 3 is -4
    expect(frail.firstLevel).toBe(2);
    expect(frail.levels[0].gain).toBe(1);
  });

  it('has no rows at level 1', () => {
    const first = hitPointsPreview(8, 1, 10, []);
    expect(first.levels).toEqual([]);
    expect(first.total).toBe(8);
  });

  it('ignores a roll that does not fit the die', () => {
    expect(hitPointsPreview(8, 2, 10, [20]).missing).toEqual([2]);
  });
});
