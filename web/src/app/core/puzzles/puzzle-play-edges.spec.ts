import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import {
  LuckyOfferSchema,
  OutsideInspirationOfferSchema,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { RollMode } from '../../../gen/meurpg/play/v1/combat_rolls_pb';
import type { AnswerHeldRollResponse } from '../../../gen/meurpg/play/v1/resources_pb';
import { type HeldAnswer, asksLucky } from '../held-rolls/held-roll-offer';
import { heldAnswer } from '../held-rolls/held-roll-testing';
import {
  PuzzleBlockedReason,
  PuzzleBlockedSchema,
  type PuzzleRun,
  TryPuzzleHintResponseSchema,
  type TryPuzzleHintResponse,
} from '../../../gen/meurpg/play/v1/puzzles_pb';
import { nextTurn } from '../../../test-settle';
import { PuzzlePlay, type PlayApi } from './puzzle-play';
import type { MoveAnswer } from './puzzles-client';
import {
  hintAnswer,
  lightsPuzzle,
  playerRun,
  riddlePuzzle,
  sequencePuzzle,
} from './puzzles-testing';

interface Deferred<T> {
  promise: Promise<T>;
  resolve: (v: T) => void;
  reject: (e: unknown) => void;
}
function deferred<T>(): Deferred<T> {
  let resolve!: (v: T) => void;
  let reject!: (e: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

/** An API whose every call waits for the spec to answer it. */
class Api implements PlayApi {
  runs: Deferred<PuzzleRun>[] = [];
  moves: Deferred<MoveAnswer>[] = [];
  hints: Deferred<TryPuzzleHintResponse>[] = [];
  dieAnswers: Deferred<TryPuzzleHintResponse>[] = [];
  luckyAnswers: Deferred<TryPuzzleHintResponse>[] = [];
  luckyKeys: string[] = [];
  extraFaces: (readonly number[] | undefined)[] = [];
  run(): Promise<PuzzleRun> {
    const d = deferred<PuzzleRun>();
    this.runs.push(d);
    return d.promise;
  }
  move(): Promise<MoveAnswer> {
    const d = deferred<MoveAnswer>();
    this.moves.push(d);
    return d.promise;
  }
  tryHint(
    _c: string,
    _p: string,
    _d: unknown,
    _k: string,
    extra?: readonly number[],
  ): Promise<TryPuzzleHintResponse> {
    this.extraFaces.push(extra);
    const d = deferred<TryPuzzleHintResponse>();
    this.hints.push(d);
    return d.promise;
  }
  answerHeldRoll(
    _c: string,
    _h: string,
    answer: HeldAnswer,
    key: string,
  ): Promise<AnswerHeldRollResponse> {
    const d = deferred<TryPuzzleHintResponse>();
    if (answer.kind === 'lucky') {
      this.luckyKeys.push(key);
      this.luckyAnswers.push(d);
    } else {
      this.dieAnswers.push(d);
    }
    // The path's own response in `result`, the next question copied out of it, as the server answers.
    return d.promise.then((value) => heldAnswer({ result: { case: 'hintTry', value } }));
  }
}

function setup(api: PlayApi = new Api()): { api: Api; play: PuzzlePlay } {
  let key = 0;
  const play = new PuzzlePlay(
    api,
    () => 'camp-1',
    async () => undefined,
    () => `key-${++key}`,
    () => () => undefined,
  );
  return { api: api as Api, play };
}

const riddle = riddlePuzzle('p1', 'A porta');
const hintable = playerRun(riddle, {
  hintByCheck: true,
  hintSkillKey: 'skill:investigation',
  canTryHint: true,
  hints: [],
  sharedHints: 0,
});
const answerMove = { kind: { case: 'riddle' as const, value: { answer: 'sombra' } } };

async function opened(api: Api, play: PuzzlePlay, run: PuzzleRun = hintable): Promise<void> {
  const opening = play.open('p1');
  api.runs[api.runs.length - 1].resolve(run);
  await opening;
}

const refused = (reason: PuzzleBlockedReason) =>
  new ConnectError('x', Code.FailedPrecondition, undefined, [
    { desc: PuzzleBlockedSchema, value: create(PuzzleBlockedSchema, { reason }) },
  ]);

describe('PuzzlePlay, reading the run', () => {
  it('reads nothing before a puzzle was opened', async () => {
    const { api, play } = setup();
    await play.refresh();
    expect(api.runs).toHaveLength(0);
    expect(play.run()).toBeNull();
  });

  it('ignores a successful read of the puzzle that was open before', async () => {
    const { api, play } = setup();
    const first = play.open('p1');
    const second = play.open('p2');
    api.runs[0].resolve(playerRun(riddle, { revision: 5 }));
    await first;
    expect(play.run()).toBeNull();
    api.runs[1].resolve(playerRun(riddlePuzzle('p2', 'Outra'), { revision: 1 }));
    await second;
    expect(play.run()?.puzzleId).toBe('p2');
  });

  it('keeps what is on screen when a later read fails, and says so only when nothing is', async () => {
    const { api, play } = setup();
    await opened(api, play);
    const again = play.refresh();
    api.runs[1].reject(new ConnectError('x', Code.Unavailable));
    await again;
    expect(play.run()).not.toBeNull();
    expect(play.loadError()).toBe('');

    const blank = setup();
    const opening = blank.play.open('p1');
    blank.api.runs[0].reject(new ConnectError('x', Code.Unavailable));
    await opening;
    expect(blank.play.loadError()).toBe(
      'Não deu para abrir o quebra-cabeça: o servidor não respondeu. Tente de novo.',
    );
  });

  it('forgets a failed first read when the next one works', async () => {
    const { api, play } = setup();
    const first = play.open('p1');
    api.runs[0].reject(new ConnectError('x', Code.Unavailable));
    await first;
    expect(play.loadError()).not.toBe('');
    const again = play.refresh();
    api.runs[1].resolve(hintable);
    await again;
    expect(play.loadError()).toBe('');
    expect(play.gone()).toBe(false);
  });
});

describe('PuzzlePlay, a move that outlives its puzzle', () => {
  it('drops the answer of a move sent to the puzzle that was open before', async () => {
    const { api, play } = setup();
    await opened(api, play);
    const verdict = play.move(answerMove);
    await nextTurn();
    const next = play.open('p2');
    api.runs[1].resolve(playerRun(riddlePuzzle('p2', 'Outra'), { revision: 1 }));
    await next;
    api.moves[0].resolve({
      run: playerRun(riddle, { revision: 9, solved: true }),
      replayed: false,
      solvedByThisMove: true,
      wrong: false,
    });
    expect(await verdict).toEqual({ sent: false, wrong: false, solved: false });
    expect(play.run()?.puzzleId).toBe('p2');
  });

  it('writes no message for the failure of a move sent to the puzzle that was open before', async () => {
    const { api, play } = setup();
    await opened(api, play);
    const verdict = play.move(answerMove);
    await nextTurn();
    const next = play.open('p2');
    api.runs[1].resolve(playerRun(riddlePuzzle('p2', 'Outra'), { revision: 1 }));
    await next;
    api.moves[0].reject(new ConnectError('x', Code.PermissionDenied));
    expect(await verdict).toEqual({ sent: false, wrong: false, solved: false });
    expect(play.message()).toBe('');
  });

  it('says solved when the run came back solved, and not wrong even when the server judged it so', async () => {
    const { api, play } = setup();
    await opened(api, play);
    const verdict = play.move(answerMove);
    await nextTurn();
    api.moves[0].resolve({
      run: playerRun(riddle, { revision: 2, solved: true }),
      replayed: false,
      solvedByThisMove: false,
      wrong: true,
    });
    expect(await verdict).toEqual({ sent: true, wrong: false, solved: true });
  });

  it('reads the run again after a refusal and after a not_found, not after another failure', async () => {
    const { api, play } = setup();
    await opened(api, play);
    const blocked = play.move(answerMove);
    await nextTurn();
    api.moves[0].reject(refused(PuzzleBlockedReason.SOLVED));
    await nextTurn();
    expect(api.runs).toHaveLength(2);
    api.runs[1].resolve(playerRun(riddle, { revision: 2 }));
    await blocked;

    const gone = play.move(answerMove);
    await nextTurn();
    api.moves[1].reject(new ConnectError('x', Code.NotFound));
    await nextTurn();
    expect(api.runs).toHaveLength(3);
    api.runs[2].resolve(playerRun(riddle, { revision: 3 }));
    await gone;

    const other = play.move(answerMove);
    await nextTurn();
    api.moves[2].reject(new ConnectError('x', Code.PermissionDenied));
    await other;
    expect(api.runs).toHaveLength(3);
    expect(play.message()).toBe('Você não pode fazer isso agora.');
  });
});

describe('PuzzlePlay, a try for a hint', () => {
  it('asks for the extra dice before a real die is sent, and sends none of them for the app', async () => {
    const { api, play } = setup();
    await opened(api, play);
    play.extraDice.fields.set([{ faces: 4, sign: 1, name: 'Bênção' } as never]);
    await play.tryHint({ face: 7 });
    expect(api.hints).toHaveLength(0);
    expect(play.message()).toBe('Digite o resultado do d4 antes de confirmar.');
    play.extraDice.faces.set([3]);
    const typed = play.tryHint({ face: 7 });
    await nextTurn();
    expect(api.extraFaces).toEqual([[3]]);
    api.hints[0].resolve(hintAnswer(hintable, true));
    await typed;
    // Settled: the form for the extra dice goes away.
    expect(play.extraDice.fields()).toEqual([]);
  });

  it('asks for more d4 when the refusal says the roll takes them, with no error line of its own', async () => {
    const { api, play } = setup();
    await opened(api, play);
    const attempt = play.tryHint({ face: 7 });
    await nextTurn();
    api.hints[0].reject(new ConnectError('the roll takes 2 more die(s)', Code.InvalidArgument));
    await attempt;
    expect(play.extraDice.fields()).toHaveLength(2);
    expect(play.message()).toBe('Esta rolagem leva mais 2 d4: role-os e digite os resultados.');
    expect(play.hintPair()).toBe(false);
  });

  it('shows the error of an app roll refused for the pair, since only a typed face can be asked again', async () => {
    const { api, play } = setup();
    await opened(api, play);
    const attempt = play.tryHint({ inApp: true });
    await nextTurn();
    api.hints[0].reject(new ConnectError('the roll takes 2 d20', Code.InvalidArgument));
    await attempt;
    expect(play.hintPair()).toBe(false);
    expect(play.message()).not.toBe('');
  });

  it('reads the run again after a refusal or a not_found of the try, and not after another failure', async () => {
    const { api, play } = setup();
    await opened(api, play);
    const first = play.tryHint({ inApp: true });
    await nextTurn();
    api.hints[0].reject(refused(PuzzleBlockedReason.HINT_ALREADY_TRIED));
    await nextTurn();
    expect(api.runs).toHaveLength(2);
    api.runs[1].resolve({ ...hintable, revision: 2 });
    await first;

    const second = play.tryHint({ inApp: true });
    await nextTurn();
    api.hints[1].reject(new ConnectError('x', Code.NotFound));
    await nextTurn();
    expect(api.runs).toHaveLength(3);
    api.runs[2].resolve({ ...hintable, revision: 3 });
    await second;

    const third = play.tryHint({ inApp: true });
    await nextTurn();
    api.hints[2].reject(new ConnectError('x', Code.PermissionDenied));
    await third;
    expect(api.runs).toHaveLength(3);
    expect(play.hintBusy()).toBe(false);
  });

  it('does not try while another try is in flight or after the master closed it', async () => {
    const { api, play } = setup();
    await opened(api, play);
    const first = play.tryHint({ inApp: true });
    await nextTurn();
    expect(play.hintBusy()).toBe(true);
    await play.tryHint({ inApp: true });
    expect(api.hints).toHaveLength(1);
    api.hints[0].resolve(hintAnswer(hintable, true));
    await first;

    const closed = setup();
    await opened(closed.api, closed.play);
    const reading = closed.play.refresh();
    closed.api.runs[1].reject(new ConnectError('x', Code.NotFound));
    await reading;
    expect(closed.play.gone()).toBe(true);
    await closed.play.tryHint({ inApp: true });
    expect(closed.api.hints).toHaveLength(0);
  });

  it('keeps the current run when the answer carries none, and remembers the roll and the mode', async () => {
    const { api, play } = setup();
    await opened(api, play);
    const attempt = play.tryHint({ inApp: true });
    await nextTurn();
    api.hints[0].resolve(
      create(TryPuzzleHintResponseSchema, { passed: false, mode: RollMode.ADVANTAGE }),
    );
    await attempt;
    expect(play.run()?.revision).toBe(hintable.revision);
    expect(play.hintTry()).toMatchObject({ passed: false, mode: RollMode.ADVANTAGE });
    expect(play.hintPair()).toBe(true);
  });

  it('shows the Lucky question before the die’s when the answer carries both', async () => {
    const { api, play } = setup();
    await opened(api, play);
    const attempt = play.tryHint({ inApp: true });
    await nextTurn();
    api.hints[0].resolve(
      create(TryPuzzleHintResponseSchema, {
        luckyOffer: create(LuckyOfferSchema, { holdId: 'l1' }),
        inspirationOffer: create(OutsideInspirationOfferSchema, { holdId: 'i1', sides: 6 }),
      }),
    );
    await attempt;
    expect(play.hintHeld()?.holdId).toBe('l1');
    expect(asksLucky(play.hintHeld())).toBe(true);
    expect(play.hintTry()).toBeNull();
  });
});

describe('PuzzlePlay, the answers to the questions of a try', () => {
  const offerAnswer = () =>
    create(TryPuzzleHintResponseSchema, {
      inspirationOffer: create(OutsideInspirationOfferSchema, { holdId: 'h1', sides: 8 }),
    });
  const luckyAnswer = () =>
    create(TryPuzzleHintResponseSchema, { luckyOffer: create(LuckyOfferSchema, { holdId: 'l1' }) });

  async function withOffer(
    answer: () => TryPuzzleHintResponse,
  ): Promise<{ api: Api; play: PuzzlePlay }> {
    const made = setup();
    await opened(made.api, made.play);
    const attempt = made.play.tryHint({ inApp: true });
    await nextTurn();
    made.api.hints[0].resolve(answer());
    await attempt;
    return made;
  }

  it('does nothing without an offer, without a run or when the client has no such question', async () => {
    const { play } = setup();
    await play.answerHint({ kind: 'die', use: true, roll: null });
    await play.answerHint({ kind: 'lucky', reroll: true, roll: null });
    expect(play.hintBusy()).toBe(false);

    const bare: PlayApi = {
      run: () => Promise.resolve(hintable),
      move: () => Promise.reject(new Error('unused')),
      tryHint: () => Promise.resolve(offerAnswer()),
    };
    const made = setup(bare);
    await made.play.open('p1');
    await made.play.tryHint({ inApp: true });
    expect(made.play.hintHeld()).not.toBeNull();
    await made.play.answerHint({ kind: 'die', use: true, roll: null });
    expect(made.play.hintHeld()).not.toBeNull();
    expect(made.play.hintBusy()).toBe(false);
  });

  it('says why the Bardic Inspiration answer failed, and lets the player answer again', async () => {
    const { api, play } = await withOffer(offerAnswer);
    const answering = play.answerHint({ kind: 'die', use: true, roll: { inApp: true } });
    await nextTurn();
    expect(play.hintBusy()).toBe(true);
    api.dieAnswers[0].reject(new ConnectError('x', Code.PermissionDenied));
    await answering;
    expect(play.message()).toBe('Você não pode fazer isso agora.');
    expect(play.hintBusy()).toBe(false);
    expect(play.hintHeld()).not.toBeNull();
  });

  it('says why the Lucky answer failed, and keeps the offer', async () => {
    const { api, play } = await withOffer(luckyAnswer);
    const answering = play.answerHint({ kind: 'lucky', reroll: true, roll: { inApp: true } });
    await nextTurn();
    api.luckyAnswers[0].reject(new ConnectError('x', Code.PermissionDenied));
    await answering;
    expect(play.message()).toBe('Você não pode fazer isso agora.');
    expect(play.hintHeld()).not.toBeNull();
    expect(play.hintBusy()).toBe(false);
  });

  it('settles the try with a Lucky answer, and uses the same key for the same answer until it got through', async () => {
    const { api, play } = await withOffer(luckyAnswer);
    const first = play.answerHint({ kind: 'lucky', reroll: false, roll: null });
    await nextTurn();
    api.luckyAnswers[0].reject(new ConnectError('x', Code.PermissionDenied));
    await first;
    const second = play.answerHint({ kind: 'lucky', reroll: false, roll: null });
    await nextTurn();
    expect(api.luckyKeys[1]).toBe(api.luckyKeys[0]);
    api.luckyAnswers[1].resolve(hintAnswer(hintable, true));
    await second;
    expect(play.hintHeld()).toBeNull();
    expect(play.hintTry()?.passed).toBe(true);
    const third = await withOffer(luckyAnswer);
    expect(third.api.luckyKeys).toEqual([]);
  });

  it('writes nothing of an answer that comes after another puzzle was opened', async () => {
    const { api, play } = await withOffer(offerAnswer);
    const answering = play.answerHint({ kind: 'die', use: true, roll: null });
    await nextTurn();
    const next = play.open('p2');
    api.runs[1].resolve(playerRun(riddlePuzzle('p2', 'Outra'), { revision: 1 }));
    await next;
    api.dieAnswers[0].resolve(hintAnswer(hintable, true));
    await answering;
    expect(play.hintTry()).toBeNull();
    expect(play.run()?.puzzleId).toBe('p2');

    const lucky = await withOffer(luckyAnswer);
    const askedLucky = lucky.play.answerHint({ kind: 'lucky', reroll: true, roll: null });
    await nextTurn();
    const other = lucky.play.open('p2');
    lucky.api.runs[1].resolve(playerRun(riddlePuzzle('p2', 'Outra'), { revision: 1 }));
    await other;
    lucky.api.luckyAnswers[0].reject(new ConnectError('x', Code.PermissionDenied));
    await askedLucky;
    expect(lucky.play.message()).toBe('');
  });
});

describe('PuzzlePlay, the real clock', () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it('waits on the real clock before sending a move again, and reads the sequence again on it', async () => {
    vi.useFakeTimers();
    let calls = 0;
    const api: PlayApi = {
      run: () => Promise.resolve(hintable),
      move: () => {
        calls++;
        return calls === 1
          ? Promise.reject(new ConnectError('x', Code.Unavailable))
          : Promise.resolve({
              run: playerRun(riddle, { revision: 2 }),
              replayed: false,
              solvedByThisMove: false,
              wrong: false,
            });
      },
      tryHint: () => Promise.reject(new Error('unused')),
    };
    const play = new PuzzlePlay(api, () => 'camp-1');
    await play.open('p1');
    const verdict = play.move(answerMove);
    await vi.advanceTimersByTimeAsync(399);
    expect(calls).toBe(1);
    await vi.advanceTimersByTimeAsync(1);
    expect(await verdict).toMatchObject({ sent: true });
    expect(calls).toBe(2);
    play.dispose();
  });

  it('reads the playing sequence a little after the step the server named, and stops on dispose', async () => {
    vi.useFakeTimers();
    let reads = 0;
    const sequence = playerRun(sequencePuzzle('p1', 's'), {
      revision: 2,
      sequence: { totalSteps: 6, plays: 1, playing: true, shown: [0], stepMs: 1200, nextInMs: 900 },
    });
    const api: PlayApi = {
      run: () => {
        reads++;
        return Promise.resolve(sequence);
      },
      move: () => Promise.reject(new Error('unused')),
      tryHint: () => Promise.reject(new Error('unused')),
    };
    const play = new PuzzlePlay(api, () => 'camp-1');
    await play.open('p1');
    expect(reads).toBe(1);
    await vi.advanceTimersByTimeAsync(959);
    expect(reads).toBe(1);
    await vi.advanceTimersByTimeAsync(1);
    expect(reads).toBe(2);
    play.dispose();
    await vi.advanceTimersByTimeAsync(5000);
    expect(reads).toBe(2);
  });
});

describe('PuzzlePlay, a run of lights', () => {
  it('judges a lights move at once, with no wait for another answer', async () => {
    const { api, play } = setup();
    await opened(api, play, playerRun(lightsPuzzle('p1', 'x')));
    const first = play.move({ kind: { case: 'lights', value: { row: 0, col: 0 } } });
    const second = play.move({ kind: { case: 'lights', value: { row: 1, col: 1 } } });
    await nextTurn();
    expect(api.moves).toHaveLength(2);
    expect(play.pending()).toBe(2);
    const done = (revision: number): MoveAnswer => ({
      run: playerRun(lightsPuzzle('p1', 'x'), { revision }),
      replayed: false,
      solvedByThisMove: false,
      wrong: false,
    });
    api.moves[0].resolve(done(2));
    api.moves[1].resolve(done(3));
    await Promise.all([first, second]);
    expect(play.pending()).toBe(0);
    expect(play.run()?.revision).toBe(3);
  });
});
