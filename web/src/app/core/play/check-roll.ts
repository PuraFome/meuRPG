import { Code, ConnectError } from '@connectrpc/connect';

import type { DiceRoll } from '../../../gen/meurpg/play/v1/combat_pb';
import { RollMode } from '../../../gen/meurpg/play/v1/combat_rolls_pb';

/** A d20 of a check, with whether it is the one that counts. */
export interface CheckFace {
  readonly value: number;
  readonly counts: boolean;
}

/** The two d20 of a roll made with advantage or disadvantage, the counted one marked; empty for a single die. */
export function checkFaces(roll: DiceRoll | undefined): readonly CheckFace[] {
  if (!roll || roll.faces.length < 2) {
    return [];
  }
  return roll.faces.map((value, i) => ({ value, counts: i === roll.countedIndex }));
}

/** The pair of a trap search: `roll` is the die that counts and `second` the other when the mode is not Normal. */
export function searchFaces(
  mode: RollMode,
  roll: DiceRoll | undefined,
  second: DiceRoll | undefined,
): readonly CheckFace[] {
  if (!roll || !second || mode === RollMode.NORMAL || mode === RollMode.UNSPECIFIED) {
    return [];
  }
  const first = roll.faces[0];
  const other = second.faces[0];
  return first === undefined || other === undefined
    ? []
    : [
        { value: first, counts: true },
        { value: other, counts: false },
      ];
}

/** The server refused a typed roll because the roll takes two d20 ("the roll takes 2 d20"): the form shows two fields and tries again. */
export function needsTwoD20(err: unknown): boolean {
  const e = ConnectError.from(err, Code.Unavailable);
  return e.code === Code.InvalidArgument && /takes 2 d20/.test(e.rawMessage);
}

/** Whether a result's mode makes the next physical roll take two d20. */
export function takesTwo(mode: RollMode): boolean {
  return mode === RollMode.ADVANTAGE || mode === RollMode.DISADVANTAGE;
}

/** Whether there is anything to tell about how a check was rolled: a mode other than Normal, two dice, or sources. */
export function hasModeInfo(
  mode: RollMode,
  sources: readonly unknown[],
  faces: readonly CheckFace[],
): boolean {
  return takesTwo(mode) || faces.length > 1 || sources.length > 0;
}
