import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import {
  GameSessionBlockedReason,
  GameSessionBlockedSchema,
} from '../../../gen/meurpg/play/v1/play_pb';
import {
  ResourceBlockedReason,
  ResourceBlockedSchema,
} from '../../../gen/meurpg/play/v1/resources_pb';
import { cutErrorOf, rollErrorOf, skipErrorText } from './npc-check-errors';

const failed = (code: Code, detail?: ConstructorParameters<typeof ConnectError>[3]) =>
  new ConnectError('x', code, undefined, detail);
const blocked = (reason: ResourceBlockedReason) =>
  failed(Code.FailedPrecondition, [
    { desc: ResourceBlockedSchema, value: create(ResourceBlockedSchema, { reason }) },
  ]);

describe('rollErrorOf', () => {
  it('says a combat began and the check was not rolled, and marks the combat', () => {
    expect(rollErrorOf(blocked(ResourceBlockedReason.COMBAT_OPEN), 'Capitão bandido')).toEqual({
      lead: 'Um combate começou.',
      text: 'O teste do Capitão bandido não foi rolado: os testes agora são do combate.',
      combat: true,
    });
  });

  it('says only an open session takes the roll', () => {
    const err = failed(Code.FailedPrecondition, [
      {
        desc: GameSessionBlockedSchema,
        value: create(GameSessionBlockedSchema, { reason: GameSessionBlockedReason.NO_OPEN_SESSION }),
      },
    ]);
    expect(rollErrorOf(err, 'Espião')).toEqual({
      lead: 'Só numa sessão aberta.',
      text: 'Nada foi rolado.',
      combat: false,
    });
  });

  it('says nothing was rolled for any other failure', () => {
    expect(rollErrorOf(failed(Code.Unavailable), 'Espião')).toEqual({
      lead: 'Não deu para rolar agora.',
      text: 'Nada foi rolado. Tente de novo.',
      combat: false,
    });
  });
});

describe('skipErrorText', () => {
  it('says the master can try again', () => {
    expect(skipErrorText(failed(Code.Unavailable))).toBe(
      'Não deu para continuar sem esperar agora. Tente de novo.',
    );
  });
});

describe('cutErrorOf', () => {
  it('reads a roll answered already as a race lost and spends nothing', () => {
    expect(cutErrorOf(failed(Code.FailedPrecondition), true)).toEqual({
      lead: 'Essa pergunta já foi respondida.',
      text: 'Nada foi gasto.',
      closed: true,
    });
  });

  it('says the bard cannot cut now when the server refuses the cut for the bard', () => {
    expect(cutErrorOf(blocked(ResourceBlockedReason.NOT_AVAILABLE), true)).toMatchObject({
      lead: 'Não dá para cortar agora.',
      text: 'Nada foi gasto. Você ainda pode deixar passar.',
      closed: false,
    });
    expect(cutErrorOf(blocked(ResourceBlockedReason.NO_USES_LEFT), true).lead).toBe(
      'Não dá para cortar agora.',
    );
  });

  it('keeps the sheet open to try again on any other failure', () => {
    expect(cutErrorOf(failed(Code.Unavailable), true)).toEqual({
      lead: 'Não deu para cortar agora.',
      text: 'Nada foi gasto. Tente de novo.',
      closed: false,
    });
    expect(cutErrorOf(failed(Code.Unavailable), false)).toEqual({
      lead: 'Não deu para deixar passar agora.',
      text: 'Tente de novo.',
      closed: false,
    });
  });

  it('reads a question that is gone, closed or passed by the server as closed, for a cut and for a pass', () => {
    for (const cut of [true, false]) {
      expect(cutErrorOf(failed(Code.NotFound), cut)).toEqual({
        lead: 'A pergunta já foi encerrada.',
        text: 'Nada foi gasto.',
        closed: true,
      });
    }
  });
});
