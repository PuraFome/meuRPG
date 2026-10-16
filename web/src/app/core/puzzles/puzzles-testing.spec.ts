import { PuzzleKind, PuzzleRunStatus } from '../../../gen/meurpg/play/v1/puzzles_pb';
import {
  FakePuzzlesClient,
  NOW,
  SKILLS,
  asClient,
  at,
  cipherPuzzle,
  fakeChecks,
  fakeRoster,
  fromNow,
  hintAnswer,
  lightsPuzzle,
  lockPuzzle,
  masterRun,
  pillarsPuzzle,
  playerRun,
  preview,
  riddlePuzzle,
  sequencePuzzle,
  summary,
  symbolsOf,
} from './puzzles-testing';

describe('the puzzle fixtures', () => {
  it('builds each kind with its artboard numbers', () => {
    expect(lightsPuzzle('a', 'x')).toMatchObject({ kind: PuzzleKind.LIGHTS, id: 'a' });
    expect(lightsPuzzle('a', 'x').start?.kind).toMatchObject({
      case: 'lights',
      value: { lit: expect.any(Array) },
    });
    expect(lockPuzzle('b', 'x')).toMatchObject({ kind: PuzzleKind.LOCK });
    expect(pillarsPuzzle('c', 'x')).toMatchObject({ kind: PuzzleKind.PILLARS });
    expect(riddlePuzzle('d', 'x')).toMatchObject({ kind: PuzzleKind.RIDDLE });
    expect(sequencePuzzle('e', 'x')).toMatchObject({ kind: PuzzleKind.SEQUENCE });
    expect(cipherPuzzle('f', 'x')).toMatchObject({ kind: PuzzleKind.CIPHER });
    expect(lockPuzzle('b', 'x', { name: 'outro' }).name).toBe('outro');
  });

  it('lists the symbols of a lock (eight of them) and of the pillars', () => {
    expect(symbolsOf('lock', 4)).toHaveLength(8);
    expect(symbolsOf('pillars', 4)).toHaveLength(4);
  });

  it('builds the run, the master run, the summary and the answers around a puzzle', () => {
    const puzzle = lightsPuzzle('a', 'x');
    expect(playerRun(puzzle)).toMatchObject({ puzzleId: 'a', revision: 1 });
    expect(masterRun(puzzle, PuzzleRunStatus.NOT_SHOWN).run).toBeUndefined();
    expect(masterRun(puzzle, PuzzleRunStatus.SHOWN).run?.puzzleId).toBe('a');
    expect(summary(puzzle)).toMatchObject({ puzzleId: 'a', name: 'x' });
    expect(hintAnswer(playerRun(puzzle), true, 20).roll?.total).toBe(20);
    expect(preview([true, false], 3).minimum?.moves).toBe(3);
  });

  it('keeps the clock of the specs, the skills and the roster', async () => {
    expect(at(10).seconds).toBe(BigInt(Math.floor(NOW.getTime() / 1000) - 10));
    expect(Number(fromNow(60).seconds)).toBeGreaterThan(Date.now() / 1000);
    expect(SKILLS).toHaveLength(2);
    await expect(fakeChecks.skills()).resolves.toBe(SKILLS);
    await expect(fakeRoster.list()).resolves.toHaveLength(4);
  });
});

describe('FakePuzzlesClient', () => {
  it('remembers each call with its arguments and answers what the spec set', async () => {
    const fake = new FakePuzzlesClient();
    const puzzle = lightsPuzzle('p', 'x');
    const run = masterRun(puzzle, PuzzleRunStatus.SHOWN);
    fake.listResult = [puzzle];
    fake.getResult = puzzle;
    fake.createResult = puzzle;
    fake.previewResult = preview([true], 1);
    fake.sessionResult = [run];
    fake.runResults.set('p', run);
    fake.shownResult = [summary(puzzle)];
    fake.playerRunResult = playerRun(puzzle);

    expect(await fake.list('c', true)).toEqual([puzzle]);
    expect(await fake.get('c', 'p')).toBe(puzzle);
    expect(await fake.create('c', {} as never)).toBe(puzzle);
    expect(await fake.update('c', 'p', {} as never)).toBe(puzzle);
    expect(await fake.previewStart('c', {}, {}, 7n)).toBe(fake.previewResult);
    expect((await fake.archive('c', 'p')).archived).toBe(true);
    expect((await fake.unarchive('c', 'p')).id).toBe('p');
    expect(await fake.listSession('c')).toEqual([run]);
    expect(await fake.masterRun('c', 'p')).toBe(run);
    expect(await fake.show('c', 'p')).toBe(run);
    expect(await fake.reset('c', 'p', 3)).toBe(run);
    expect(await fake.reseed('c', 'p', 3)).toBe(run);
    expect(await fake.close('c', 'p')).toBe(run);
    expect(await fake.releaseHint('c', 'p', 3)).toBe(run);
    expect(await fake.listShown('c')).toHaveLength(1);
    expect(await fake.run('c', 'p')).toBe(fake.playerRunResult);
    expect(await fake.previewCipher('c', {})).toBe(fake.cipherResult);
    expect(await fake.playSequence('c', 'p', 3)).toBe(run);
    expect(fake.calls.map((c) => c[0])).toEqual([
      'list',
      'get',
      'create',
      'update',
      'previewStart',
      'archive',
      'unarchive',
      'listSession',
      'masterRun',
      'show',
      'reset',
      'reseed',
      'close',
      'releaseHint',
      'listShown',
      'run',
      'previewCipher',
      'playSequence',
    ]);
    expect(fake.calls[0]).toEqual(['list', 'c', true]);
    expect(fake.calls[10]).toEqual(['reset', 'c', 'p', 3]);
  });

  it('fails every plain call with what the spec set', async () => {
    const fake = new FakePuzzlesClient();
    fake.failWith = new Error('down');
    await expect(fake.list('c')).rejects.toThrow('down');
    await expect(fake.get('c', 'p')).rejects.toThrow('down');
  });

  it('counts the moves and the hint tries with their keys, and refuses one nobody set', async () => {
    const fake = new FakePuzzlesClient();
    await expect(fake.move('c', 'p', {}, 'k0')).rejects.toThrow('no moveResult set');
    await expect(fake.tryHint('c', 'p', { inApp: true }, 'k0')).rejects.toThrow(
      'no hintResult set',
    );
    const answer = {
      run: playerRun(lightsPuzzle('p', 'x')),
      replayed: false,
      solvedByThisMove: false,
      wrong: false,
    };
    fake.moveResult = (n) => ({ ...answer, replayed: n > 1 });
    fake.hintResult = () => hintAnswer(answer.run, true);
    expect((await fake.move('c', 'p', {}, 'k1')).replayed).toBe(true);
    expect((await fake.tryHint('c', 'p', { inApp: true }, 'k2')).passed).toBe(true);
    expect(fake.moveKeys).toEqual(['k0', 'k1']);
    expect(fake.hintKeys).toEqual(['k0', 'k2']);
    expect(asClient(fake)).toBe(fake);
  });
});
