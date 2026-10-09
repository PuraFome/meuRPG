import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import {
  EncounterBlockedReason,
  EncounterBlockedSchema,
} from '../../../gen/meurpg/play/v1/combat_pb';
import {
  GameSessionBlockedReason,
  GameSessionBlockedSchema,
} from '../../../gen/meurpg/play/v1/play_pb';
import {
  ResourceBlockedReason,
  ResourceBlockedSchema,
} from '../../../gen/meurpg/play/v1/resources_pb';
import {
  hitDiceErrorMessage,
  resourceBlocked,
  resourceBlockedMessage,
  restErrorMessage,
} from './resources-errors';

function blocked(reason: ResourceBlockedReason): ConnectError {
  return new ConnectError('blocked', Code.FailedPrecondition, undefined, [
    { desc: ResourceBlockedSchema, value: create(ResourceBlockedSchema, { reason }) },
  ]);
}

const noSession = new ConnectError('no session', Code.FailedPrecondition, undefined, [
  {
    desc: GameSessionBlockedSchema,
    value: create(GameSessionBlockedSchema, { reason: GameSessionBlockedReason.NO_OPEN_SESSION }),
  },
]);

describe('resource errors', () => {
  it('reads the typed detail, and only on a failed precondition', () => {
    expect(resourceBlocked(blocked(ResourceBlockedReason.COMBAT_OPEN))?.reason).toBe(
      ResourceBlockedReason.COMBAT_OPEN,
    );
    expect(resourceBlocked(new ConnectError('x', Code.Internal))).toBeNull();
    expect(resourceBlocked(noSession)).toBeNull();
  });

  it('words the combat in the way of a rest', () => {
    expect(restErrorMessage(blocked(ResourceBlockedReason.COMBAT_OPEN))).toBe(
      'Há um combate em andamento: termine o combate antes de descansar.',
    );
  });

  it('words a session that is over, for the rest and for a hit die', () => {
    expect(restErrorMessage(noSession)).toBe(
      'A sessão acabou: o descanso só vale durante a sessão.',
    );
    expect(hitDiceErrorMessage(noSession)).toContain('A sessão acabou');
  });

  it('words the codes of a rest: the master only, a hit dice choice that is not the sheet', () => {
    expect(restErrorMessage(new ConnectError('x', Code.PermissionDenied))).toContain('Só o mestre');
    expect(restErrorMessage(new ConnectError('x', Code.InvalidArgument))).toContain(
      'dados de vida escolhidos',
    );
    expect(restErrorMessage(new ConnectError('x', Code.Unavailable))).toContain(
      'o servidor não respondeu',
    );
  });

  it('words a hit die of a size the character has none of, and a combat in the way', () => {
    expect(hitDiceErrorMessage(blocked(ResourceBlockedReason.NO_HIT_DICE_LEFT))).toBe(
      'Não há dado de vida desse tipo.',
    );
    expect(hitDiceErrorMessage(blocked(ResourceBlockedReason.COMBAT_OPEN))).toBe(
      'Há um combate em andamento.',
    );
  });

  it("reuses the combat's wording for the wrong way of rolling the die", () => {
    const wrong = new ConnectError('wrong', Code.FailedPrecondition, undefined, [
      {
        desc: EncounterBlockedSchema,
        value: create(EncounterBlockedSchema, {
          reason: EncounterBlockedReason.WRONG_DICE_MODE,
        }),
      },
    ]);
    expect(hitDiceErrorMessage(wrong)).toContain('forma de rolar os dados');
  });

  it('has a sentence for a reason it does not word', () => {
    expect(resourceBlockedMessage(ResourceBlockedReason.UNSPECIFIED)).toContain(
      'não pode ser feito agora',
    );
  });
});
