import { awardInputOf } from './award-input';

describe('awardInputOf', () => {
  it('is the gold itself in a gold campaign', () => {
    expect(awardInputOf(true, 90, { encounterId: 'e', amount: 90 })).toEqual({
      mode: 'gold',
      gold: 90,
    });
  });

  it("is the combat's own award while the total is still the combat's", () => {
    expect(awardInputOf(false, 150, { encounterId: 'e', amount: 150 })).toEqual({
      mode: 'enemies',
      encounterId: 'e',
    });
  });

  it('is a manual award once the total changed, or without a combat', () => {
    expect(awardInputOf(false, 300, { encounterId: 'e', amount: 150 })).toEqual({
      mode: 'manual',
      amount: 300,
    });
    expect(awardInputOf(false, 150, { amount: 150 })).toEqual({ mode: 'manual', amount: 150 });
  });
});
