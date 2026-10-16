import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import {
  GameSessionBlockedReason,
  GameSessionBlockedSchema,
} from '../../../gen/meurpg/play/v1/play_pb';
import {
  PuzzleBlockedReason,
  PuzzleBlockedSchema,
  PuzzleInvalidReason,
  PuzzleInvalidSchema,
} from '../../../gen/meurpg/play/v1/puzzles_pb';
import {
  invalidSection,
  isTransient,
  puzzleBlocked,
  puzzleBlockedMessage,
  puzzleErrorMessage,
  puzzleInvalid,
  puzzleInvalidMessage,
} from './puzzle-errors';

function blocked(reason: PuzzleBlockedReason): ConnectError {
  return new ConnectError('x', Code.FailedPrecondition, undefined, [
    { desc: PuzzleBlockedSchema, value: create(PuzzleBlockedSchema, { reason }) },
  ]);
}

describe('puzzle errors', () => {
  it('reads the typed detail, never the message', () => {
    expect(puzzleBlocked(blocked(PuzzleBlockedReason.SOLVED))?.reason).toBe(
      PuzzleBlockedReason.SOLVED,
    );
    expect(puzzleBlocked(new ConnectError('SOLVED', Code.FailedPrecondition))).toBeNull();
    // A detail on any other code is not a refusal of this kind.
    const elsewhere = new ConnectError('x', Code.Internal, undefined, [
      {
        desc: PuzzleBlockedSchema,
        value: create(PuzzleBlockedSchema, { reason: PuzzleBlockedReason.SOLVED }),
      },
    ]);
    expect(puzzleBlocked(elsewhere)).toBeNull();
    const invalid = new ConnectError('x', Code.InvalidArgument, undefined, [
      {
        desc: PuzzleInvalidSchema,
        value: create(PuzzleInvalidSchema, { reason: PuzzleInvalidReason.NAME, field: 'name' }),
      },
    ]);
    expect(puzzleInvalid(invalid)?.reason).toBe(PuzzleInvalidReason.NAME);
  });

  it('turns each refusal into words the master or the player can act on', () => {
    expect(puzzleErrorMessage(blocked(PuzzleBlockedReason.SOLVED))).toBe(
      'Este quebra-cabeça já foi resolvido.',
    );
    expect(puzzleErrorMessage(blocked(PuzzleBlockedReason.STOPPED))).toContain(
      'O quebra-cabeça parou',
    );
    expect(puzzleErrorMessage(blocked(PuzzleBlockedReason.ALREADY_SHOWN))).toContain(
      'não pode mais ser editado',
    );
    expect(puzzleErrorMessage(blocked(PuzzleBlockedReason.NO_CHARACTER))).toContain(
      'personagem vivo',
    );
    expect(puzzleErrorMessage(blocked(PuzzleBlockedReason.NO_OPEN_SESSION))).toContain(
      'A sessão acabou',
    );
  });

  it('turns a refused field into the sentence under it', () => {
    const err = new ConnectError('x', Code.InvalidArgument, undefined, [
      {
        desc: PuzzleInvalidSchema,
        value: create(PuzzleInvalidSchema, { reason: PuzzleInvalidReason.START_SOLVED }),
      },
    ]);
    expect(puzzleErrorMessage(err)).toContain('igual à solução');
  });

  it('falls back to the code', () => {
    expect(puzzleErrorMessage(new ConnectError('x', Code.PermissionDenied))).toBe(
      'Você não pode fazer isso agora.',
    );
    expect(puzzleErrorMessage(new ConnectError('x', Code.NotFound))).toContain('não existe mais');
    expect(puzzleErrorMessage(new TypeError('Failed to fetch'), 'fazer essa jogada')).toBe(
      'Não deu para fazer essa jogada: o servidor não respondeu. Tente de novo.',
    );
  });

  it('retries only what never got an answer', () => {
    expect(isTransient(new TypeError('Failed to fetch'))).toBe(true);
    expect(isTransient(new ConnectError('x', Code.Unavailable))).toBe(true);
    expect(isTransient(new ConnectError('x', Code.Aborted))).toBe(true);
    expect(isTransient(blocked(PuzzleBlockedReason.SOLVED))).toBe(false);
    expect(isTransient(new ConnectError('x', Code.InvalidArgument))).toBe(false);
  });

  it('says the sequence and hint refusals in words, to the player and to the master', () => {
    expect(
      puzzleErrorMessage(blocked(PuzzleBlockedReason.SEQUENCE_NOT_PLAYED), 'x', 'player'),
    ).toBe('O mestre ainda não tocou a sequência. Esperem ele tocar.');
    expect(
      puzzleErrorMessage(blocked(PuzzleBlockedReason.SEQUENCE_NOT_PLAYED), 'x', 'master'),
    ).toBe('Toque a sequência para os jogadores antes.');
    expect(puzzleErrorMessage(blocked(PuzzleBlockedReason.SEQUENCE_PLAYING))).toContain(
      'está tocando',
    );
    expect(puzzleErrorMessage(blocked(PuzzleBlockedReason.HINT_ALREADY_TRIED))).toContain(
      'Outro jogador pode tentar',
    );
    expect(puzzleErrorMessage(blocked(PuzzleBlockedReason.WRONG_DICE_MODE))).toContain(
      'de outro jeito',
    );
    expect(puzzleErrorMessage(blocked(PuzzleBlockedReason.NO_SKILL))).toContain(
      'Seu personagem não tem os números',
    );
    expect(puzzleErrorMessage(blocked(PuzzleBlockedReason.NO_MORE_HINTS), 'x', 'player')).toBe(
      'Não há mais dicas para ganhar.',
    );
    expect(puzzleErrorMessage(blocked(PuzzleBlockedReason.NO_MORE_HINTS))).toBe(
      'Não há mais dicas para soltar.',
    );
  });

  it('says what the server refused in the new fields, by the typed detail', () => {
    const invalid = (reason: PuzzleInvalidReason, field = '') =>
      new ConnectError('x', Code.InvalidArgument, undefined, [
        { desc: PuzzleInvalidSchema, value: create(PuzzleInvalidSchema, { reason, field }) },
      ]);
    expect(puzzleErrorMessage(invalid(PuzzleInvalidReason.ANSWERS))).toContain('respostas aceitas');
    expect(puzzleErrorMessage(invalid(PuzzleInvalidReason.CIPHER))).toContain('A cifra não vale');
    expect(puzzleErrorMessage(invalid(PuzzleInvalidReason.PARTS))).toContain('informação dividida');
    expect(puzzleErrorMessage(invalid(PuzzleInvalidReason.ON_WRONG))).toContain('“Ao errar”');
    expect(puzzleErrorMessage(invalid(PuzzleInvalidReason.HINT_CHECK))).toContain('18 perícias');
  });

  it('puts a refusal on the part of the form its field names', () => {
    const at = (field: string) =>
      invalidSection(create(PuzzleInvalidSchema, { reason: PuzzleInvalidReason.TEXT, field }));
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

describe('puzzle refusals, every reason in turn', () => {
  const BLOCKED: readonly (readonly [PuzzleBlockedReason, string])[] = [
    [
      PuzzleBlockedReason.NO_OPEN_SESSION,
      'A sessão acabou: os quebra-cabeças só se jogam durante a sessão.',
    ],
    [PuzzleBlockedReason.SOLVED, 'Este quebra-cabeça já foi resolvido.'],
    [
      PuzzleBlockedReason.ALREADY_SHOWN,
      'Este quebra-cabeça já foi mostrado numa sessão e não pode mais ser editado.',
    ],
    [PuzzleBlockedReason.ARCHIVED, 'Este quebra-cabeça está arquivado. Desarquive-o para mostrar.'],
    [PuzzleBlockedReason.NOT_SHOWN, 'Este quebra-cabeça não está sendo mostrado.'],
    [
      PuzzleBlockedReason.NO_CHARACTER,
      'Você não tem um personagem vivo nesta campanha para jogar.',
    ],
    [PuzzleBlockedReason.SEQUENCE_PLAYING, 'A sequência está tocando. Espere ela terminar.'],
    [PuzzleBlockedReason.NOT_A_SEQUENCE, 'Só uma sequência se toca.'],
    [PuzzleBlockedReason.NO_HINT_CHECK, 'Este quebra-cabeça não dá dicas por teste de perícia.'],
    [
      PuzzleBlockedReason.HINT_ALREADY_TRIED,
      'Você já tentou esta dica. Outro jogador pode tentar, ou o mestre solta uma dica.',
    ],
    [
      PuzzleBlockedReason.WRONG_DICE_MODE,
      'A mesa rola os dados de outro jeito. A tela foi atualizada.',
    ],
    [PuzzleBlockedReason.NO_SKILL, 'Seu personagem não tem os números desta perícia.'],
    [
      PuzzleBlockedReason.NO_GENERATED_START,
      'O começo da fechadura é o que você escolheu: não há outro para gerar.',
    ],
    [
      PuzzleBlockedReason.STOPPED,
      'O quebra-cabeça parou. Ninguém joga mais até o mestre recomeçar ou fechar.',
    ],
    [
      PuzzleBlockedReason.STALE_REVISION,
      'O quebra-cabeça mudou desde que você olhou. Confira como ficou antes de escolher de novo.',
    ],
    [PuzzleBlockedReason.NO_ATTEMPTS_LEFT, 'Você não tem mais tentativas nesta rodada.'],
    [PuzzleBlockedReason.NO_MORE_HINTS, 'Não há mais dicas para soltar.'],
    [PuzzleBlockedReason.SEQUENCE_NOT_PLAYED, 'Toque a sequência para os jogadores antes.'],
  ];
  const INVALID: readonly (readonly [PuzzleInvalidReason, string])[] = [
    [PuzzleInvalidReason.NAME, 'O nome precisa ter de 1 a 80 letras.'],
    [
      PuzzleInvalidReason.TEXT,
      'Um dos textos passa do limite: a pista tem até 500 letras, cada dica até 300 e a mensagem até 200.',
    ],
    [
      PuzzleInvalidReason.SIZE,
      'O painel vai de 3 a 7 de lado, a fechadura de 2 a 6 rodas e os símbolos de 3 a 6 pilares e de 3 a 6 símbolos.',
    ],
    [
      PuzzleInvalidReason.SYMBOLS,
      'Um símbolo ou um sino não existe. Confira a quantidade e os passos da sequência.',
    ],
    [
      PuzzleInvalidReason.LINKS,
      'As ligações entre os pilares não valem: um pilar não gira junto consigo mesmo.',
    ],
    [
      PuzzleInvalidReason.START_SOLVED,
      'O começo é igual à solução. Gire uma roda para o começo ser outro.',
    ],
    [
      PuzzleInvalidReason.UNSOLVABLE,
      'Com essas ligações, os pilares não chegam ao mural. Mude o mural ou as ligações.',
    ],
    [
      PuzzleInvalidReason.TARGET,
      'O alvo do “Ao resolver” não é desta campanha, ou não existe mais. Escolha de novo.',
    ],
    [
      PuzzleInvalidReason.HINT_CHECK,
      'O teste de perícia da dica não vale: escolha uma das 18 perícias, uma CD de 1 a 30, e escreva pelo menos uma dica.',
    ],
    [
      PuzzleInvalidReason.ANSWERS,
      'As respostas aceitas não valem: de 1 a 10, cada uma de 1 a 80 letras, e nenhuma igual a outra.',
    ],
    [
      PuzzleInvalidReason.CIPHER,
      'A cifra não vale: a mensagem tem de 1 a 300 letras com pelo menos uma de A a Z, e a chave troca letras (a pista da chave tem de ser desta campanha).',
    ],
    [
      PuzzleInvalidReason.PARTS,
      'A informação dividida não vale: até 8 partes de 1 a 300 letras, cada uma para um personagem vivo, e nenhum personagem com duas.',
    ],
    [
      PuzzleInvalidReason.ON_WRONG,
      'O “Ao errar” não vale: a armadilha precisa ser de um mapa da campanha, e as tentativas e a armadilha só servem a enigma, sequência e cifra.',
    ],
    [
      PuzzleInvalidReason.MOVE,
      'Essa jogada não vale: escreva de 1 a 600 letras, ou toque num sino que existe.',
    ],
    [
      PuzzleInvalidReason.KIND,
      'Algo no quebra-cabeça não é do tipo escolhido. Volte e tente de novo.',
    ],
  ];

  it.each(BLOCKED)('says blocked reason %i to the master', (reason, message) => {
    expect(puzzleBlockedMessage(reason)).toBe(message);
    expect(puzzleBlockedMessage(reason, 'master')).toBe(message);
  });

  it('says the two reasons that differ to the player in his own words', () => {
    expect(puzzleBlockedMessage(PuzzleBlockedReason.NO_MORE_HINTS, 'player')).toBe(
      'Não há mais dicas para ganhar.',
    );
    expect(puzzleBlockedMessage(PuzzleBlockedReason.SEQUENCE_NOT_PLAYED, 'player')).toBe(
      'O mestre ainda não tocou a sequência. Esperem ele tocar.',
    );
    // The rest reads the same to both.
    expect(puzzleBlockedMessage(PuzzleBlockedReason.SOLVED, 'player')).toBe(
      puzzleBlockedMessage(PuzzleBlockedReason.SOLVED, 'master'),
    );
  });

  it('has the words of every blocked reason the server can name', () => {
    const named: PuzzleBlockedReason[] = BLOCKED.map(([reason]) => reason);
    const all = Object.values(PuzzleBlockedReason).filter(
      (v): v is PuzzleBlockedReason =>
        typeof v === 'number' && v !== PuzzleBlockedReason.UNSPECIFIED,
    );
    expect(all.filter((v) => !named.includes(v))).toEqual([]);
  });

  it('falls back to a general sentence for a blocked reason it does not know', () => {
    const fallback = 'O quebra-cabeça não está num estado que aceite isso. A tela foi atualizada.';
    expect(puzzleBlockedMessage(PuzzleBlockedReason.UNSPECIFIED)).toBe(fallback);
    expect(puzzleBlockedMessage(9999 as PuzzleBlockedReason, 'player')).toBe(fallback);
  });

  it.each(INVALID)('says invalid reason %i under the field', (reason, message) => {
    expect(puzzleInvalidMessage(create(PuzzleInvalidSchema, { reason }))).toBe(message);
  });

  it('has the words of every invalid reason the server can name', () => {
    const named: PuzzleInvalidReason[] = INVALID.map(([reason]) => reason);
    const all = Object.values(PuzzleInvalidReason).filter(
      (v): v is PuzzleInvalidReason =>
        typeof v === 'number' && v !== PuzzleInvalidReason.UNSPECIFIED,
    );
    expect(all.filter((v) => !named.includes(v))).toEqual([]);
  });

  it('falls back to a general sentence for an invalid reason it does not know', () => {
    const fallback = 'Confira os campos e tente de novo.';
    expect(
      puzzleInvalidMessage(
        create(PuzzleInvalidSchema, { reason: PuzzleInvalidReason.UNSPECIFIED }),
      ),
    ).toBe(fallback);
    expect(
      puzzleInvalidMessage(create(PuzzleInvalidSchema, { reason: 9999 as PuzzleInvalidReason })),
    ).toBe(fallback);
  });

  it('reads a closed session sent as a game-session refusal like the puzzle one', () => {
    const err = new ConnectError('x', Code.FailedPrecondition, undefined, [
      {
        desc: GameSessionBlockedSchema,
        value: create(GameSessionBlockedSchema, {
          reason: GameSessionBlockedReason.NO_OPEN_SESSION,
        }),
      },
    ]);
    expect(puzzleErrorMessage(err, 'x', 'player')).toBe(
      'A sessão acabou: os quebra-cabeças só se jogam durante a sessão.',
    );
    // Another game-session reason is not that refusal: the code decides.
    const other = new ConnectError('x', Code.FailedPrecondition, undefined, [
      {
        desc: GameSessionBlockedSchema,
        value: create(GameSessionBlockedSchema, { reason: GameSessionBlockedReason.UNSPECIFIED }),
      },
    ]);
    expect(puzzleErrorMessage(other)).toBe(
      'Não foi possível falar com o servidor agora. Tente de novo em instantes.',
    );
  });

  it('says the codes of the fallback, with what the call was doing', () => {
    expect(puzzleErrorMessage(new ConnectError('x', Code.InvalidArgument), 'salvar')).toBe(
      'Não deu para salvar: confira os campos e tente de novo.',
    );
    expect(puzzleErrorMessage(new ConnectError('x', Code.Aborted), 'salvar')).toBe(
      'Não deu para salvar: o servidor estava ocupado. Tente de novo.',
    );
  });

  it('puts every field of a refusal on a section of the form', () => {
    const at = (field: string) =>
      invalidSection(create(PuzzleInvalidSchema, { reason: PuzzleInvalidReason.TEXT, field }));
    expect(at('clue')).toBe('clue');
    expect(at('hint_check')).toBe('check');
    expect(at('hints[1]')).toBe('hints');
    expect(at('on_solve.target')).toBe('target');
    expect(at('')).toBe('');
  });
});
