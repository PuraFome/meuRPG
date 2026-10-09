import type { DiceRoll } from '../../../gen/meurpg/play/v1/combat_pb';
import {
  damageFormula,
  damageWords,
  extraDiceOf,
  parseFace,
  parseSum,
  rollFormula,
  rollText,
  splitFormula,
  sumRange,
  treatedFormula,
  treatedPreview,
  treatedSentence,
  treatedSpeech,
  typedTotal,
} from './combat-dice';

function roll(over: Partial<DiceRoll>): DiceRoll {
  return {
    diceCount: 1,
    diceSides: 20,
    faces: [],
    modifier: 0,
    total: 0,
    physical: false,
    ...over,
  } as DiceRoll;
}

describe('the dice formula', () => {
  it('writes the d20 with its face and the bonus', () => {
    expect(rollFormula(roll({ faces: [13], modifier: 6, total: 19 }))).toBe('1d20 (13) + 6 = 19');
    expect(rollFormula(roll({ faces: [4], modifier: -1, total: 3 }))).toBe('1d20 (4) − 1 = 3');
  });

  it('writes damage with several dice, and without a modifier', () => {
    const crit = roll({ diceCount: 2, diceSides: 6, faces: [5, 4], modifier: 2, total: 11 });
    expect(damageFormula(crit, '')).toBe('2d6 (5, 4) + 2 = 11 de dano');
    expect(damageFormula(roll({ diceCount: 1, diceSides: 10, faces: [7], total: 7 }), 'fogo')).toBe(
      '1d10 (7) = 7 de fogo',
    );
  });

  it('writes a typed roll without the dice', () => {
    expect(rollFormula(roll({ faces: [16], modifier: 5, total: 21, physical: true }))).toBe(
      '16 + 5 = 21',
    );
    expect(typedTotal(16, 5)).toBe('16 + 5 = 21');
    const typed = roll({ diceCount: 1, diceSides: 8, modifier: 3, total: 9, physical: true });
    expect(damageFormula(typed, 'cortante')).toBe('6 + 3 = 9 de dano cortante');
  });

  it('says the damage type as words', () => {
    expect(damageWords(5, 'perfurante')).toBe('5 de dano perfurante');
    expect(damageWords(7, 'fogo')).toBe('7 de fogo');
    expect(damageWords(3, '')).toBe('3 de dano');
  });
});

describe('a typed roll', () => {
  it('takes 1 to 20 for a d20', () => {
    expect(parseFace('16')).toBe(16);
    expect(parseFace(' 20 ')).toBe(20);
    for (const bad of ['', '0', '21', '27', '1.5', '-3', 'abc', '1e1']) {
      expect(parseFace(bad)).toBeNull();
    }
  });

  it('takes N to N times the faces for damage (Q38)', () => {
    expect(sumRange(2, 6)).toEqual({ min: 2, max: 12 });
    expect(parseSum('9', 2, 12)).toBe(9);
    expect(parseSum('13', 2, 12)).toBeNull();
    expect(parseSum('1', 2, 12)).toBeNull();
  });
});

describe('a roll in a sentence', () => {
  it("writes the app's roll with its dice and a typed d20 as the number it showed", () => {
    expect(
      rollText(roll({ diceCount: 1, diceSides: 20, faces: [14], modifier: 0, total: 14 })),
    ).toBe('1d20 (14) = 14');
    expect(
      rollText(
        roll({ diceCount: 1, diceSides: 20, faces: [], modifier: 0, total: 1, physical: true }),
      ),
    ).toBe('1 · dado físico');
    expect(
      rollText(
        roll({ diceCount: 1, diceSides: 20, faces: [], modifier: 5, total: 21, physical: true }),
      ),
    ).toBe('16 + 5 = 21 · dado físico');
  });
});

