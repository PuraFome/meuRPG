import { DC_MAX, DC_MIN, dcOf } from './group-check-dc';

describe('dcOf', () => {
  it('reads an empty field as no DC', () => {
    expect(dcOf('')).toBe(0);
    expect(dcOf('   ')).toBe(0);
  });

  it('takes a whole number from the lowest DC to the highest, spaces around it allowed', () => {
    expect(dcOf(String(DC_MIN))).toBe(DC_MIN);
    expect(dcOf(' 13 ')).toBe(13);
    expect(dcOf(String(DC_MAX))).toBe(DC_MAX);
  });

  it('refuses what the server would: out of range, fractions and words', () => {
    expect(dcOf('0')).toBeNull();
    expect(dcOf(String(DC_MAX + 1))).toBeNull();
    expect(dcOf('-3')).toBeNull();
    expect(dcOf('12.5')).toBeNull();
    expect(dcOf('doze')).toBeNull();
  });
});
