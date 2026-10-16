import { create } from '@bufbuild/protobuf';

import {
  PuzzleAlphabet,
  PuzzleKind,
  PuzzleLastMoveSchema,
  PuzzleSolveOutcome,
} from '../../../gen/meurpg/play/v1/puzzles_pb';
import {
  agoText,
  alphabetWord,
  clockOf,
  clockSeconds,
  kindIcon,
  kindName,
  lastMoveText,
  lightLabel,
  litCount,
  litWords,
  moveWord,
  outcomeText,
  pillarsChangedText,
  limitRows,
  plural,
  puzzleSummary,
  secondsUntil,
  trapFired,
} from './puzzle-format';
import {
  NOW,
  at,
  cipherPuzzle,
  lightsPuzzle,
  lockPuzzle,
  pillarsPuzzle,
  playerRun,
  riddlePuzzle,
  sequencePuzzle,
} from './puzzles-testing';

const plain = (text: string) => text.replace(/\u00a0/g, ' ');

describe('puzzle words', () => {
  it("names each kind and writes the list row's second line (E10-06 state 1)", () => {
    expect(kindName(PuzzleKind.LOCK)).toBe('Fechadura de combinação');
    expect(plain(puzzleSummary(lightsPuzzle('a', 'O selo da Capela')))).toBe(
      'Apagar as luzes · 5 × 5',
    );
    expect(plain(puzzleSummary(lockPuzzle('b', 'O cofre do Refeitório')))).toBe(
      'Fechadura de combinação · 4 rodas de runas',
    );
    expect(plain(puzzleSummary(pillarsPuzzle('c', 'Os pilares da Galeria')))).toBe(
      'Símbolos giratórios · 4 pilares, girando juntos',
    );
  });

  it('names a light by its row, its column and its state (never colour alone)', () => {
    expect(lightLabel(1, 2, true)).toBe('Luz na linha 2, coluna 3, acesa');
    expect(lightLabel(0, 0, false)).toBe('Luz na linha 1, coluna 1, apagada');
  });

  it('counts what the server sent and writes it', () => {
    const start = lightsPuzzle('a', 'x').start;
    expect(litCount(start)).toBe(9);
    expect(litWords(1)).toBe('1 acesa');
    expect(litWords(12)).toBe('12 acesas');
    expect(moveWord(PuzzleKind.LIGHTS, 1)).toBe('toque');
    expect(moveWord(PuzzleKind.PILLARS, 7)).toBe('giros');
  });

  it('says how long ago, from the two clocks', () => {
    expect(agoText(new Date(NOW.getTime() - 2000), NOW)).toBe('agora há pouco');
    expect(plain(agoText(new Date(NOW.getTime() - 12_000), NOW))).toBe('há 12 s');
    expect(plain(agoText(new Date(NOW.getTime() - 180_000), NOW))).toBe('há 3 min');
    expect(agoText(new Date(NOW.getTime() + 5000), NOW)).toBe('agora há pouco');
    // The edges: 5 s is "há 5 s", 60 s is "há 1 min", an hour is "há 1 h".
    expect(plain(agoText(new Date(NOW.getTime() - 4000), NOW))).toBe('agora há pouco');
    expect(plain(agoText(new Date(NOW.getTime() - 5000), NOW))).toBe('há 5 s');
    expect(plain(agoText(new Date(NOW.getTime() - 59_000), NOW))).toBe('há 59 s');
    expect(plain(agoText(new Date(NOW.getTime() - 60_000), NOW))).toBe('há 1 min');
    expect(plain(agoText(new Date(NOW.getTime() - 3_540_000), NOW))).toBe('há 59 min');
    expect(plain(agoText(new Date(NOW.getTime() - 3_600_000), NOW))).toBe('há 1 h');
  });

  it('names no one as "Você" when neither the move nor the reader has a name', () => {
    const nameless = create(PuzzleLastMoveSchema, {
      characterName: '',
      move: { kind: { case: 'lights', value: { row: 1, col: 1 } } },
    });
    expect(lastMoveText(nameless, '')).not.toContain('Você');
  });

  it('writes the step of a sequence, and that a wrong one starts the attempt again', () => {
    const move = (wrong: boolean) =>
      create(PuzzleLastMoveSchema, {
        characterName: 'Lia',
        step: 3,
        wrong,
        move: { kind: { case: 'sequence', value: {} } },
      });
    expect(lastMoveText(move(false))).toBe('Lia acertou o passo 3');
    expect(lastMoveText(move(true))).toBe('Lia errou no passo 3. A tentativa recomeçou');
  });

  it("writes the second line of a puzzle by what it is made of: the alphabet, the links, the cipher's method", () => {
    const lock = (alphabet: PuzzleAlphabet) =>
      puzzleSummary(
        lockPuzzle('l', 'x', {
          config: { kind: { case: 'lock', value: { wheels: 3, alphabet } } },
        }),
      );
    expect(plain(lock(PuzzleAlphabet.DIGITS))).toBe('Fechadura de combinação · 3 rodas de dígitos');
    expect(plain(lock(PuzzleAlphabet.LETTERS))).toBe('Fechadura de combinação · 3 rodas de letras');
    const loose = pillarsPuzzle('p', 'x', {
      config: {
        kind: {
          case: 'pillars',
          value: { pillars: 2, symbols: 4, links: [{ alsoTurns: [] }, { alsoTurns: [] }] },
        },
      },
    });
    expect(plain(puzzleSummary(loose))).toBe('Símbolos giratórios · 2 pilares, sem ligações');
    expect(plain(puzzleSummary(cipherPuzzle('c', 'x')))).toBe('Cifra · deslocamento');
    const keyword = cipherPuzzle('k', 'x', {
      solution: {
        kind: {
          case: 'cipher',
          value: { message: 'a', method: { case: 'keyword', value: 'chave' } },
        },
      },
    });
    expect(plain(puzzleSummary(keyword))).toBe('Cifra · palavra-chave');
  });

  it('writes the last move of each kind, and "Você" for the reader\'s own character', () => {
    const lights = create(PuzzleLastMoveSchema, {
      characterName: 'Lia',
      move: { kind: { case: 'lights', value: { row: 1, col: 1 } } },
      at: at(12),
    });
    const lock = create(PuzzleLastMoveSchema, {
      characterName: 'Toren',
      move: { kind: { case: 'lock', value: { wheel: 2, delta: 1 } } },
    });
    const pillars = create(PuzzleLastMoveSchema, {
      characterName: 'Lia',
      move: { kind: { case: 'pillars', value: { pillar: 0, delta: 1 } } },
    });
    expect(lastMoveText(lights)).toBe('Lia tocou numa luz');
    expect(lastMoveText(lock)).toBe('Toren girou a 3ª roda');
    expect(lastMoveText(pillars)).toBe('Lia girou o pilar 1');
    expect(lastMoveText(lights, 'Lia')).toBe('Você tocou numa luz');
  });

  it('lists the pillars a move turned', () => {
    expect(pillarsChangedText([])).toBe('');
    expect(pillarsChangedText([0])).toBe('O pilar 1 mudou.');
    expect(pillarsChangedText([0, 1])).toBe('Os pilares 1 e 2 mudaram.');
    expect(pillarsChangedText([0, 1, 3])).toBe('Os pilares 1, 2 e 4 mudaram.');
  });

  it('says what the server did when it was solved', () => {
    expect(outcomeText(PuzzleSolveOutcome.DOOR_OPENED)).toBe('Uma porta se abriu.');
    expect(outcomeText(PuzzleSolveOutcome.TARGET_GONE)).toContain('foi apagado');
    expect(outcomeText(PuzzleSolveOutcome.UNSPECIFIED)).toBe('');
  });
});

