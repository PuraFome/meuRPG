import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import { PuzzleBlockedReason, PuzzleBlockedSchema, type PuzzleRun } from '../../../gen/meurpg/play/v1/puzzles_pb';
import { PuzzlePlay, RETRY_WAITS_MS } from './puzzle-play';
import { type FakePuzzlesClient as Fake, FakePuzzlesClient, lightsPuzzle, playerRun } from './puzzles-testing';

const puzzle = lightsPuzzle('p1', 'O selo da Capela');
const press = { kind: { case: 'lights' as const, value: { row: 1, col: 1 } } };

function setup(): { fake: Fake; play: PuzzlePlay; waits: number[] } {
  const fake = new FakePuzzlesClient();
  const waits: number[] = [];
  let key = 0;
  const play = new PuzzlePlay(
    fake,
    () => 'camp-1',
    async (ms) => void waits.push(ms),
    () => `key-${++key}`,
  );
  return { fake, play, waits };
}

const run = (revision: number, partial: Partial<PuzzleRun> = {}): PuzzleRun => ({ ...playerRun(puzzle), revision, ...partial });

describe('PuzzlePlay (MR-038, RN-27)', () => {
  it('opens a puzzle with the run the server sends', async () => {
    const { fake, play } = setup();
    fake.playerRunResult = run(1);
    await play.open('p1');
    expect(play.run()?.puzzleId).toBe('p1');
    expect(fake.calls[0]).toEqual(['run', 'camp-1', 'p1']);
  });

  it('applies an answer only when its revision is larger: a late answer never puts the board back', async () => {
    const { fake, play } = setup();
    fake.playerRunResult = run(5);
    await play.open('p1');
    expect(play.apply(run(4))).toBe(false);
    expect(play.apply(run(5))).toBe(false);
    expect(play.run()?.revision).toBe(5);
    expect(play.apply(run(6))).toBe(true);
    expect(play.run()?.revision).toBe(6);
  });

  it('makes a move with its own key and follows the server\'s answer, not its own guess', async () => {
    const { fake, play } = setup();
    fake.playerRunResult = run(1);
    await play.open('p1');
    fake.moveResult = () => ({ run: run(2, { name: 'do servidor' }), replayed: false, solvedByThisMove: false });
    await play.move(press);
    expect(fake.moveKeys).toEqual(['key-1']);
    expect(play.run()?.revision).toBe(2);
    expect(play.run()?.name).toBe('do servidor');
    await play.move(press);
    expect(fake.moveKeys).toEqual(['key-1', 'key-2']);
  });

  it('sends a move again with the SAME key when the answer never came, and waits longer each time', async () => {
    const { fake, play, waits } = setup();
    fake.playerRunResult = run(1);
    await play.open('p1');
    fake.moveResult = (n) => {
      if (n < 3) {
        throw new ConnectError('down', Code.Unavailable);
      }
      return { run: run(2), replayed: true, solvedByThisMove: false };
    };
    await play.move(press);
    expect(fake.moveKeys).toEqual(['key-1', 'key-1', 'key-1']);
    expect(waits).toEqual([RETRY_WAITS_MS[0], RETRY_WAITS_MS[1]]);
    expect(play.run()?.revision).toBe(2);
    expect(play.message()).toBe('');
  });

  it('gives up after the last retry and says so in words', async () => {
    const { fake, play } = setup();
    fake.playerRunResult = run(1);
    await play.open('p1');
    fake.moveResult = () => {
      throw new ConnectError('down', Code.Unavailable);
    };
    await play.move(press);
    expect(fake.moveKeys.length).toBe(1 + RETRY_WAITS_MS.length);
    expect(new Set(fake.moveKeys).size).toBe(1);
    expect(play.message()).toContain('o servidor não respondeu');
    expect(play.pending()).toBe(0);
  });

  it('does not retry a refusal, says why and reads the run again', async () => {
    const { fake, play } = setup();
    fake.playerRunResult = run(1);
    await play.open('p1');
    fake.playerRunResult = run(3, { solved: true });
    fake.moveResult = () => {
      throw new ConnectError('x', Code.FailedPrecondition, undefined, [{ desc: PuzzleBlockedSchema, value: create(PuzzleBlockedSchema, { reason: PuzzleBlockedReason.SOLVED }) }]);
    };
    await play.move(press);
    expect(fake.moveKeys.length).toBe(1);
    expect(play.message()).toBe('Este quebra-cabeça já foi resolvido.');
    expect(play.run()?.solved).toBe(true);
  });

  it('knows the master closed it (not_found) and leaves the board as it was', async () => {
    const { fake, play } = setup();
    fake.playerRunResult = run(1);
    await play.open('p1');
    fake.failWith = new ConnectError('x', Code.NotFound);
    await play.refresh();
    expect(play.gone()).toBe(true);
    expect(play.run()?.revision).toBe(1);
  });

  it('moves nothing in a frozen puzzle, and ignores a run of another puzzle', async () => {
    const { fake, play } = setup();
    fake.playerRunResult = run(1);
    await play.open('p1');
    fake.moveResult = () => ({ run: run(2, { solved: true }), replayed: false, solvedByThisMove: true });
    await play.move(press);
    await play.move(press);
    expect(fake.moveKeys.length).toBe(1);
  });

  it('ignores a late answer of the puzzle that was open before, and loads the new one', async () => {
    const { fake, play } = setup();
    fake.playerRunResult = run(1);
    await play.open('p1');
    fake.playerRunResult = { ...run(1), puzzleId: 'p2', name: 'B' };
    await play.open('p2');
    expect(play.apply(run(9))).toBe(false); // a run of p1 now
    expect(play.run()?.puzzleId).toBe('p2');
  });

  it('counts the moves in flight, two taps in a row go out together', async () => {
    const { fake, play } = setup();
    fake.playerRunResult = run(1);
    await play.open('p1');
    const open: ((v: { run: PuzzleRun; replayed: boolean; solvedByThisMove: boolean }) => void)[] = [];
    fake.moveResult = () => new Promise((resolve) => open.push(resolve));
    const a = play.move(press);
    const b = play.move(press);
    expect(play.pending()).toBe(2);
    open.forEach((resolve, i) => resolve({ run: run(2 + i), replayed: false, solvedByThisMove: false }));
    await Promise.all([a, b]);
    expect(play.pending()).toBe(0);
    expect(play.run()?.revision).toBe(3);
  });
});