describe('a damage with the extra dice of a critical hit (PM-03b)', () => {
  const brutal = { count: 1, name: 'Crítico Brutal' };
  const rolled = roll({
    diceCount: 3,
    diceSides: 12,
    faces: [7, 11, 4],
    modifier: 3,
    total: 25,
  });

  it("splits the groups with the feature's name: the critical's faces first, the extra ones last", () => {
    expect(splitFormula(rolled, brutal)).toBe('2d12 (7, 11) + 1d12 Crítico Brutal (4) + 3 = 25');
    expect(damageFormula(rolled, 'cortante', brutal)).toBe(
      '2d12 (7, 11) + 1d12 Crítico Brutal (4) + 3 = 25 de dano cortante',
    );
  });

  it('writes two and three extra dice of the higher levels', () => {
    const two = roll({ diceCount: 4, diceSides: 12, faces: [1, 2, 3, 4], modifier: 3, total: 13 });
    expect(splitFormula(two, { count: 2, name: 'Crítico Brutal' })).toBe(
      '2d12 (1, 2) + 2d12 Crítico Brutal (3, 4) + 3 = 13',
    );
  });

  it('keeps the maximum that came without rolling apart from the modifier', () => {
    const max = roll({ diceCount: 2, diceSides: 12, faces: [10, 5], modifier: 15, total: 30 });
    expect(splitFormula(max, brutal, 12)).toBe(
      '12 (máximo) + 1d12 (10) + 1d12 Crítico Brutal (5) + 3 = 30',
    );
  });

  it('cannot split what has no faces (typed dice) or no extra dice, and falls back to the plain line', () => {
    const typed = roll({ diceCount: 3, diceSides: 12, modifier: 3, total: 25, physical: true });
    expect(splitFormula(typed, brutal)).toBeNull();
    expect(damageFormula(typed, 'cortante', brutal)).toBe('22 + 3 = 25 de dano cortante');
    expect(splitFormula(rolled, { count: 0, name: '' })).toBeNull();
    expect(damageFormula(rolled, 'cortante')).toBe('3d12 (7, 11, 4) + 3 = 25 de dano cortante');
  });

  it('reads the extra dice of a pending or a log damage, and nothing without them', () => {
    expect(extraDiceOf({ extraDiceCount: 1, extraDiceNamePt: 'Crítico Brutal' })).toEqual(brutal);
    expect(extraDiceOf({ extraDiceCount: 0, extraDiceNamePt: '' })).toBeUndefined();
  });
});

describe('a d20 that Talento Confiável raised (PM-03b)', () => {
  const raised = roll({
    faces: [6],
    modifier: 9,
    total: 19,
    treatedAs: 10,
    treatedAsSource: 'feature:reliable-talent',
  });

  it('writes what came up, what it counted as and the feature, only when the rule changed the number', () => {
    expect(treatedFormula(raised)).toBe('d20: 6 → 10 (Talento Confiável) + 9 = 19');
    expect(rollFormula(raised)).toBe('d20: 6 → 10 (Talento Confiável) + 9 = 19');
    expect(rollText(raised)).toBe('d20: 6 → 10 (Talento Confiável) + 9 = 19');
    // A 14 is never marked: the plain formula stays.
    const plain = roll({ faces: [14], modifier: 9, total: 23 });
    expect(treatedFormula(plain)).toBeNull();
    expect(rollFormula(plain)).toBe('1d20 (14) + 9 = 23');
  });

  it('keeps the face that came up for a typed die, and names what the bonus is for when the page asks', () => {
    const typed = roll({ ...raised, physical: true });
    expect(rollText(typed)).toBe('d20: 6 → 10 (Talento Confiável) + 9 = 19');
    expect(treatedFormula(raised, 'Acrobacia')).toBe(
      'd20: 6 → 10 (Talento Confiável) + 9 (Acrobacia) = 19',
    );
  });

  it('shows the die that counted when two were rolled', () => {
    const two = roll({ ...raised, faces: [15, 6] });
    expect(treatedFormula(two)).toBe('d20: 6 → 10 (Talento Confiável) + 9 = 19');
  });

  it('says it in a sentence and for a screen reader', () => {
    expect(treatedSentence(raised)).toBe('O d20 de 6 contou como 10: perícia com proficiência.');
    expect(treatedSpeech(raised)).toBe('d20: 6, contou 10 por Talento Confiável, mais 9, total 19');
    expect(treatedSentence(roll({ faces: [14], modifier: 9, total: 23 }))).toBeNull();
    expect(treatedSpeech(roll({ faces: [14], modifier: 9, total: 23 }))).toBeNull();
  });

  it('previews the typed die: 6 becomes 10, a 14 is plain', () => {
    expect(treatedPreview(6, 9, 'feature:reliable-talent')).toEqual({
      text: '6 → 10 (Talento Confiável) + 9 = 19',
      total: 19,
    });
    expect(treatedPreview(9, 0, 'feature:reliable-talent')).toEqual({
      text: '9 → 10 (Talento Confiável) = 10',
      total: 10,
    });
    expect(treatedPreview(10, 9, 'feature:reliable-talent')).toBeNull();
    expect(treatedPreview(14, 9, 'feature:reliable-talent')).toBeNull();
  });
});
