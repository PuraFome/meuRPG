import { LIST, article, newNames, row } from './levelup-summary-rows';

describe('the helpers of the summary rows', () => {
  it('drops a row whose two sides are the same, and keeps the others with their line under', () => {
    expect(row('hp', 'Pontos de vida', '23', '23')).toBeNull();
    expect(row('hp', 'Pontos de vida', '23', '30', 'Média')).toEqual({
      key: 'hp',
      label: 'Pontos de vida',
      before: '23',
      after: '30',
      sub: 'Média',
    });
    expect(row('hp', 'Pontos de vida', '23', '30')?.sub).toBe('');
  });

  it('writes the new names, one or many, or nothing', () => {
    expect(newNames([], 'Novo', 'Novos')).toBe('');
    expect(newNames(['Luz'], 'Novo', 'Novos')).toBe('Novo: Luz');
    expect(newNames(['Luz', 'Raio'], 'Novo', 'Novos')).toBe('Novos: Luz e Raio');
    expect(LIST.format(['a', 'b', 'c'])).toBe('a, b e c');
  });

  it('says "a" for the two feminine classes and "o" for the rest', () => {
    expect(article('Bruxa')).toBe('a');
    expect(article('Druida')).toBe('a');
    expect(article('Mago')).toBe('o');
  });
});
