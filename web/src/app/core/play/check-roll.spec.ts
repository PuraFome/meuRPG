import { Code, ConnectError } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';

import { DiceRollSchema } from '../../../gen/meurpg/play/v1/combat_pb';
import { RollMode } from '../../../gen/meurpg/play/v1/combat_rolls_pb';
import { checkFaces, needsTwoD20, searchFaces, takesTwo } from './check-roll';

const die = (faces: number[], countedIndex = 0) =>
  create(DiceRollSchema, { diceCount: faces.length, diceSides: 20, faces, countedIndex });

describe('check rolls with a pair of d20', () => {
  it('marks the counted die of a pair and gives nothing for a single die', () => {
    expect(checkFaces(die([5, 14], 1))).toEqual([
      { value: 5, counts: false },
      { value: 14, counts: true },
    ]);
    expect(checkFaces(die([9]))).toEqual([]);
    expect(checkFaces(undefined)).toEqual([]);
  });

  it('reads the two rolls of a search: the first counts when the mode is not Normal', () => {
    expect(searchFaces(RollMode.DISADVANTAGE, die([4]), die([11]))).toEqual([
      { value: 4, counts: true },
      { value: 11, counts: false },
    ]);
    expect(searchFaces(RollMode.NORMAL, die([4]), die([11]))).toEqual([]);
    expect(searchFaces(RollMode.ADVANTAGE, die([4]), undefined)).toEqual([]);
  });

  it('knows the refusal that asks for two d20 by its code and message', () => {
    const asked = new ConnectError(
      'the roll takes 2 d20: type 2 face(s) in d20_faces',
      Code.InvalidArgument,
    );
    expect(needsTwoD20(asked)).toBe(true);
    expect(needsTwoD20(new ConnectError('d20_face must be 1 to 20', Code.InvalidArgument))).toBe(
      false,
    );
    expect(needsTwoD20(new ConnectError('the roll takes 2 d20', Code.Internal))).toBe(false);
    expect(needsTwoD20(new Error('x'))).toBe(false);
  });

  it('says which modes take two dice', () => {
    expect(takesTwo(RollMode.ADVANTAGE)).toBe(true);
    expect(takesTwo(RollMode.DISADVANTAGE)).toBe(true);
    expect(takesTwo(RollMode.NORMAL)).toBe(false);
  });
});
