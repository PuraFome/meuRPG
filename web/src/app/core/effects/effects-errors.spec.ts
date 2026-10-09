import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import {
  EncounterBlockedReason,
  EncounterBlockedSchema,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { effectsErrorMessage } from './effects-errors';

const err = (code: Code) => new ConnectError('x', code);

describe('effectsErrorMessage', () => {
  it('words each code the effects calls use', () => {
    expect(effectsErrorMessage(err(Code.InvalidArgument), 'adicionar o efeito', 'combat')).toBe(
      'Não deu para adicionar o efeito: confira os campos e tente de novo.',
    );
    expect(effectsErrorMessage(err(Code.NotFound), 'x', 'combat')).toContain('não existe mais');
    expect(effectsErrorMessage(err(Code.NotFound), 'x', 'character')).toContain('personagem');
    expect(effectsErrorMessage(err(Code.PermissionDenied), 'x', 'combat')).toBe(
      'Só o mestre faz isso.',
    );
    expect(effectsErrorMessage(err(Code.Aborted), 'x', 'combat')).toContain('mudaram');
  });

  it('says the level of exhaustion changed when expected_level is stale', () => {
    expect(effectsErrorMessage(err(Code.Aborted), 'mudar a exaustão', 'exhaustion')).toContain(
      'nível de exaustão mudou',
    );
  });

  it('reads a refusal of the encounter as the combat does', () => {
    const blocked = new ConnectError('b', Code.FailedPrecondition, undefined, [
      {
        desc: EncounterBlockedSchema,
        value: create(EncounterBlockedSchema, { reason: EncounterBlockedReason.NOT_ACTIVE }),
      },
    ]);
    expect(effectsErrorMessage(blocked, 'x', 'combat')).toBe('O combate não está em andamento.');
  });

  it('keeps the words every call shares for a lost session or a lost answer', () => {
    expect(effectsErrorMessage(err(Code.Unauthenticated), 'x', 'combat')).toContain(
      'Entre de novo',
    );
    expect(effectsErrorMessage(err(Code.Unknown), 'x', 'combat')).toContain('confirmar');
    expect(effectsErrorMessage(err(Code.Unavailable), 'passar o tempo', 'character')).toBe(
      'Não deu para passar o tempo: o servidor não respondeu. Tente de novo.',
    );
  });
});
