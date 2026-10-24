import { dcLabel } from './save-dc';

describe('dcLabel', () => {
  it('says "CD 14" for a DC the server sent', () => {
    expect(dcLabel(14)).toBe('CD 14');
    expect(dcLabel(23)).toBe('CD 23');
  });

  it('says nothing when there is no DC: absent or 0 (the server sends none to a player who may not read it)', () => {
    expect(dcLabel(undefined)).toBe('');
    expect(dcLabel(0)).toBe('');
  });
});
