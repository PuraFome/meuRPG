import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import {
  EncounterBlockedReason,
  EncounterBlockedSchema,
} from '../../../gen/meurpg/play/v1/combat_pb';
import {
  ResourceBlockedReason,
  ResourceBlockedSchema,
} from '../../../gen/meurpg/play/v1/resources_pb';
import { classResourceErrorMessage } from './resources-errors';

function blocked(reason: ResourceBlockedReason, needed = 0, available = 0): ConnectError {
  return new ConnectError('blocked', Code.FailedPrecondition, undefined, [
    {
      desc: ResourceBlockedSchema,
      value: create(ResourceBlockedSchema, { reason, needed, available }),
    },
  ]);
}

describe('classResourceErrorMessage', () => {
  it('words the points that are not enough with the numbers of the detail', () => {
    expect(
      classResourceErrorMessage(blocked(ResourceBlockedReason.NOT_ENOUGH_POINTS, 8, 3), 'x'),
    ).toBe('Não há pontos suficientes: precisa de 8, restam 3.');
  });

  it('words the full sorcery points with the maximum (needed) and the refusal of the board', () => {
    expect(
      classResourceErrorMessage(blocked(ResourceBlockedReason.SORCERY_POINTS_FULL, 5, 5), 'x'),
    ).toBe(
      'Você já tem o máximo de pontos de feitiçaria. O máximo é o seu nível (5): converter um espaço agora perderia os pontos.',
    );
  });

  it('words a conversion that would pass the maximum, a slot that is not there and a level too high', () => {
    expect(
      classResourceErrorMessage(blocked(ResourceBlockedReason.SORCERY_POINTS_OVER, 5, 4), 'x'),
    ).toContain('passaria do máximo de pontos de feitiçaria (5): você tem 4');
    expect(classResourceErrorMessage(blocked(ResourceBlockedReason.NO_FREE_SLOT), 'x')).toBe(
      'Você não tem espaço livre desse nível.',
    );
    expect(classResourceErrorMessage(blocked(ResourceBlockedReason.SLOT_LEVEL_TOO_HIGH), 'x')).toBe(
      'A Conjuração Flexível cria espaços de 1º a 5º nível.',
    );
  });

  it('words the bard: no uses left, a target that cannot get the die (no type), a roll waiting for an answer', () => {
    expect(classResourceErrorMessage(blocked(ResourceBlockedReason.NO_USES_LEFT), 'x')).toContain(
      'Sem usos da Inspiração de Bardo',
    );
    const refused = classResourceErrorMessage(blocked(ResourceBlockedReason.TARGET_REFUSED), 'x');
    expect(refused).toBe('Essa criatura não pode receber o dado.');
    expect(
      classResourceErrorMessage(blocked(ResourceBlockedReason.INSPIRATION_PENDING), 'x'),
    ).toContain('Responda primeiro');
  });

  it('keeps the combat words for a refusal of the combat (the bonus action is used)', () => {
    const err = new ConnectError('used', Code.FailedPrecondition, undefined, [
      {
        desc: EncounterBlockedSchema,
        value: create(EncounterBlockedSchema, { reason: EncounterBlockedReason.BONUS_ACTION_USED }),
      },
    ]);
    expect(classResourceErrorMessage(err, 'x')).toBe('Sua ação bônus já foi usada neste turno.');
  });

  it('falls to the shared words of a code', () => {
    expect(
      classResourceErrorMessage(new ConnectError('down', Code.Unavailable), 'dar o dado'),
    ).toBe('Não deu para dar o dado: o servidor não respondeu. Tente de novo.');
  });
});