describe('the counters of "Ao errar" (slice 10.15b)', () => {
  const limits = {
    attemptsPerPlayer: 0,
    attemptsLeft: 0,
    maxMoves: 10,
    movesMade: 4,
    timeLimitSeconds: 300,
    secondsLeft: 120,
  };
  const run = (partial: object) =>
    playerRun(riddlePuzzle('p', 'x'), { limits: { ...limits, deadline: at(-120) }, ...partial });

  it('runs the clock down from the deadline while the puzzle is open', () => {
    const rows = limitRows(run({}), new Date(NOW.getTime() + 20_000));
    expect(rows.find((r) => r.key === 'time')).toMatchObject({
      value: '1:40 de 5:00',
      spent: false,
    });
  });

  it('stops the clock where the server read it once the puzzle is solved: never a red "acabou"', () => {
    const later = new Date(NOW.getTime() + 600_000);
    expect(limitRows(run({ solved: true }), later).find((r) => r.key === 'time')).toMatchObject({
      value: '2:00 de 5:00',
      spent: false,
    });
    expect(
      limitRows(
        run({
          solved: true,
          limits: { ...limits, movesMade: 10, secondsLeft: 120, deadline: at(-120) },
        }),
        later,
      ).find((r) => r.key === 'moves')?.spent,
    ).toBe(false);
  });

  it('stops it too when a limit stopped the puzzle, and says "acabou" only for what was spent', () => {
    const later = new Date(NOW.getTime() + 600_000);
    const rows = limitRows(
      run({
        stopped: true,
        limits: { ...limits, movesMade: 10, secondsLeft: 120, deadline: at(-120) },
      }),
      later,
    );
    expect(rows.find((r) => r.key === 'time')).toMatchObject({
      value: '2:00 de 5:00',
      spent: false,
    });
    expect(rows.find((r) => r.key === 'moves')).toMatchObject({ value: '10 de 10', spent: true });
  });
});

