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
import { lightsPuzzle, lockPuzzle, pillarsPuzzle } from './puzzles-testing';

describe('the puzzle form\'s draft', () => {
  it('carries the 10.7b fields of a puzzle through an edit untouched (UpdatePuzzle replaces the whole puzzle)', () => {
    const puzzle = lockPuzzle('p', 'O cofre', {
      hintCheck: { skillKey: 'skill:investigation', dc: 14 },
      parts: [{ characterId: 'c1', text: 'A primeira metade.', ownerUnavailable: false }],
      onWrong: { attemptsPerPlayer: 3, maxMoves: 20, timeLimitSeconds: 600 },
    });
    const init = toInit(draftOf(puzzle)!);
    expect(init.hintCheck).toMatchObject({ skillKey: 'skill:investigation', dc: 14 });
    expect(init.parts).toHaveLength(1);
    expect((init.parts?.[0] as { characterId: string }).characterId).toBe('c1');
    expect(init.onWrong).toMatchObject({ attemptsPerPlayer: 3, maxMoves: 20, timeLimitSeconds: 600 });
    // A puzzle without them sends none.
    const plain = toInit(draftOf(lockPuzzle('q', 'x'))!);
    expect(plain.hintCheck).toBeUndefined();
    expect(plain.parts).toEqual([]);
    expect(plain.onWrong).toBeUndefined();
  });

  it('asks for the lights with only the size and the seed on screen', () => {
    const init = toInit({ ...newDraft('lights'), name: ' O selo da Capela ', size: 5, seed: 7n, clue: ' pista ', hints: ['  a ', ''] });
    expect(init.name).toBe('O selo da Capela');
    expect(init.clue).toBe('pista');
    expect(init.hints).toEqual(['a']);
    expect(init.seed).toBe(7n);
    expect(init.config).toEqual({ kind: { case: 'lights', value: { size: 5 } } });
    expect(init.solution).toBeUndefined();
  });

  it('asks for the lock with the solution and the start the master chose, and never a seed', () => {
    const init = toInit({ ...newDraft('lock'), name: 'x', wheels: 3, alphabet: PuzzleAlphabet.DIGITS, lockSolution: [1, 2, 3], lockStart: [0, 0, 1], seed: 9n });
    expect(init.config).toEqual({ kind: { case: 'lock', value: { wheels: 3, alphabet: PuzzleAlphabet.DIGITS } } });
    expect(init.solution).toEqual({ kind: { case: 'lock', value: { wheels: [1, 2, 3] } } });
    expect(init.start).toEqual({ kind: { case: 'lock', value: { wheels: [0, 0, 1] } } });
    expect(init.seed).toBe(0n);
  });

  it('asks for the pillars with the mural and, when linked, each pillar turning its neighbours', () => {
    const init = toInit({ ...newDraft('pillars'), name: 'x', pillars: 4, symbols: 4, mural: [1, 0, 3, 2], linked: true, seed: 5n });
    expect(init.solution).toEqual({ kind: { case: 'pillars', value: { pillars: [1, 0, 3, 2] } } });
    expect(init.config?.kind?.case).toBe('pillars');
    expect(neighbourLinks(4, true)).toEqual([{ alsoTurns: [1] }, { alsoTurns: [0, 2] }, { alsoTurns: [1, 3] }, { alsoTurns: [2] }]);
    expect(neighbourLinks(3, false)).toEqual([{ alsoTurns: [] }, { alsoTurns: [] }, { alsoTurns: [] }]);
    expect(startRequestOf(newDraft('lock'))).toBeNull();
  });

  it('writes "Ao resolver" with its target', () => {
    expect(onSolveOf({ ...newDraft('lights').solve })).toEqual({ action: PuzzleSolveAction.NOTIFY, message: '' });
    expect(onSolveOf({ choice: 'door', mapId: 'm', col: 4, row: 2, pointId: '', clueId: '', message: ' Ela se abriu. ' })).toEqual({
      action: PuzzleSolveAction.OPEN_DOOR,
      message: 'Ela se abriu.',
      target: { case: 'door', value: { mapId: 'm', col: 4, row: 2 } },
    });
    expect(onSolveOf({ choice: 'point', mapId: 'm', col: -1, row: -1, pointId: 'p', clueId: '', message: '' }).target).toEqual({ case: 'point', value: { mapId: 'm', pointId: 'p' } });
    expect(onSolveOf({ choice: 'clue', mapId: '', col: -1, row: -1, pointId: '', clueId: 'c', message: '' }).target).toEqual({ case: 'clue', value: { clueId: 'c' } });
  });

  it('opens an existing puzzle of each kind back into its draft', () => {
    const lights = draftOf(lightsPuzzle('a', 'O selo'));
    expect(lights).toMatchObject({ kind: 'lights', size: 5, name: 'O selo' });
    const lock = draftOf(lockPuzzle('b', 'O cofre'));
    expect(lock).toMatchObject({ kind: 'lock', wheels: 4, lockSolution: [1, 0, 3, 5], lockStart: [0, 0, 2, 5] });
    const pillars = draftOf(pillarsPuzzle('c', 'Os pilares'));
    expect(pillars).toMatchObject({ kind: 'pillars', pillars: 4, symbols: 4, linked: true, mural: [1, 0, 3, 2] });
  });

  it('keeps a puzzle\'s door target when it is opened', () => {
    const puzzle = lightsPuzzle('a', 'x', { onSolve: { action: PuzzleSolveAction.OPEN_DOOR, message: 'Abriu.', target: { case: 'door', value: { mapId: 'm', col: 3, row: 1 } } } });
    expect(draftOf(puzzle)?.solve).toMatchObject({ choice: 'door', mapId: 'm', col: 3, row: 1, message: 'Abriu.' });
  });

  it('resizes and clamps the wheels\' positions', () => {
    expect(resized([1, 2], 4, 1)).toEqual([1, 2, 1, 1]);
    expect(resized([1, 2, 3], 2)).toEqual([1, 2]);
    expect(clamped([9, 25], 8)).toEqual([1, 1]);
  });

  describe('what the form says is wrong, by field', () => {
    it('asks for a name and cuts nothing silently', () => {
      expect(draftErrors(newDraft('lights')).name).toBe('Dê um nome ao quebra-cabeça.');
      expect(draftErrors({ ...newDraft('lights'), name: 'x'.repeat(81) }).name).toContain('passa de 80');
      expect(isValid(draftErrors({ ...newDraft('lights'), name: 'ok' }))).toBe(true);
    });

    it('checks each hint, the clue and the message by their limits', () => {
      const errors = draftErrors({ ...newDraft('lights'), name: 'ok', hints: ['bom', '  ', 'y'.repeat(301)], clue: 'c'.repeat(501), solve: { ...newDraft('lights').solve, message: 'm'.repeat(201) } });
      expect(errors.hints[0]).toBeUndefined();
      expect(errors.hints[1]).toBe('Escreva a dica, ou tire esta.');
      expect(errors.hints[2]).toContain('passa de 300');
      expect(errors.clue).toContain('passa de 500');
      expect(errors.message).toContain('passa de 200');
    });

    it('asks for the target an action needs', () => {
      const base = { ...newDraft('lights'), name: 'ok' };
      expect(draftErrors({ ...base, solve: { ...base.solve, choice: 'door' } }).target).toBe('Escolha a porta que se abre.');
      expect(draftErrors({ ...base, solve: { ...base.solve, choice: 'point', mapId: 'm' } }).target).toBe('Escolha o ponto que aparece no mapa.');
      expect(draftErrors({ ...base, solve: { ...base.solve, choice: 'clue' } }).target).toBe('Escolha a pista que vai a quem resolver.');
      expect(draftErrors({ ...base, solve: { ...base.solve, choice: 'door', mapId: 'm', col: 1, row: 1 } }).target).toBeUndefined();
    });

    it('says when a lock starts solved', () => {
      expect(draftErrors({ ...newDraft('lock'), name: 'ok', lockSolution: [1, 1, 1, 1], lockStart: [1, 1, 1, 1] }).lock).toContain('igual à solução');
      expect(draftErrors({ ...newDraft('lock'), name: 'ok' }).lock).toBeUndefined();
    });
  });
});
