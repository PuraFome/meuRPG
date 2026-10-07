import { PuzzleAlphabet, PuzzleSolveAction } from '../../../gen/meurpg/play/v1/puzzles_pb';
import {
  clamped,
  draftErrors,
  draftOf,
  isValid,
  neighbourLinks,
  newDraft,
  onSolveOf,
  resized,
  startRequestOf,
  toInit,
} from './puzzle-draft';
import {
  cipherPuzzle,
  lightsPuzzle,
  lockPuzzle,
  pillarsPuzzle,
  riddlePuzzle,
  sequencePuzzle,
} from './puzzles-testing';

describe("the puzzle form's draft", () => {
  it('carries the 10.7b fields of a puzzle through an edit untouched (UpdatePuzzle replaces the whole puzzle)', () => {
    const puzzle = lockPuzzle('p', 'O cofre', {
      hintCheck: { skillKey: 'skill:investigation', dc: 14 },
      parts: [{ characterId: 'c1', text: 'A primeira metade.', ownerUnavailable: false }],
      onWrong: { maxMoves: 20, timeLimitSeconds: 600 },
    });
    const init = toInit(draftOf(puzzle)!);
    expect(init.hintCheck).toMatchObject({ skillKey: 'skill:investigation', dc: 14 });
    expect(init.parts).toHaveLength(1);
    expect((init.parts?.[0] as { characterId: string }).characterId).toBe('c1');
    expect(init.onWrong).toMatchObject({ maxMoves: 20, timeLimitSeconds: 600 });
    // A puzzle without them sends none.
    const plain = toInit(draftOf(lockPuzzle('q', 'x'))!);
    expect(plain.hintCheck).toBeUndefined();
    expect(plain.parts).toEqual([]);
    expect(plain.onWrong).toBeUndefined();
  });

  it('keeps a limit of time that is not a whole number of minutes when the minutes are not touched', () => {
    const puzzle = riddlePuzzle('p', 'x', { onWrong: { timeLimitSeconds: 90 } });
    const draft = draftOf(puzzle)!;
    expect(draft.wrong.minutesText).toBe('2');
    expect(toInit(draft).onWrong).toMatchObject({ timeLimitSeconds: 90 });
    expect(toInit({ ...draft, wrong: { ...draft.wrong, minutesText: '5' } }).onWrong).toMatchObject(
      { timeLimitSeconds: 300 },
    );
  });

  it('asks for the lights with only the size and the seed on screen', () => {
    const init = toInit({
      ...newDraft('lights'),
      name: ' O selo da Capela ',
      size: 5,
      seed: 7n,
      clue: ' pista ',
      hints: ['  a ', ''],
    });
    expect(init.name).toBe('O selo da Capela');
    expect(init.clue).toBe('pista');
    expect(init.hints).toEqual(['a']);
    expect(init.seed).toBe(7n);
    expect(init.config).toEqual({ kind: { case: 'lights', value: { size: 5 } } });
    expect(init.solution).toBeUndefined();
  });

  it('asks for the lock with the solution and the start the master chose, and never a seed', () => {
    const init = toInit({
      ...newDraft('lock'),
      name: 'x',
      wheels: 3,
      alphabet: PuzzleAlphabet.DIGITS,
      lockSolution: [1, 2, 3],
      lockStart: [0, 0, 1],
      seed: 9n,
    });
    expect(init.config).toEqual({
      kind: { case: 'lock', value: { wheels: 3, alphabet: PuzzleAlphabet.DIGITS } },
    });
    expect(init.solution).toEqual({ kind: { case: 'lock', value: { wheels: [1, 2, 3] } } });
    expect(init.start).toEqual({ kind: { case: 'lock', value: { wheels: [0, 0, 1] } } });
    expect(init.seed).toBe(0n);
  });

  it('asks for the pillars with the mural and, when linked, each pillar turning its neighbours', () => {
    const init = toInit({
      ...newDraft('pillars'),
      name: 'x',
      pillars: 4,
      symbols: 4,
      mural: [1, 0, 3, 2],
      linked: true,
      seed: 5n,
    });
    expect(init.solution).toEqual({ kind: { case: 'pillars', value: { pillars: [1, 0, 3, 2] } } });
    expect(init.config?.kind?.case).toBe('pillars');
    expect(neighbourLinks(4, true)).toEqual([
      { alsoTurns: [1] },
      { alsoTurns: [0, 2] },
      { alsoTurns: [1, 3] },
      { alsoTurns: [2] },
    ]);
    expect(neighbourLinks(3, false)).toEqual([
      { alsoTurns: [] },
      { alsoTurns: [] },
      { alsoTurns: [] },
    ]);
    expect(startRequestOf(newDraft('lock'))).toBeNull();
  });

  it('writes "Ao resolver" with its target', () => {
    expect(onSolveOf({ ...newDraft('lights').solve })).toEqual({
      action: PuzzleSolveAction.NOTIFY,
      message: '',
    });
    expect(
      onSolveOf({
        choice: 'door',
        mapId: 'm',
        col: 4,
        row: 2,
        pointId: '',
        clueId: '',
        message: ' Ela se abriu. ',
      }),
    ).toEqual({
      action: PuzzleSolveAction.OPEN_DOOR,
      message: 'Ela se abriu.',
      target: { case: 'door', value: { mapId: 'm', col: 4, row: 2 } },
    });
    expect(
      onSolveOf({
        choice: 'point',
        mapId: 'm',
        col: -1,
        row: -1,
        pointId: 'p',
        clueId: '',
        message: '',
      }).target,
    ).toEqual({ case: 'point', value: { mapId: 'm', pointId: 'p' } });
    expect(
      onSolveOf({
        choice: 'clue',
        mapId: '',
        col: -1,
        row: -1,
        pointId: '',
        clueId: 'c',
        message: '',
      }).target,
    ).toEqual({ case: 'clue', value: { clueId: 'c' } });
  });

  it('opens an existing puzzle of each kind back into its draft', () => {
    const lights = draftOf(lightsPuzzle('a', 'O selo'));
    expect(lights).toMatchObject({ kind: 'lights', size: 5, name: 'O selo' });
    const lock = draftOf(lockPuzzle('b', 'O cofre'));
    expect(lock).toMatchObject({
      kind: 'lock',
      wheels: 4,
      lockSolution: [1, 0, 3, 5],
      lockStart: [0, 0, 2, 5],
    });
    const pillars = draftOf(pillarsPuzzle('c', 'Os pilares'));
    expect(pillars).toMatchObject({
      kind: 'pillars',
      pillars: 4,
      symbols: 4,
      linked: true,
      mural: [1, 0, 3, 2],
    });
  });

  it("keeps a puzzle's door target when it is opened", () => {
    const puzzle = lightsPuzzle('a', 'x', {
      onSolve: {
        action: PuzzleSolveAction.OPEN_DOOR,
        message: 'Abriu.',
        target: { case: 'door', value: { mapId: 'm', col: 3, row: 1 } },
      },
    });
    expect(draftOf(puzzle)?.solve).toMatchObject({
      choice: 'door',
      mapId: 'm',
      col: 3,
      row: 1,
      message: 'Abriu.',
    });
  });

  it("resizes and clamps the wheels' positions", () => {
    expect(resized([1, 2], 4, 1)).toEqual([1, 2, 1, 1]);
    expect(resized([1, 2, 3], 2)).toEqual([1, 2]);
    expect(clamped([9, 25], 8)).toEqual([1, 1]);
  });

  describe('what the form says is wrong, by field', () => {
    it('asks for a name and cuts nothing silently', () => {
      expect(draftErrors(newDraft('lights')).name).toBe('Dê um nome ao quebra-cabeça.');
      expect(draftErrors({ ...newDraft('lights'), name: 'x'.repeat(81) }).name).toContain(
        'passa de 80',
      );
      expect(isValid(draftErrors({ ...newDraft('lights'), name: 'ok' }))).toBe(true);
    });

    it('checks each hint, the clue and the message by their limits', () => {
      const errors = draftErrors({
        ...newDraft('lights'),
        name: 'ok',
        hints: ['bom', '  ', 'y'.repeat(301)],
        clue: 'c'.repeat(501),
        solve: { ...newDraft('lights').solve, message: 'm'.repeat(201) },
      });
      expect(errors.hints[0]).toBeUndefined();
      expect(errors.hints[1]).toBe('Escreva a dica, ou tire esta.');
      expect(errors.hints[2]).toContain('passa de 300');
      expect(errors.clue).toContain('passa de 500');
      expect(errors.message).toContain('passa de 200');
    });

    it('asks for the target an action needs', () => {
      const base = { ...newDraft('lights'), name: 'ok' };
      expect(draftErrors({ ...base, solve: { ...base.solve, choice: 'door' } }).target).toBe(
        'Escolha a porta que se abre.',
      );
      expect(
        draftErrors({ ...base, solve: { ...base.solve, choice: 'point', mapId: 'm' } }).target,
      ).toBe('Escolha o ponto que aparece no mapa.');
      expect(draftErrors({ ...base, solve: { ...base.solve, choice: 'clue' } }).target).toBe(
        'Escolha a pista que vai a quem resolver.',
      );
      expect(
        draftErrors({
          ...base,
          solve: { ...base.solve, choice: 'door', mapId: 'm', col: 1, row: 1 },
        }).target,
      ).toBeUndefined();
    });

    it('says when a lock starts solved', () => {
      expect(
        draftErrors({
          ...newDraft('lock'),
          name: 'ok',
          lockSolution: [1, 1, 1, 1],
          lockStart: [1, 1, 1, 1],
        }).lock,
      ).toContain('igual à solução');
      expect(draftErrors({ ...newDraft('lock'), name: 'ok' }).lock).toBeUndefined();
    });
  });

  describe('the riddle, the sequence and the cipher (slice 10.15b)', () => {
    it('asks for the riddle with its text and answers, never a seed', () => {
      const init = toInit({
        ...newDraft('riddle'),
        name: 'A porta da Cripta pergunta',
        riddleText: ' O que sou? ',
        answers: [' sombra ', 'a sombra'],
        seed: 9n,
      });
      expect(init.config).toEqual({ kind: { case: 'riddle', value: { text: 'O que sou?' } } });
      expect(init.solution).toEqual({
        kind: { case: 'riddle', value: { answers: ['sombra', 'a sombra'] } },
      });
      expect(init.seed).toBe(0n);
    });

    it('asks for the sequence with its bells and the steps, the number of steps being their count', () => {
      const init = toInit({
        ...newDraft('sequence'),
        name: 'Os sinos',
        bells: 4,
        steps: [0, 1, 3, 0, 2, 1],
      });
      expect(init.config).toEqual({ kind: { case: 'sequence', value: { bells: 4, steps: 6 } } });
      expect(init.solution).toEqual({
        kind: { case: 'sequence', value: { steps: [0, 1, 3, 0, 2, 1] } },
      });
    });

    it('asks for the cipher with a shift or with a keyword, and the scene clue of the key', () => {
      const shifted = toInit({
        ...newDraft('cipher'),
        name: 'A carta',
        cipherMessage: ' O tesouro está sob o altar ',
        cipherMethod: 'shift',
        shift: 3,
        keyClueId: 'clue-1',
      });
      expect(shifted.config).toEqual({ kind: { case: 'cipher', value: { keyClueId: 'clue-1' } } });
      expect(shifted.solution).toEqual({
        kind: {
          case: 'cipher',
          value: { message: 'O tesouro está sob o altar', method: { case: 'shift', value: 3 } },
        },
      });
      const keyed = toInit({
        ...newDraft('cipher'),
        name: 'A carta',
        cipherMessage: 'x',
        cipherMethod: 'keyword',
        keyword: ' lua ',
      });
      expect(keyed.solution).toEqual({
        kind: {
          case: 'cipher',
          value: { message: 'x', method: { case: 'keyword', value: 'lua' } },
        },
      });
    });

    it('carries each new kind through an edit: the draft of a puzzle asks for the same puzzle', () => {
      for (const puzzle of [
        riddlePuzzle('r', 'x'),
        sequencePuzzle('s', 'y'),
        cipherPuzzle('c', 'z'),
      ]) {
        const draft = draftOf(puzzle)!;
        const init = toInit(draft);
        expect(init.config?.kind?.case).toBe(puzzle.config?.kind.case);
        expect(init.solution).toEqual(
          expect.objectContaining({
            kind: expect.objectContaining({ case: puzzle.solution?.kind.case }),
          }),
        );
      }
      const cipher = draftOf(
        cipherPuzzle('c', 'z', {
          solution: {
            kind: {
              case: 'cipher',
              value: { message: 'Olá mundo', method: { case: 'keyword', value: 'LUA' } },
            },
          },
        }),
      )!;
      expect(cipher.cipherMethod).toBe('keyword');
      expect(cipher.keyword).toBe('LUA');
      expect(draftOf(sequencePuzzle('s', 'y'))!.steps).toEqual([0, 1, 3, 0, 2, 1]);
      expect(draftOf(riddlePuzzle('r', 'x'))!.answers).toEqual(['sombra', 'a sombra']);
    });

    it('refuses a riddle without a text, without answers, with an answer that repeats another or has no letter', () => {
      const errors = draftErrors({
        ...newDraft('riddle'),
        name: 'x',
        riddleText: '  ',
        answers: [],
      });
      expect(errors.riddle).toContain('Escreva o enigma');
      expect(errors.answers).toBe('Escreva pelo menos uma resposta aceita.');
      const again = draftErrors({
        ...newDraft('riddle'),
        name: 'x',
        riddleText: 'O que sou?',
        answers: ['A Sombra', '!!!', 'a sombra', 'x'.repeat(81)],
      });
      expect(again.riddle).toBeUndefined();
      expect(again.answerRows[1]).toContain('nenhuma letra');
      expect(again.answerRows[2]).toContain('igual a outra');
      expect(again.answerRows[3]).toContain('passa de 80');
      expect(isValid(again)).toBe(false);
    });

    it('refuses a sequence with fewer than 3 steps or with a single bell', () => {
      expect(draftErrors({ ...newDraft('sequence'), name: 'x', steps: [0, 1] }).sequence).toContain(
        'Faltam passos',
      );
      expect(
        draftErrors({ ...newDraft('sequence'), name: 'x', steps: [2, 2, 2] }).sequence,
      ).toContain('dois sinos diferentes');
      expect(
        draftErrors({ ...newDraft('sequence'), name: 'x', steps: [0, 1, 2] }).sequence,
      ).toBeUndefined();
    });

    it('refuses a cipher with no letter to swap, and a keyword with too few letters or that swaps nothing', () => {
      expect(
        draftErrors({ ...newDraft('cipher'), name: 'x', cipherMessage: '12 34' }).cipherMessage,
      ).toContain('pelo menos uma letra');
      expect(
        draftErrors({ ...newDraft('cipher'), name: 'x', cipherMessage: '' }).cipherMessage,
      ).toContain('Escreva a mensagem');
      const few = draftErrors({
        ...newDraft('cipher'),
        name: 'x',
        cipherMessage: 'ok',
        cipherMethod: 'keyword',
        keyword: 'aa',
      });
      expect(few.cipherKey).toContain('3 a 26 letras diferentes');
      const none = draftErrors({
        ...newDraft('cipher'),
        name: 'x',
        cipherMessage: 'ok',
        cipherMethod: 'keyword',
        keyword: 'abc',
      });
      expect(none.cipherKey).toContain('não troca nenhuma letra');
      expect(
        draftErrors({
          ...newDraft('cipher'),
          name: 'x',
          cipherMessage: 'ok',
          cipherMethod: 'keyword',
          keyword: 'lua',
        }).cipherKey,
      ).toBeUndefined();
    });
  });

  describe('the skill check, the parts and "Ao errar" of every kind', () => {
    const lights = (partial: object) => ({ ...newDraft('lights'), name: 'x', ...partial });

    it('asks for a skill and a DC, both or neither', () => {
      expect(
        toInit(
          lights({ hintCheck: { skillKey: 'skill:investigation', dcText: '13' }, hints: ['uma'] }),
        ).hintCheck,
      ).toEqual({ skillKey: 'skill:investigation', dc: 13 });
      expect(toInit(lights({})).hintCheck).toBeUndefined();
      expect(draftErrors(lights({ hintCheck: { skillKey: '', dcText: '13' } })).check).toContain(
        'Escolha a perícia',
      );
      expect(
        draftErrors(lights({ hintCheck: { skillKey: 'skill:arcana', dcText: '' }, hints: ['a'] }))
          .check,
      ).toContain('A CD vai de 1 a 30');
      expect(
        draftErrors(lights({ hintCheck: { skillKey: 'skill:arcana', dcText: '31' }, hints: ['a'] }))
          .check,
      ).toContain('A CD vai de 1 a 30');
      expect(
        draftErrors(lights({ hintCheck: { skillKey: 'skill:arcana', dcText: '12' }, hints: ['a'] }))
          .check,
      ).toBeUndefined();
    });

    it('needs a hint to win when there is a skill check', () => {
      expect(
        draftErrors(lights({ hintCheck: { skillKey: 'skill:arcana', dcText: '12' }, hints: [] }))
          .check,
      ).toContain('escreva pelo menos uma dica');
    });

    it('asks for each part with its character, and refuses an empty text or a character twice', () => {
      const init = toInit(
        lights({
          parts: [
            { characterId: 'c1', text: ' A porta ouve. ', ownerUnavailable: false },
            { characterId: '', text: 'Sem dono', ownerUnavailable: false },
          ],
        }),
      );
      expect(init.parts).toEqual([
        { characterId: 'c1', text: 'A porta ouve.' },
        { characterId: '', text: 'Sem dono' },
      ]);
      const errors = draftErrors(
        lights({
          parts: [
            { characterId: 'c1', text: '', ownerUnavailable: false },
            { characterId: 'c1', text: 'x', ownerUnavailable: false },
          ],
        }),
      );
      expect(errors.parts[0].text).toContain('Escreva a parte');
      expect(errors.parts[1].owner).toContain('uma parte só');
    });

    it('writes "Ao errar" for the chosen option only: nothing, a trap, attempts, or the limits', () => {
      expect(toInit(lights({})).onWrong).toBeUndefined();
      const base = { ...newDraft('riddle'), name: 'x' };
      expect(
        toInit({
          ...base,
          wrong: {
            ...NO_WRONG_FOR_TEST,
            option: 'trap',
            mapId: 'm1',
            pointId: 'p1',
            attempts: 5,
            movesText: '9',
          },
        }).onWrong,
      ).toEqual({ trap: { mapId: 'm1', pointId: 'p1' } });
      expect(
        toInit({
          ...base,
          wrong: { ...NO_WRONG_FOR_TEST, option: 'attempts', attempts: 3, mapId: 'm1' },
        }).onWrong,
      ).toEqual({ attemptsPerPlayer: 3 });
      expect(
        toInit({
          ...base,
          wrong: { ...NO_WRONG_FOR_TEST, option: 'limits', movesText: '10', minutesText: '5' },
        }).onWrong,
      ).toEqual({ maxMoves: 10, timeLimitSeconds: 300 });
      expect(
        toInit({
          ...base,
          wrong: { ...NO_WRONG_FOR_TEST, option: 'limits', movesText: '', minutesText: '5' },
        }).onWrong,
      ).toEqual({ maxMoves: 0, timeLimitSeconds: 300 });
    });

    it('refuses a trap not chosen, and limits with none, out of range or not whole', () => {
      const base = { ...newDraft('sequence'), name: 'x', steps: [0, 1, 2] };
      expect(
        draftErrors({ ...base, wrong: { ...NO_WRONG_FOR_TEST, option: 'trap' } }).wrong,
      ).toContain('Escolha a armadilha');
      expect(
        draftErrors({ ...base, wrong: { ...NO_WRONG_FOR_TEST, option: 'limits' } }).wrong,
      ).toContain('Ponha um limite');
      expect(
        draftErrors({
          ...base,
          wrong: { ...NO_WRONG_FOR_TEST, option: 'limits', movesText: '201' },
        }).wrong,
      ).toContain('1 a 200');
      expect(
        draftErrors({
          ...base,
          wrong: { ...NO_WRONG_FOR_TEST, option: 'limits', minutesText: '241' },
        }).wrong,
      ).toContain('1 a 240 minutos');
      expect(
        draftErrors({
          ...base,
          wrong: { ...NO_WRONG_FOR_TEST, option: 'limits', movesText: '2,5' },
        }).wrong,
      ).toContain('1 a 200');
      expect(
        draftErrors({
          ...base,
          wrong: { ...NO_WRONG_FOR_TEST, option: 'limits', movesText: '10', minutesText: '5' },
        }).wrong,
      ).toBeUndefined();
    });

    it('reads "Ao errar" of a puzzle back as one option, and says so when the puzzle came with more than one rule', () => {
      const trapped = draftOf(
        riddlePuzzle('p', 'x', { onWrong: { trap: { mapId: 'm1', pointId: 'p1' }, maxMoves: 10 } }),
      )!;
      expect(trapped.wrong).toMatchObject({
        option: 'trap',
        mapId: 'm1',
        pointId: 'p1',
        combined: true,
      });
      const attempts = draftOf(riddlePuzzle('p', 'x', { onWrong: { attemptsPerPlayer: 4 } }))!;
      expect(attempts.wrong).toMatchObject({ option: 'attempts', attempts: 4, combined: false });
      const limits = draftOf(lightsPuzzle('p', 'x', { onWrong: { maxMoves: 12 } }))!;
      expect(limits.wrong).toMatchObject({
        option: 'limits',
        movesText: '12',
        minutesText: '',
        combined: false,
      });
    });
  });
});

const NO_WRONG_FOR_TEST = {
  option: 'none' as const,
  mapId: '',
  pointId: '',
  attempts: 3,
  movesText: '',
  minutesText: '',
  keptSeconds: 0,
  combined: false,
};
