import { Code, ConnectError } from '@connectrpc/connect';

import { type ReadiedAction, ReadiedKind } from '../../../gen/meurpg/play/v1/combat_pb';
import { combatErrorMessage } from './combat-errors';

/**
 * The words of a refused answer of a readied action. A held spell is released with `AnswerReaction`, which carries the targets with
 * their darts (Magic Missile, Scorching Ray) or the creatures, and the d20. The server refuses with `invalid_argument` a choice that
 * does not fit the spell (a dart not placed, a creature that cannot be aimed at, too many targets), and the usual words ("confira os
 * campos") would send the player looking for a field. So the sentence points at what the window asks: the targets and the darts.
 */
export function readiedRefusalText(
  err: unknown,
  readied: ReadiedAction | undefined,
  doing: string,
): string {
  if (
    readied?.kind === ReadiedKind.SPELL &&
    err instanceof ConnectError &&
    err.code === Code.InvalidArgument
  ) {
    return 'Não deu para soltar a magia com essa escolha. Confira os alvos e os dardos. Dá para deixar passar.';
  }
  return combatErrorMessage(err, doing);
}
