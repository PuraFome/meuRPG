import { CriticalDamageRule } from '../../../gen/meurpg/play/v1/combat_pb';
import { criticalHint, criticalTypedHint, fixedParts } from './critical';

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
    expect(hint.line).toBe('Acerto crítico: o máximo mais uma rolagem. O máximo dos dados (12) já vale sem rolar; role 2d6 uma vez.');
  });

  it('says nothing for a hit that is not a critical one', () => {
    expect(criticalHint(CriticalDamageRule.UNSPECIFIED, 1, 8, 0)).toBeNull();
  });

  it('gives a physical roll one clear instruction: roll the dice and type only what came out; the app adds the rest', () => {
    expect(criticalTypedHint(CriticalDamageRule.MAX_PLUS_ROLL, '2d6', 2, 12, 12)).toBe('Role 2d6 e digite só o que saiu, de 2 a 12. O app soma o resto.');
    expect(criticalTypedHint(CriticalDamageRule.DOUBLED_DICE, '4d6', 4, 24, 0)).toBe('Digite a soma dos dados, de 4 a 24. O app soma o modificador.');
    expect(criticalTypedHint(CriticalDamageRule.UNSPECIFIED, '1d8', 1, 8, 0)).toBe('Digite a soma dos dados, de 1 a 8. O app soma o modificador.');
  });

  it('names the fixed parts the app adds, from the server\'s own numbers (critical_max and the modifier)', () => {
    expect(fixedParts(8, 3)).toBe('+ 8 do crítico + 3 de modificador');
    expect(fixedParts(8, -1)).toBe('+ 8 do crítico − 1 de modificador');
    expect(fixedParts(8, 0)).toBe('+ 8 do crítico');
    expect(fixedParts(0, 3)).toBe('');
  });
});
