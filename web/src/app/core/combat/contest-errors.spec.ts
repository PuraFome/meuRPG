import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import {
  EncounterBlockedReason,
  EncounterBlockedSchema,
} from '../../../gen/meurpg/play/v1/combat_pb';
import {
  ContestBlockedReason,
  ContestBlockedSchema,
} from '../../../gen/meurpg/play/v1/contest_types_pb';
import { combatErrorMessage, contestBlocked, encounterBlocked } from './combat-errors';

const contest = (reason: ContestBlockedReason) =>
  new ConnectError('x', Code.FailedPrecondition, undefined, [
    { desc: ContestBlockedSchema, value: create(ContestBlockedSchema, { reason }) },
  ]);

describe('the refusals of the contests (W7-X), by reason', () => {
  it('decodes the ContestBlocked detail next to the EncounterBlocked one', () => {
    const err = contest(ContestBlockedReason.NOT_AWAITING);
    expect(contestBlocked(err)?.reason).toBe(ContestBlockedReason.NOT_AWAITING);
    expect(encounterBlocked(err)).toBeNull();
    const old = new ConnectError('x', Code.FailedPrecondition, undefined, [
      {
        desc: EncounterBlockedSchema,
        value: create(EncounterBlockedSchema, { reason: EncounterBlockedReason.ACTION_USED }),
      },
    ]);
    expect(contestBlocked(old)).toBeNull();
    expect(encounterBlocked(old)?.reason).toBe(EncounterBlockedReason.ACTION_USED);
    expect(contestBlocked(new ConnectError('x', Code.NotFound))).toBeNull();
  });

  it('keeps saying the old reasons the old way', () => {
    const old = new ConnectError('x', Code.FailedPrecondition, undefined, [
      {
        desc: EncounterBlockedSchema,
        value: create(EncounterBlockedSchema, { reason: EncounterBlockedReason.ACTION_USED }),
      },
    ]);
    expect(combatErrorMessage(old)).toBe('Sua ação já foi usada neste turno.');
  });

  it.each([
    [ContestBlockedReason.TARGET_TOO_BIG, 'Grande demais: no máximo um tamanho acima do seu.'],
    [ContestBlockedReason.CONTEST_OPEN, 'Uma disputa sua ainda espera a resposta.'],
    [
      ContestBlockedReason.NO_ROOM_TO_DRAG,
      'Não dá para arrastar por aí: não há casa livre atrás de você para quem você segura.',
    ],
    [ContestBlockedReason.PUSH_BLOCKED, 'Há algo na casa de trás: o empurrão não sai do lugar.'],
    [
      ContestBlockedReason.SURPRISED,
      'Surpresa: não se move, não age e não reage até o fim do turno.',
    ],
    [ContestBlockedReason.NOT_AN_ALLY, 'Só dá para ajudar um aliado que ainda está de pé.'],
    [ContestBlockedReason.ALREADY_ANSWERED, 'Você já rolou esse teste.'],
    [ContestBlockedReason.GROUP_CHECK_CLOSED, 'O mestre já encerrou esse teste.'],
  ])('%s', (reason, text) => {
    expect(combatErrorMessage(contest(reason))).toBe(text);
  });

  it('never says a total, a DC or a name', () => {
    for (const reason of Object.values(ContestBlockedReason)) {
      if (typeof reason === 'number') {
        expect(combatErrorMessage(contest(reason))).not.toMatch(/\d|CD /);
      }
    }
  });
});
