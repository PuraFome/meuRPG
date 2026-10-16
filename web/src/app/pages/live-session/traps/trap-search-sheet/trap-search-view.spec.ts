import { listKeyMove, rollLabel, typedLineOf } from './trap-search-view';

describe('listKeyMove', () => {
  it('moves the cursor and stops at both ends', () => {
    expect(listKeyMove('ArrowDown', 1, 2)).toBe(2);
    expect(listKeyMove('ArrowDown', 2, 2)).toBe(2);
    expect(listKeyMove('ArrowUp', 1, 2)).toBe(0);
    expect(listKeyMove('ArrowUp', 0, 2)).toBe(0);
    expect(listKeyMove('Home', 2, 2)).toBe(0);
    expect(listKeyMove('End', 0, 2)).toBe(2);
    expect(listKeyMove('End', 0, -1)).toBe(0);
  });

  it('picks on Enter and Space and ignores the other keys', () => {
    expect(listKeyMove('Enter', 0, 2)).toBe('pick');
    expect(listKeyMove(' ', 0, 2)).toBe('pick');
    expect(listKeyMove('a', 0, 2)).toBeNull();
  });
});

describe('rollLabel', () => {
  it('ends with the bonus of the chosen title, or with the skill alone when the sheet is unknown', () => {
    expect(rollLabel('Arcanismo', 'Arcanismo +8')).toBe('Role 1d20 para Arcanismo (+8)');
    expect(rollLabel('Atletismo', 'Atletismo −1')).toBe('Role 1d20 para Atletismo (−1)');
    expect(rollLabel('Percepção', null)).toBe('Role 1d20 para Percepção');
  });
});

describe('typedLineOf', () => {
  it('adds the bonus to the face, and shows the raised die only for a skill the feature covers', () => {
    expect(typedLineOf(6, 4, 'skill:acrobatics', undefined)).toBe(typedLineOf(6, 4, null, []));
    expect(typedLineOf(6, 4, 'skill:acrobatics', ['skill:acrobatics'])).toContain('10');
    expect(typedLineOf(6, 4, 'skill:arcana', ['skill:acrobatics'])).toBe(
      typedLineOf(6, 4, null, undefined),
    );
  });
});
