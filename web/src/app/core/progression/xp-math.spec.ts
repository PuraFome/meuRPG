import { divisionSentence, eachLine, lostWords, parseAmount, shortDivision, splitAnnouncement, splitXp } from './xp-math';

const nbsp = ' ';

describe('splitXp (question 46: down, as in the book)', () => {
  it('splits exactly', () => {
    expect(splitXp(350, 2)).toEqual({ count: 2, each: 175, lost: 0 });
    expect(splitXp(150, 3)).toEqual({ count: 3, each: 50, lost: 0 });
  });

  it('rounds down and says what is lost', () => {
    expect(splitXp(350, 3)).toEqual({ count: 3, each: 116, lost: 2 });
    expect(splitXp(100, 3)).toEqual({ count: 3, each: 33, lost: 1 });
  });

  it('with nobody checked there is nothing to split', () => {
    expect(splitXp(350, 0)).toEqual({ count: 0, each: 0, lost: 0 });
  });

  it('gives each 0 when the total is smaller than the group', () => {
    expect(splitXp(2, 3)).toEqual({ count: 3, each: 0, lost: 2 });
  });

  it('ignores a total that is not a whole number', () => {
    expect(splitXp(Number.NaN, 3).each).toBe(0);
    expect(splitXp(-5, 3).each).toBe(0);
  });
});

describe('the lines of the split', () => {
  it('keeps the number and XP together', () => {
    expect(eachLine(splitXp(350, 3))).toBe(`116${nbsp}XP para cada`);
  });

  it('writes the division out, with the remainder', () => {
    expect(divisionSentence(350, splitXp(350, 3))).toBe(
      `350${nbsp}XP ÷ 3 = 116,67, arredondado para baixo. 2${nbsp}XP se perdem na divisão.`,
    );
  });

  it('says an exact division loses nothing', () => {
    expect(divisionSentence(350, splitXp(350, 2))).toBe(`350${nbsp}XP ÷ 2 = 175. Divisão exata, nada se perde.`);
  });

  it('writes the short sum beside the number, with the gold when there is some', () => {
    expect(shortDivision(150, splitXp(150, 3))).toBe(`150${nbsp}XP ÷ 3 = 50`);
    expect(shortDivision(120, splitXp(120, 3), 120)).toBe(`120${nbsp}PO = 120${nbsp}XP ÷ 3 = 40`);
    expect(shortDivision(350, splitXp(350, 3))).toBe(`350${nbsp}XP ÷ 3 = 116. 2${nbsp}XP se perdem na divisão.`);
  });

  it('says the lost XP in the singular and the plural', () => {
    expect(lostWords(1)).toBe('1 XP se perde na divisão.');
    expect(lostWords(2)).toBe('2 XP se perdem na divisão.');
  });

  it('announces the number, or that nobody is checked', () => {
    expect(splitAnnouncement(splitXp(350, 2))).toBe(`175${nbsp}XP para cada.`);
    expect(splitAnnouncement(splitXp(350, 0))).toBe('Ninguém marcado.');
  });
});

describe('parseAmount', () => {
  it('takes whole numbers from 1 to 1.000.000', () => {
    expect(parseAmount('1')).toBe(1);
    expect(parseAmount(' 150 ')).toBe(150);
    expect(parseAmount('1000000')).toBe(1_000_000);
  });

  it('refuses the rest', () => {
    for (const text of ['', '0', '-5', '1000001', '1,5', '1.000', 'dez', '12 XP', '00000000']) {
      expect(parseAmount(text), text).toBeNull();
    }
  });
});
