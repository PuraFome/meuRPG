import type { DiceRoll } from '../../../gen/meurpg/play/v1/combat_pb';
import {
  damageFormula,
  damageWords,
  parseFace,
  parseSum,
  rollFormula,
  sumRange,
  typedTotal,
} from './combat-dice';

function roll(over: Partial<DiceRoll>): DiceRoll {
  return { diceCount: 1, diceSides: 20, faces: [], modifier: 0, total: 0, physical: false, ...over } as DiceRoll;
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
    expect(rollFormula(roll({ faces: [16], modifier: 5, total: 21, physical: true }))).toBe('16 + 5 = 21');
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
