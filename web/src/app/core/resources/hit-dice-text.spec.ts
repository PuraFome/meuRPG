import {
  dieName,
  diceLeft,
  hitDiceLeftWords,
  hitDiceWords,
  listWords,
  totalDiceLeft,
} from './hit-dice-text';

describe('hit dice words', () => {
  it('names a die by its size', () => {
    expect(dieName(10)).toBe('d10');
  });

  it('reads a list the way it is said', () => {
    expect(listWords([])).toBe('');
    expect(listWords(['a'])).toBe('a');
    expect(listWords(['a', 'b'])).toBe('a e b');
    expect(listWords(['a', 'b', 'c'])).toBe('a, b e c');
  });

  it('says the dice by size, as the level-up summary does', () => {
    expect(hitDiceWords([{ faces: 10, count: 5 }])).toBe('5d10');
    expect(
      hitDiceWords([
        { faces: 10, count: 5 },
        { faces: 6, count: 1 },
      ]),
    ).toBe('5d10 e 1d6');
    expect(hitDiceWords([])).toBe('—');
    expect(hitDiceWords([{ faces: 8, count: 0 }])).toBe('—');
  });

  it('says what is left of each size next to how many there are', () => {
    expect(hitDiceLeftWords([{ faces: 6, total: 3, used: 1 }])).toBe('2 de 3d6');
    expect(
      hitDiceLeftWords([
        { faces: 10, total: 5, used: 2 },
        { faces: 6, total: 1, used: 0 },
      ]),
    ).toBe('3 de 5d10 e 1 de 1d6');
    expect(hitDiceLeftWords([])).toBe('—');
  });

  it('never counts less than none left', () => {
    expect(diceLeft({ faces: 8, total: 2, used: 5 })).toBe(0);
    expect(
      totalDiceLeft([
        { faces: 10, total: 5, used: 2 },
        { faces: 6, total: 1, used: 1 },
      ]),
    ).toBe(3);
  });
});
