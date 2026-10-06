import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import { PuzzleBlockedReason, PuzzleBlockedSchema, type PuzzleRun } from '../../../gen/meurpg/play/v1/puzzles_pb';
import { PuzzlePlay, RETRY_WAITS_MS } from './puzzle-play';
import { type FakePuzzlesClient as Fake, FakePuzzlesClient, at, cipherPuzzle, hintAnswer, lightsPuzzle, playerRun, riddlePuzzle, sequencePuzzle } from './puzzles-testing';

const puzzle = lightsPuzzle('p1', 'O selo da Capela');
const press = { kind: { case: 'lights' as const, value: { row: 1, col: 1 } } };

function setup(own = ''): { fake: Fake; play: PuzzlePlay; waits: number[]; timers: { ms: number; fn: () => void; cancelled: boolean }[] } {
  const fake = new FakePuzzlesClient();
  const waits: number[] = [];
  const timers: { ms: number; fn: () => void; cancelled: boolean }[] = [];
  let key = 0;
  const play = new PuzzlePlay(
    fake,
    () => 'camp-1',
    async (ms) => void waits.push(ms),
    () => `key-${++key}`,
    () => own,
    // A clock the spec drives by hand: no real sleeps.
    (ms, fn) => {
      const timer = { ms, fn, cancelled: false };
      timers.push(timer);
      return () => (timer.cancelled = true);
    },
  );
  return { fake, play, waits, timers };
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

  describe('the riddle and the cipher (a typed answer is judged, not applied)', () => {
    const riddle = riddlePuzzle('p1', 'A porta da Cripta pergunta');
    const answer = { kind: { case: 'riddle' as const, value: { answer: 'escuridão' } } };
    const riddleRun = (revision: number, partial: Partial<PuzzleRun> = {}): PuzzleRun => ({ ...playerRun(riddle), revision, ...partial });

    it('says the answer was wrong when the server\'s last move is the player\'s own wrong one', async () => {
      const { fake, play } = setup('Toren');
      fake.playerRunResult = riddleRun(1);
      await play.open('p1');
      fake.moveResult = () => ({ run: riddleRun(2, { lastMove: { characterName: 'Toren', wrong: true, changed: [], at: at(0) } as never }), replayed: false, solvedByThisMove: false });
      const verdict = await play.move(answer);
      expect(verdict).toEqual({ sent: true, wrong: true, solved: false });
      expect(fake.calls.find((c) => c[0] === 'move')![3]).toEqual(answer);
    });

    it('does not take another player\'s wrong answer for its own', async () => {
      const { fake, play } = setup('Toren');
      fake.playerRunResult = riddleRun(1);
      await play.open('p1');
      fake.moveResult = () => ({ run: riddleRun(2, { lastMove: { characterName: 'Lia', wrong: true, changed: [], at: at(0) } as never }), replayed: false, solvedByThisMove: false });
      expect((await play.move(answer)).wrong).toBe(false);
    });

    it('says solved when the move solved it, and never wrong', async () => {
      const { fake, play } = setup('Toren');
      fake.playerRunResult = riddleRun(1);
      await play.open('p1');
      fake.moveResult = () => ({ run: riddleRun(2, { solved: true }), replayed: false, solvedByThisMove: true });
      expect(await play.move(answer)).toEqual({ sent: true, wrong: false, solved: true });
    });

    it('sends nothing in a stopped puzzle and says why in words when the server refuses for attempts', async () => {
      const { fake, play } = setup('Toren');
      fake.playerRunResult = riddleRun(1);
      await play.open('p1');
      fake.moveResult = () => {
        throw new ConnectError('x', Code.FailedPrecondition, undefined, [{ desc: PuzzleBlockedSchema, value: create(PuzzleBlockedSchema, { reason: PuzzleBlockedReason.NO_ATTEMPTS_LEFT }) }]);
      };
      expect((await play.move(answer)).sent).toBe(false);
      expect(play.message()).toBe('Você não tem mais tentativas nesta rodada.');
    });

    it('sends the cipher\'s message as it was typed, with a key of its own', async () => {
      const { fake, play } = setup('Toren');
      fake.playerRunResult = { ...playerRun(cipherPuzzle('p1', 'A carta')), revision: 1 };
      await play.open('p1');
      fake.moveResult = () => ({ run: { ...playerRun(cipherPuzzle('p1', 'A carta')), revision: 2 }, replayed: false, solvedByThisMove: false });
      await play.move({ kind: { case: 'cipher', value: { text: 'o tesouro esta sobre o altar' } } });
      expect(fake.calls.find((c) => c[0] === 'move')![3]).toEqual({ kind: { case: 'cipher', value: { text: 'o tesouro esta sobre o altar' } } });
      expect(fake.moveKeys).toEqual(['key-1']);
    });
  });

  describe('"Tentar uma dica" (a skill check, RN-18)', () => {
    const base = playerRun(riddlePuzzle('p1', 'A porta'), { hintByCheck: true, hintSkillKey: 'skill:investigation', canTryHint: true, hints: [], sharedHints: 0 });

    it('tries with the d20 rolled in the app, with its own key, and keeps the hint the player won', async () => {
      const { fake, play } = setup();
      fake.playerRunResult = { ...base, revision: 1 };
      await play.open('p1');
      fake.hintResult = () => hintAnswer({ ...base, revision: 2, hints: ['Pense no que acompanha você ao meio-dia.'], sharedHints: 0 }, true, 17);
      await play.tryHint({ inApp: true });
      expect(fake.calls.find((c) => c[0] === 'tryHint')!.slice(1, 4)).toEqual(['camp-1', 'p1', { inApp: true }]);
      expect(fake.hintKeys).toEqual(['key-1']);
      expect(play.run()?.hints).toEqual(['Pense no que acompanha você ao meio-dia.']);
      expect(play.hintTry()).toMatchObject({ passed: true });
      expect(play.hintTry()?.roll?.total).toBe(17);
    });

    it('tries with the face of a real die and says it failed, without any number to beat', async () => {
      const { fake, play } = setup();
      fake.playerRunResult = { ...base, revision: 1 };
      await play.open('p1');
      fake.hintResult = () => hintAnswer({ ...base, revision: 2, canTryHint: false }, false, 9);
      await play.tryHint({ face: 6 });
      expect(fake.calls.find((c) => c[0] === 'tryHint')![3]).toEqual({ face: 6 });
      expect(play.hintTry()).toMatchObject({ passed: false });
      expect(play.run()?.canTryHint).toBe(false);
      // The player's own state never holds a DC: the run has none to hold.
      expect(JSON.stringify(play.hintTry())).not.toMatch(/dc/i);
    });

    it('sends again with the SAME key when the answer never came', async () => {
      const { fake, play } = setup();
      fake.playerRunResult = { ...base, revision: 1 };
      await play.open('p1');
      fake.hintResult = (n) => {
        if (n < 2) {
          throw new ConnectError('down', Code.Unavailable);
        }
        return hintAnswer({ ...base, revision: 2 }, true, 15, { replayed: true });
      };
      await play.tryHint({ inApp: true });
      expect(fake.hintKeys).toEqual(['key-1', 'key-1']);
    });

    it('tries nothing when the server did not say the player may, and says the refusal in words', async () => {
      const { fake, play } = setup();
      fake.playerRunResult = { ...base, revision: 1, canTryHint: false };
      await play.open('p1');
      fake.hintResult = () => hintAnswer(base, true);
      await play.tryHint({ inApp: true });
      expect(fake.hintKeys).toEqual([]);

      fake.playerRunResult = { ...base, revision: 3 };
      await play.refresh();
      fake.hintResult = () => {
        throw new ConnectError('x', Code.FailedPrecondition, undefined, [{ desc: PuzzleBlockedSchema, value: create(PuzzleBlockedSchema, { reason: PuzzleBlockedReason.HINT_ALREADY_TRIED }) }]);
      };
      await play.tryHint({ inApp: true });
      expect(play.message()).toContain('Você já tentou esta dica');
    });
  });

  describe('the sequence played step by step (a fake clock, no real sleeps)', () => {
    const seq = sequencePuzzle('p1', 'Os sinos');
    const playing = (revision: number, shown: number[], nextInMs: number, plays = 1): PuzzleRun => ({
      ...playerRun(seq, { sequence: { totalSteps: 6, plays, playing: nextInMs > 0, shown, stepMs: 1200, nextInMs } }),
      revision,
    });

    it('reads again when the server says the next step is shown, and keeps asking until the play ends', async () => {
      const { fake, play, timers } = setup();
      fake.playerRunResult = playing(2, [0], 1200);
      await play.open('p1');
      expect(timers).toHaveLength(1);
      expect(timers[0].ms).toBeGreaterThanOrEqual(1200);
      // The server's clock moved on a step: the same revision, one more bell shown.
      fake.playerRunResult = playing(2, [0, 1], 1200);
      timers[0].fn();
      await Promise.resolve();
      await Promise.resolve();
      expect(play.run()?.sequence?.shown).toEqual([0, 1]);
      expect(timers).toHaveLength(2);
      // The last step ends the play: nothing more is asked.
      fake.playerRunResult = playing(2, [], 0);
      timers[1].fn();
      await Promise.resolve();
      await Promise.resolve();
      expect(play.run()?.sequence?.playing).toBe(false);
      expect(timers).toHaveLength(2);
    });

    it('never shows a step older than the one on screen: a late read of the same revision is ignored', async () => {
      const { fake, play } = setup();
      fake.playerRunResult = playing(2, [0, 1, 3], 600);
      await play.open('p1');
      expect(play.apply(playing(2, [0, 1], 600))).toBe(false);
      expect(play.run()?.sequence?.shown).toEqual([0, 1, 3]);
    });

    it('takes a new play of the sequence even when nothing else in the run changed', async () => {
      const { fake, play } = setup();
      fake.playerRunResult = playing(2, [], 0, 1);
      await play.open('p1');
      expect(play.apply(playing(2, [0], 1200, 2))).toBe(true);
      expect(play.run()?.sequence?.plays).toBe(2);
    });

    it('stops the timer when the page leaves or another puzzle opens', async () => {
      const { fake, play, timers } = setup();
      fake.playerRunResult = playing(2, [0], 1200);
      await play.open('p1');
      play.dispose();
      expect(timers[0].cancelled).toBe(true);
    });

    it('takes the run when the player found the cipher\'s key (a clue in the notes moves no revision)', async () => {
      const { fake, play } = setup();
      const cipher = playerRun(cipherPuzzle('p1', 'x'), { hasKeyClue: true });
      fake.playerRunResult = { ...cipher, revision: 3 };
      await play.open('p1');
      expect(play.apply({ ...cipher, revision: 3, keyClueId: 'k1' })).toBe(true);
      expect(play.run()?.keyClueId).toBe('k1');
    });

    it('takes the run as stopped when only the clock says so (a time limit moves no revision)', async () => {
      const { fake, play } = setup();
      fake.playerRunResult = { ...playerRun(riddlePuzzle('p1', 'x')), revision: 4 };
      await play.open('p1');
      expect(play.apply({ ...playerRun(riddlePuzzle('p1', 'x')), revision: 4, stopped: true })).toBe(true);
      expect(play.run()?.stopped).toBe(true);
    });
  });
});
