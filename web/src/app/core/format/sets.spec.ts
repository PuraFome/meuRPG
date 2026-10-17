import { toggledCapped, toggledIn } from './sets';

describe('toggledIn', () => {
  it('copies the set with the key put in or taken out', () => {
    const base = new Set(['a']);
    expect([...toggledIn(base, 'b')]).toEqual(['a', 'b']);
    expect([...toggledIn(base, 'a')]).toEqual([]);
    expect([...base]).toEqual(['a']);
  });
});

describe('toggledCapped', () => {
  it('adds up to the cap, takes out at any time and ignores a click past the cap', () => {
    expect([...toggledCapped(new Set(['a']), 'b', 2)]).toEqual(['a', 'b']);
    expect([...toggledCapped(new Set(['a', 'b']), 'c', 2)]).toEqual(['a', 'b']);
    expect([...toggledCapped(new Set(['a', 'b']), 'a', 2)]).toEqual(['b']);
  });
});
