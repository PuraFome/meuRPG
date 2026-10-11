import { RollModeKind } from '../../../gen/meurpg/play/v1/contest_types_pb';
import { checkNaturalMark } from './contest-view';
import { checkRoll } from './contest-testing';
import { diceNaturalMark, initiativeMark, naturalMark } from './natural-mark';

describe('naturalMark: the tag of a d20 that came up 20 or 1', () => {
  it('marks a 20 and a 1 and nothing else', () => {
    expect(naturalMark(20)).toBe('20 natural');
    expect(naturalMark(1)).toBe('1 natural');
    for (const face of [0, 2, 10, 19, 21]) {
      expect(naturalMark(face)).toBe('');
    }
  });

  it('follows the d20 that counts of a check roll', () => {
    expect(checkNaturalMark(checkRoll({ faces: [20] }))).toBe('20 natural');
    expect(checkNaturalMark(checkRoll({ faces: [1] }))).toBe('1 natural');
    expect(checkNaturalMark(checkRoll({ faces: [14] }))).toBe('');
    // Advantage keeps the higher, disadvantage the lower.
    expect(checkNaturalMark(checkRoll({ faces: [20, 4], mode: RollModeKind.ADVANTAGE }))).toBe(
      '20 natural',
    );
    expect(checkNaturalMark(checkRoll({ faces: [20, 4], mode: RollModeKind.DISADVANTAGE }))).toBe(
      '',
    );
    expect(checkNaturalMark(checkRoll({ faces: [1, 9], mode: RollModeKind.DISADVANTAGE }))).toBe(
      '1 natural',
    );
    expect(checkNaturalMark(checkRoll({ faces: [1, 9], mode: RollModeKind.ADVANTAGE }))).toBe('');
    expect(checkNaturalMark(undefined)).toBe('');
  });

  it('reads a DiceRoll at the counted index, and only a d20', () => {
    expect(diceNaturalMark({ diceSides: 20, faces: [20, 4], countedIndex: 0 })).toBe('20 natural');
    expect(diceNaturalMark({ diceSides: 20, faces: [20, 4], countedIndex: 1 })).toBe('');
    expect(diceNaturalMark({ diceSides: 20, faces: [1], countedIndex: 0 })).toBe('1 natural');
    expect(diceNaturalMark({ diceSides: 8, faces: [8], countedIndex: 0 })).toBe('');
    expect(diceNaturalMark({ diceSides: 20, faces: [], countedIndex: 0 })).toBe('');
    expect(diceNaturalMark(undefined)).toBe('');
  });

  it('reads the face of an initiative, empty until it is rolled', () => {
    expect(initiativeMark({ initiativeFace: 20 })).toBe('20 natural');
    expect(initiativeMark({ initiativeFace: 1 })).toBe('1 natural');
    expect(initiativeMark({ initiativeFace: 12 })).toBe('');
    expect(initiativeMark({})).toBe('');
  });
});
