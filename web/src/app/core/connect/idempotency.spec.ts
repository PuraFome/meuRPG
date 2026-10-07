import { ActionKey, newKey } from './idempotency';

describe('ActionKey', () => {
  it('makes UUIDs', () => {
    expect(newKey()).toMatch(/^[0-9a-f-]{36}$/);
  });

  it('keeps the key for the same values, so a retry sends the same key', () => {
    const k = new ActionKey();
    const first = k.keyFor(['Mirathel', 1]);
    expect(k.keyFor(['Mirathel', 1])).toBe(first);
    expect(k.keyFor(['Mirathel', 1])).toBe(first);
  });

  it('makes a new key when the values changed: it is another action', () => {
    const k = new ActionKey();
    const first = k.keyFor('a');
    const second = k.keyFor('b');
    expect(second).not.toBe(first);
    // Going back to the first values is again another action: the first key was used for 'a'.
    expect(k.keyFor('a')).not.toBe(first);
  });

  it('makes a new key after the action worked, even for the same values', () => {
    const k = new ActionKey();
    const first = k.keyFor('same');
    k.renew();
    expect(k.keyFor('same')).not.toBe(first);
  });
});
