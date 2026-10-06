import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import {
  PuzzleBlockedReason,
  PuzzleBlockedSchema,
  PuzzleInvalidReason,
  PuzzleInvalidSchema,
} from '../../../gen/meurpg/play/v1/puzzles_pb';
import { invalidSection, isTransient, puzzleBlocked, puzzleErrorMessage, puzzleInvalid } from './puzzle-errors';

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

  it('says the sequence and hint refusals in words, to the player and to the master', () => {
    expect(puzzleErrorMessage(blocked(PuzzleBlockedReason.SEQUENCE_NOT_PLAYED), 'x', 'player')).toBe('O mestre ainda não tocou a sequência. Esperem ele tocar.');
    expect(puzzleErrorMessage(blocked(PuzzleBlockedReason.SEQUENCE_NOT_PLAYED), 'x', 'master')).toBe('Toque a sequência para os jogadores antes.');
    expect(puzzleErrorMessage(blocked(PuzzleBlockedReason.SEQUENCE_PLAYING))).toContain('está tocando');
    expect(puzzleErrorMessage(blocked(PuzzleBlockedReason.HINT_ALREADY_TRIED))).toContain('Outro jogador pode tentar');
    expect(puzzleErrorMessage(blocked(PuzzleBlockedReason.WRONG_DICE_MODE))).toContain('de outro jeito');
    expect(puzzleErrorMessage(blocked(PuzzleBlockedReason.NO_SKILL))).toContain('Seu personagem não tem os números');
    expect(puzzleErrorMessage(blocked(PuzzleBlockedReason.NO_MORE_HINTS), 'x', 'player')).toBe('Não há mais dicas para ganhar.');
    expect(puzzleErrorMessage(blocked(PuzzleBlockedReason.NO_MORE_HINTS))).toBe('Não há mais dicas para soltar.');
  });

  it('says what the server refused in the new fields, by the typed detail', () => {
    const invalid = (reason: PuzzleInvalidReason, field = '') =>
      new ConnectError('x', Code.InvalidArgument, undefined, [{ desc: PuzzleInvalidSchema, value: create(PuzzleInvalidSchema, { reason, field }) }]);
    expect(puzzleErrorMessage(invalid(PuzzleInvalidReason.ANSWERS))).toContain('respostas aceitas');
    expect(puzzleErrorMessage(invalid(PuzzleInvalidReason.CIPHER))).toContain('A cifra não vale');
    expect(puzzleErrorMessage(invalid(PuzzleInvalidReason.PARTS))).toContain('informação dividida');
    expect(puzzleErrorMessage(invalid(PuzzleInvalidReason.ON_WRONG))).toContain('“Ao errar”');
    expect(puzzleErrorMessage(invalid(PuzzleInvalidReason.HINT_CHECK))).toContain('18 perícias');
  });

  it('puts a refusal on the part of the form its field names', () => {
    const at = (field: string) => invalidSection(create(PuzzleInvalidSchema, { reason: PuzzleInvalidReason.TEXT, field }));
    expect(at('name')).toBe('name');
    expect(at('hint_check.dc')).toBe('check');
    expect(at('hints')).toBe('hints');
    expect(at('config.riddle.text')).toBe('riddle');
    expect(at('solution.riddle.answers')).toBe('answers');
    expect(at('solution.sequence.steps')).toBe('sequence');
    expect(at('config.sequence.bells')).toBe('sequence');
    expect(at('solution.cipher')).toBe('cipherMessage');
    expect(at('config.cipher.key_clue_id')).toBe('cipherKey');
    expect(at('parts[2].character_id')).toBe('parts');
    expect(at('on_wrong.trap')).toBe('wrong');
    expect(at('on_wrong.max_moves')).toBe('wrong');
    expect(at('on_solve.message')).toBe('message');
    expect(at('on_solve.door')).toBe('target');
    expect(at('config.lights.size')).toBe('');
  });
});
