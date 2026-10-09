import { CriticalDamageRule } from '../../../gen/meurpg/play/v1/combat_pb';
import {
  brutalLabel,
  brutalTyped,
  brutalTypedHint,
  criticalHint,
  criticalSentence,
  criticalSum,
  criticalTypedHint,
  diceToRoll,
  fixedParts,
  hasExtraDice,
  typedRange,
} from './critical';

describe('the critical hint (RN-24)', () => {
  it('says "role os dados duas vezes" for the SRD rule, with the dice the server already doubled', () => {
    const hint = criticalHint(CriticalDamageRule.DOUBLED_DICE, 4, 6, 0)!;
    expect(hint.rule).toBe('role os dados duas vezes');
    expect(hint.line).toBe('Acerto crítico: role os dados duas vezes (4d6 no total).');
    expect(hint.fixed).toBe('');
  });

  it('says "o máximo mais uma rolagem" with the maximum as a fixed part, and rolls the dice once', () => {
    const hint = criticalHint(CriticalDamageRule.MAX_PLUS_ROLL, 2, 6, 12)!;
    expect(hint.rule).toBe('o máximo mais uma rolagem');
    expect(hint.fixed).toBe('12 de máximo dos dados');
    expect(hint.line).toBe(
      'Acerto crítico: o máximo mais uma rolagem. O máximo dos dados (12) já vale sem rolar; role 2d6 uma vez.',
    );
  });

  it('does not name a maximum that is zero: the dice are simply rolled once', () => {
    const hint = criticalHint(CriticalDamageRule.MAX_PLUS_ROLL, 1, 8, 0)!;
    expect(hint.line).toBe('Acerto crítico: o máximo mais uma rolagem. Role 1d8 uma vez.');
    expect(hint.fixed).toBe('');
  });

  it('says nothing for a hit that is not a critical one', () => {
    expect(criticalHint(CriticalDamageRule.UNSPECIFIED, 1, 8, 0)).toBeNull();
  });

  it('gives a physical roll one clear instruction: roll the dice and type only what came out; the app adds the rest', () => {
    expect(criticalTypedHint(CriticalDamageRule.MAX_PLUS_ROLL, '2d6', 2, 12, 12)).toBe(
      'Role 2d6 e digite só o que saiu, de 2 a 12. O app soma o resto.',
    );
    expect(criticalTypedHint(CriticalDamageRule.DOUBLED_DICE, '4d6', 4, 24, 5)).toBe(
      'Digite a soma dos dados, de 4 a 24. O app soma o modificador.',
    );
    expect(criticalTypedHint(CriticalDamageRule.DOUBLED_DICE, '4d6', 4, 24, 0)).toBe(
      'Digite a soma dos dados, de 4 a 24. O app soma o modificador.',
    );
    expect(criticalTypedHint(CriticalDamageRule.UNSPECIFIED, '1d8', 1, 8, 0)).toBe(
      'Digite a soma dos dados, de 1 a 8. O app soma o modificador.',
    );
  });

  it("names the fixed parts the app adds, from the server's own numbers (critical_max and the modifier)", () => {
    expect(fixedParts(8, 3)).toBe('+ 8 do crítico + 3 de modificador');
    expect(fixedParts(8, -1)).toBe('+ 8 do crítico − 1 de modificador');
    expect(fixedParts(8, 0)).toBe('+ 8 do crítico');
    expect(fixedParts(0, 3)).toBe('');
  });
});

describe('the critical with the extra dice of Crítico Brutal (PM-03b)', () => {
  const doubled = {
    diceCount: 2,
    diceSides: 12,
    bonus: 3,
    criticalRule: CriticalDamageRule.DOUBLED_DICE,
    criticalMax: 0,
    extraDiceCount: 1,
    extraDiceNamePt: 'Crítico Brutal',
  };
  const maxPlusRoll = {
    ...doubled,
    diceCount: 1,
    criticalRule: CriticalDamageRule.MAX_PLUS_ROLL,
    criticalMax: 12,
  };

  it('counts the dice to roll and the range of the typed sum, for the levels 9, 13 and 17', () => {
    expect(hasExtraDice(doubled)).toBe(true);
    expect(hasExtraDice({ ...doubled, extraDiceCount: 0 })).toBe(false);
    expect([1, 2, 3].map((n) => diceToRoll({ ...doubled, extraDiceCount: n }))).toEqual([3, 4, 5]);
    expect([1, 2, 3].map((n) => typedRange({ ...doubled, extraDiceCount: n }))).toEqual([
      { min: 3, max: 36 },
      { min: 4, max: 48 },
      { min: 5, max: 60 },
    ]);
    // The maximum enters alone: the dice to roll are the critical's one and the feature's.
    expect(typedRange(maxPlusRoll)).toEqual({ min: 2, max: 24 });
  });

  it('writes the whole sum before the roll, the groups growing with the level', () => {
    expect(criticalSum(doubled)).toBe('2d12 + 1d12 + 3');
    expect(criticalSum({ ...doubled, extraDiceCount: 3 })).toBe('2d12 + 3d12 + 3');
    expect(criticalSum({ ...doubled, bonus: 0 })).toBe('2d12 + 1d12');
    expect(criticalSum({ ...doubled, bonus: -1 })).toBe('2d12 + 1d12 − 1');
    expect(criticalSum(maxPlusRoll)).toBe('12 + 1d12 + 1d12 + 3');
  });

  it('says which part is the critical and which the feature, with the level it comes at', () => {
    expect(criticalSentence(doubled, 'cortante')).toBe(
      '2d12 do crítico (dados dobrados) e 1d12 do Crítico Brutal (nível 9), mais 3 de modificador, de cortante.',
    );
    expect(criticalSentence({ ...doubled, extraDiceCount: 2, bonus: 0 }, '')).toBe(
      '2d12 do crítico (dados dobrados) e 2d12 do Crítico Brutal (nível 13).',
    );
    expect(criticalSentence({ ...doubled, extraDiceCount: 3 }, 'cortante')).toContain('(nível 17)');
    expect(criticalSentence(maxPlusRoll, 'cortante')).toBe(
      'O máximo do crítico (12) vale sem rolar; 1d12 do crítico e 1d12 do Crítico Brutal (nível 9), mais 3 de modificador, de cortante.',
    );
  });

  it('titles the typed roll with the dice to roll and the weapon, and says whose dice they are', () => {
    expect(brutalLabel(doubled, 'o Machado grande')).toBe('Role 3d12 para o Machado grande (+3)');
    expect(brutalLabel({ ...doubled, bonus: 0 }, 'o Machado grande')).toBe(
      'Role 3d12 para o Machado grande',
    );
    expect(brutalTypedHint(doubled)).toBe(
      '2d12 do crítico e 1d12 do Crítico Brutal. Role os três dados e digite a soma (3 a 36).',
    );
    expect(brutalTypedHint({ ...doubled, extraDiceCount: 2 })).toContain(
      'Role os quatro dados e digite a soma (4 a 48).',
    );
    expect(brutalTypedHint(maxPlusRoll)).toBe(
      'O máximo do crítico (12) já está contado. Role 1d12 do crítico e 1d12 do Crítico Brutal; digite a soma (2 a 24).',
    );
  });

  it('writes the live total of the typed sum with the groups', () => {
    expect(brutalTyped(doubled, 22)).toBe('22 (3d12) + 3 = 25');
    expect(brutalTyped(maxPlusRoll, 10)).toBe('12 (máximo) + 10 (2d12) + 3 = 25');
  });
});
