import { create } from '@bufbuild/protobuf';
import { TestBed } from '@angular/core/testing';

import { DiceRollSchema } from '../../../gen/meurpg/play/v1/combat_pb';
import {
  AdvantageSourceKind,
  AdvantageSourceSchema,
  RollMode,
} from '../../../gen/meurpg/play/v1/combat_rolls_pb';
import { D20Faces } from '../../pages/live-session/combat/roll-mode/d20-faces';
import { rollFormula } from './combat-dice';
import { d20Formula, sourceLine } from './roll-mode';

const plain = (t: string | null | undefined) => (t ?? '').replace(/\s+/g, ' ').trim();

const d4 = (face: number, sign = 1) => ({
  sourceNamePt: 'Bênção',
  sourceKey: 'spell:bless',
  faces: 4,
  sign,
  face,
});

describe('the dice an effect adds to a roll (RN-22)', () => {
  it('writes "1d20 (14) + 5 + 1d4 (3) = 22" for the app\'s roll', () => {
    const roll = create(DiceRollSchema, {
      diceCount: 1,
      diceSides: 20,
      faces: [14],
      modifier: 5,
      total: 22,
      extraDice: [d4(3)],
    });
    expect(rollFormula(roll)).toBe('1d20 (14) + 5 + 1d4 (3) = 22');
  });

  it('takes the die away for Perdição', () => {
    const roll = create(DiceRollSchema, {
      diceCount: 1,
      diceSides: 20,
      faces: [14],
      modifier: 5,
      total: 16,
      extraDice: [d4(3, -1)],
    });
    expect(rollFormula(roll)).toBe('1d20 (14) + 5 − 1d4 (3) = 16');
  });

  it('keeps the typed d20 out of the sum of the dice for a physical roll', () => {
    const roll = create(DiceRollSchema, {
      diceCount: 1,
      diceSides: 20,
      faces: [],
      physical: true,
      modifier: 5,
      total: 22,
      extraDice: [d4(3)],
    });
    expect(rollFormula(roll)).toBe('14 + 5 + 1d4 (3) = 22');
  });

  it('adds the dice to the pair of d20 too, and leaves a roll without them as it was', () => {
    const pair = create(DiceRollSchema, {
      diceCount: 2,
      diceSides: 20,
      faces: [7, 15],
      countedIndex: 1,
      modifier: 5,
      total: 23,
      extraDice: [d4(3)],
    });
    expect(d20Formula(pair)).toBe('15 + 5 + 1d4 (3) = 23');
    const plainRoll = create(DiceRollSchema, {
      diceCount: 1,
      diceSides: 20,
      faces: [14],
      modifier: 5,
      total: 19,
    });
    expect(rollFormula(plainRoll)).toBe('1d20 (14) + 5 = 19');
  });
});

describe('the sources of a roll that an effect writes', () => {
  const effectDie = create(AdvantageSourceSchema, {
    kind: AdvantageSourceKind.EFFECT_DIE,
    textPt: 'Bênção, de Tavo: ataques e testes de resistência.',
  });
  const other = create(AdvantageSourceSchema, {
    kind: AdvantageSourceKind.OTHER_SOURCE,
    effect: RollMode.ADVANTAGE,
    textPt: 'Outra fonte: vantagem',
  });
  const check = create(AdvantageSourceSchema, {
    kind: AdvantageSourceKind.EFFECT_CHECK,
    effect: RollMode.ADVANTAGE,
    textPt: 'Efeito ativo: vantagem em testes de habilidade',
  });
  const ordinary = create(AdvantageSourceSchema, {
    kind: AdvantageSourceKind.DODGING_TARGET,
    effect: RollMode.DISADVANTAGE,
    textPt: 'Esquivando',
  });

  it('read as the server wrote them, with no word of advantage in front', () => {
    expect(sourceLine(effectDie)).toBe('Bênção, de Tavo: ataques e testes de resistência.');
    expect(sourceLine(other)).toBe('Outra fonte: vantagem');
    expect(sourceLine(check)).toBe('Efeito ativo: vantagem em testes de habilidade');
    expect(sourceLine(ordinary)).toBe('Desvantagem: Esquivando');
  });

  it('are the lines under the d20 of a result', () => {
    const fixture = TestBed.createComponent(D20Faces);
    fixture.componentRef.setInput('roll', create(DiceRollSchema, { diceCount: 1, faces: [14] }));
    fixture.componentRef.setInput('sources', [effectDie, other]);
    fixture.detectChanges();
    const lines = Array.from((fixture.nativeElement as HTMLElement).querySelectorAll('.d20__src'));
    expect(lines.map((l) => plain(l.textContent))).toEqual([
      'Bênção, de Tavo: ataques e testes de resistência.',
      'Outra fonte: vantagem',
    ]);
  });
});