describe('puzzle words, kind by kind', () => {
  const KINDS: readonly (readonly [PuzzleKind, string, string])[] = [
    [PuzzleKind.LIGHTS, 'Apagar as luzes', 'lightbulb'],
    [PuzzleKind.LOCK, 'Fechadura de combinação', 'key'],
    [PuzzleKind.PILLARS, 'Símbolos giratórios', 'diamond'],
    [PuzzleKind.RIDDLE, 'Enigma', 'help'],
    [PuzzleKind.SEQUENCE, 'Sequência', 'notifications'],
    [PuzzleKind.CIPHER, 'Cifra', 'enhanced_encryption'],
    [PuzzleKind.UNSPECIFIED, 'Quebra-cabeça', 'extension'],
  ];

  it.each(KINDS)('names kind %i and gives it an icon', (kind, name, icon) => {
    expect(kindName(kind)).toBe(name);
    expect(kindIcon(kind)).toBe(icon);
  });

  it('names the alphabet of a lock, runes being the rest', () => {
    expect(alphabetWord(PuzzleAlphabet.DIGITS)).toBe('dígitos');
    expect(alphabetWord(PuzzleAlphabet.LETTERS)).toBe('letras');
    expect(alphabetWord(PuzzleAlphabet.RUNES)).toBe('runas');
  });

  it('counts with the singular for one and the plural for the rest', () => {
    expect(plural(1, 'roda', 'rodas')).toBe('1 roda');
    expect(plural(0, 'roda', 'rodas')).toBe('0 rodas');
    expect(plural(4, 'roda', 'rodas')).toBe('4 rodas');
  });

  it('writes the second line of a riddle, a sequence and a puzzle without a config', () => {
    expect(plain(puzzleSummary(riddlePuzzle('r', 'x')))).toMatch(
      /^Enigma · \d+ respostas? aceitas?$/,
    );
    expect(plain(puzzleSummary(sequencePuzzle('s', 'x')))).toMatch(
      /^Sequência · \d+ passos?, \d+ sinos?$/,
    );
    const bare = lightsPuzzle('b', 'x');
    bare.config = undefined;
    expect(puzzleSummary(bare)).toBe('Apagar as luzes');
    // A riddle whose solution is hidden (a player's read) has nothing to count; the dot stays.
    const hidden = riddlePuzzle('h', 'x');
    hidden.solution = undefined;
    expect(plain(puzzleSummary(hidden))).toBe('Enigma · ');
  });

  it('counts no lit light without a lights board', () => {
    expect(litCount(undefined)).toBe(0);
    expect(litCount(riddlePuzzle('r', 'x').start)).toBe(0);
  });

  it('names the move of the other kinds, and plural or singular of it', () => {
    expect(moveWord(PuzzleKind.LIGHTS, 3)).toBe('toques');
    expect(moveWord(PuzzleKind.PILLARS, 1)).toBe('giro');
    expect(moveWord(PuzzleKind.RIDDLE, 1)).toBe('jogada');
    expect(moveWord(PuzzleKind.CIPHER, 2)).toBe('jogadas');
  });
});

