import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import {
  PuzzleBlockedReason,
  PuzzleBlockedSchema,
  PuzzleInvalidReason,
  PuzzleInvalidSchema,
} from '../../../gen/meurpg/play/v1/puzzles_pb';
import { isTransient, puzzleBlocked, puzzleErrorMessage, puzzleInvalid } from './puzzle-errors';

function blocked(reason: PuzzleBlockedReason): ConnectError {
  return new ConnectError('x', Code.FailedPrecondition, undefined, [{ desc: PuzzleBlockedSchema, value: create(PuzzleBlockedSchema, { reason }) }]);
}

describe('puzzle errors', () => {
  it('reads the typed detail, never the message', () => {
    expect(puzzleBlocked(blocked(PuzzleBlockedReason.SOLVED))?.reason).toBe(PuzzleBlockedReason.SOLVED);
    expect(puzzleBlocked(new ConnectError('SOLVED', Code.FailedPrecondition))).toBeNull();
    const invalid = new ConnectError('x', Code.InvalidArgument, undefined, [
      { desc: PuzzleInvalidSchema, value: create(PuzzleInvalidSchema, { reason: PuzzleInvalidReason.NAME, field: 'name' }) },
    ]);
    expect(puzzleInvalid(invalid)?.reason).toBe(PuzzleInvalidReason.NAME);
  });

  it('turns each refusal into words the master or the player can act on', () => {
    expect(puzzleErrorMessage(blocked(PuzzleBlockedReason.SOLVED))).toBe('Este quebra-cabeça já foi resolvido.');
    expect(puzzleErrorMessage(blocked(PuzzleBlockedReason.STOPPED))).toContain('O quebra-cabeça parou');
    expect(puzzleErrorMessage(blocked(PuzzleBlockedReason.ALREADY_SHOWN))).toContain('não pode mais ser editado');
    expect(puzzleErrorMessage(blocked(PuzzleBlockedReason.NO_CHARACTER))).toContain('personagem vivo');
    expect(puzzleErrorMessage(blocked(PuzzleBlockedReason.NO_OPEN_SESSION))).toContain('A sessão acabou');
  });

  it('turns a refused field into the sentence under it', () => {
    const err = new ConnectError('x', Code.InvalidArgument, undefined, [
      { desc: PuzzleInvalidSchema, value: create(PuzzleInvalidSchema, { reason: PuzzleInvalidReason.START_SOLVED }) },
    ]);
    expect(puzzleErrorMessage(err)).toContain('igual à solução');
  });

  it('falls back to the code', () => {
    expect(puzzleErrorMessage(new ConnectError('x', Code.PermissionDenied))).toBe('Você não pode fazer isso agora.');
    expect(puzzleErrorMessage(new ConnectError('x', Code.NotFound))).toContain('não existe mais');
    expect(puzzleErrorMessage(new TypeError('Failed to fetch'), 'fazer essa jogada')).toBe('Não deu para fazer essa jogada: o servidor não respondeu. Tente de novo.');
  });

  it('retries only what never got an answer', () => {
    expect(isTransient(new TypeError('Failed to fetch'))).toBe(true);
    expect(isTransient(new ConnectError('x', Code.Unavailable))).toBe(true);
    expect(isTransient(new ConnectError('x', Code.Aborted))).toBe(true);
    expect(isTransient(blocked(PuzzleBlockedReason.SOLVED))).toBe(false);
    expect(isTransient(new ConnectError('x', Code.InvalidArgument))).toBe(false);
  });
});
