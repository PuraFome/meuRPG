import { create } from '@bufbuild/protobuf';

import {
  PuzzleKind,
  PuzzleLastMoveSchema,
  PuzzleSolveOutcome,
} from '../../../gen/meurpg/play/v1/puzzles_pb';
import {
  agoText,
  kindName,
  lastMoveText,
  lightLabel,
  litCount,
  litWords,
  moveWord,
  outcomeText,
  pillarsChangedText,
  limitRows,
  puzzleSummary,
} from './puzzle-format';
import {
  NOW,
  at,
  lightsPuzzle,
  lockPuzzle,
  pillarsPuzzle,
  playerRun,
  riddlePuzzle,
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