describe('puzzle words, the last move and the clocks', () => {
  const last = (init: Parameters<typeof create<typeof PuzzleLastMoveSchema>>[1]) =>
    create(PuzzleLastMoveSchema, { characterName: 'Lia', ...init });

  it('writes a riddle answer as given or struck, and a cipher text', () => {
    const riddle = (wrong: boolean) =>
      last({ wrong, move: { kind: { case: 'riddle', value: { answer: 'sombra' } } } });
    expect(lastMoveText(riddle(false))).toBe('Lia respondeu “sombra”: acertou');
    expect(lastMoveText(riddle(true))).toBe('Lia tentou “sombra”: errou');
    const cipher = (wrong: boolean) =>
      last({ wrong, move: { kind: { case: 'cipher', value: { text: 'abc' } } } });
    expect(lastMoveText(cipher(false))).toBe('Lia digitou “abc”: acertou');
    expect(lastMoveText(cipher(true))).toBe('Lia digitou “abc”: errou');
  });

  it('says nothing for no move, a move of no known kind, or no move at all', () => {
    expect(lastMoveText(undefined)).toBe('');
    expect(lastMoveText(last({}))).toBe('');
    expect(lastMoveText(last({ move: { kind: { case: undefined } } }))).toBe('');
  });

  it('keeps the name of the trap that fired, empty when none did', () => {
    expect(trapFired(last({ trapName: 'Dardos envenenados' }))).toBe('Dardos envenenados');
    expect(trapFired(last({}))).toBe('');
    expect(trapFired(undefined)).toBe('');
  });

  it('writes a countdown as minutes and seconds, never negative', () => {
    expect(clockSeconds(288)).toBe('4:48');
    expect(clockSeconds(5)).toBe('0:05');
    expect(clockSeconds(60)).toBe('1:00');
    expect(clockSeconds(-3)).toBe('0:00');
    expect(clockSeconds(59.6)).toBe('1:00');
  });

  it('counts the seconds to a deadline up, never below zero', () => {
    expect(secondsUntil(new Date(NOW.getTime() + 1500), NOW)).toBe(2);
    expect(secondsUntil(new Date(NOW.getTime() + 60_000), NOW)).toBe(60);
    expect(secondsUntil(new Date(NOW.getTime() - 9000), NOW)).toBe(0);
  });

  it('writes the clock of a solved puzzle with two digits each', () => {
    expect(clockOf(new Date(2026, 9, 6, 7, 5))).toBe('07:05');
    expect(clockOf(new Date(2026, 9, 6, 21, 12))).toBe('21:12');
  });

  it('says what the server did, outcome by outcome', () => {
    const OUTCOMES: readonly (readonly [PuzzleSolveOutcome, string])[] = [
      [PuzzleSolveOutcome.NOTIFIED, 'Só você foi avisado.'],
      [PuzzleSolveOutcome.DOOR_OPENED, 'Uma porta se abriu.'],
      [PuzzleSolveOutcome.DOOR_NOT_CLOSED, 'A porta já estava aberta, ou foi pintada por cima.'],
      [PuzzleSolveOutcome.POINT_REVEALED, 'Um ponto apareceu no mapa.'],
      [PuzzleSolveOutcome.POINT_ALREADY_REVEALED, 'O ponto já estava à vista.'],
      [PuzzleSolveOutcome.CLUE_REVEALED, 'Uma pista foi para quem resolveu.'],
      [PuzzleSolveOutcome.CLUE_ALREADY_HAD, 'Quem resolveu já tinha a pista.'],
      [
        PuzzleSolveOutcome.TARGET_GONE,
        'O alvo do “Ao resolver” foi apagado: nada aconteceu além do aviso.',
      ],
    ];
    for (const [outcome, text] of OUTCOMES) {
      expect(outcomeText(outcome)).toBe(text);
    }
  });

  it('shows no counters for a run without limits', () => {
    const run = playerRun(lightsPuzzle('a', 'x'));
    run.limits = undefined;
    expect(limitRows(run, NOW)).toEqual([]);
  });
});

describe('the counters of "Ao errar", attempts and a clock with no deadline', () => {
  const limits = {
    attemptsPerPlayer: 3,
    attemptsLeft: 1,
    maxMoves: 0,
    movesMade: 0,
    timeLimitSeconds: 0,
    secondsLeft: 0,
  };
  const run = (partial: Record<string, unknown>) => playerRun(riddlePuzzle('p', 'x'), partial);

  it('shows the attempts left, spent at none and only while the puzzle is unsolved', () => {
    expect(limitRows(run({ limits }), NOW)).toEqual([
      { key: 'attempts', label: 'Tentativas restantes', value: '1 de 3', spent: false },
    ]);
    const none = { ...limits, attemptsLeft: 0 };
    expect(limitRows(run({ limits: none }), NOW)[0].spent).toBe(true);
    expect(limitRows(run({ limits: none, solved: true }), NOW)[0].spent).toBe(false);
  });

  it('shows the moves made out of the limit, spent when they reach it', () => {
    const rows = limitRows(
      run({ limits: { ...limits, attemptsPerPlayer: 0, maxMoves: 5, movesMade: 5 } }),
      NOW,
    );
    expect(rows).toEqual([{ key: 'moves', label: 'Jogadas', value: '5 de 5', spent: true }]);
  });

  it('reads the clock as the server left it when there is no deadline', () => {
    const rows = limitRows(
      run({ limits: { ...limits, attemptsPerPlayer: 0, timeLimitSeconds: 90, secondsLeft: 0 } }),
      NOW,
    );
    expect(rows).toEqual([{ key: 'time', label: 'Tempo', value: '0:00 de 1:30', spent: true }]);
  });
});
