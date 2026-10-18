import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';
import { vi } from 'vitest';

import {
  ResourceBlockedReason,
  ResourceBlockedSchema,
} from '../../../gen/meurpg/play/v1/resources_pb';
import { type HeldQuestionsDeps, HeldQuestions, NOTE_LINGER_MS } from './held-questions';
import { MASTER_WAIT_SECONDS } from './held-roll-controller';
import { MASTER_TOO_EARLY_REASON } from './held-roll-errors';
import type { HeldAnswer } from './held-roll-offer';
import { askedAt, heldAnswer, heldDie, heldLucky } from './held-roll-testing';

const T0 = 1_700_000_000_000;
const SECOND = 1000;
const reroll: HeldAnswer = { kind: 'lucky', reroll: true, roll: { inApp: true } };
/** A reason number no generated value uses. */
const TOO_EARLY = 9999;
const keep: HeldAnswer = { kind: 'lucky', reroll: false, roll: null };

/** A question the roller asked `agoSeconds` ago, in a combat for `combatantId`. */
const asked = (hold: string, agoSeconds: number, combatantId = 'c-brisa', roller = 'Brisa') =>
  heldLucky([1], 5, {
    holdId: hold,
    combatantId,
    rollerLabel: roller,
    ...askedAt(T0 - agoSeconds * SECOND),
  });

describe('HeldQuestions', () => {
  let sent: { hold: string; answer: HeldAnswer; key: string }[];
  let reply: () => Promise<ReturnType<typeof heldAnswer>>;
  let questions: HeldQuestions;
  let counter: number;

  function build(over: Partial<HeldQuestionsDeps> = {}): HeldQuestions {
    questions = new HeldQuestions({
      send: (hold, answer, key) => {
        sent.push({ hold, answer, key });
        return reply();
      },
      clock: () => Date.now(),
      newKey: () => `k${counter++}`,
      ...over,
    });
    return questions;
  }

  beforeEach(() => {
    vi.useFakeTimers({ now: T0, toFake: ['Date', 'setInterval', 'clearInterval'] });
    sent = [];
    counter = 0;
    reply = () => Promise.resolve(heldAnswer());
  });

  afterEach(() => {
    questions.dispose();
    vi.useRealTimers();
  });

  it('waits for the roller for ten seconds, then lets the master answer', () => {
    build().sync([asked('h1', 3)]);
    const q = questions.entries()[0];
    expect(q.phase()).toBe('waiting');
    expect(q.seconds()).toBe(MASTER_WAIT_SECONDS - 3);
    expect(q.lead()).toBe('Brisa responde primeiro');
    expect(q.countdown()).toBe('você em 7 s');
    vi.advanceTimersByTime(6 * SECOND);
    expect(q.seconds()).toBe(1);
    vi.advanceTimersByTime(SECOND);
    expect(q.phase()).toBe('ready');
    expect(q.lead()).toBe('Brisa não respondeu');
    expect(q.countdown()).toBe('');
  });

  it('ticks once a second while something waits, and stops when nothing does', () => {
    build().sync([asked('h1', 8)]);
    expect(vi.getTimerCount()).toBe(1);
    vi.advanceTimersByTime(2 * SECOND);
    expect(questions.entries()[0].phase()).toBe('ready');
    expect(vi.getTimerCount()).toBe(0);
    // A question already past its wait never starts the clock.
    build().sync([asked('h2', 30)]);
    expect(vi.getTimerCount()).toBe(0);
  });

  it('counts the questions that wait and finds the one of a combatant', () => {
    build().sync([asked('h1', 3), asked('h2', 3, 'c-pens', 'Pensantus')]);
    expect(questions.count()).toBe(2);
    expect(questions.forCombatant('c-pens')?.roller).toBe('Pensantus');
    expect(questions.forCombatant('nobody')).toBeNull();
  });

  it('keeps a question across reads, and keeps its countdown', () => {
    build().sync([asked('h1', 3)]);
    const first = questions.entries()[0];
    questions.sync([asked('h1', 3)]);
    expect(questions.entries()[0]).toBe(first);
    expect(questions.entries()).toHaveLength(1);
  });

  it('answers for the roller once the wait is over, under one key', async () => {
    build().sync([asked('h1', 12)]);
    const q = questions.entries()[0];
    await q.answer(reroll);
    expect(sent).toEqual([{ hold: 'h1', answer: reroll, key: 'k0' }]);
  });

  it('says the answer in the words of the board while it is on the way', async () => {
    let finish: (v: ReturnType<typeof heldAnswer>) => void = () => undefined;
    reply = () => new Promise((resolve) => (finish = resolve));
    build().sync([asked('h1', 12)]);
    const q = questions.entries()[0];
    const done = q.answer(reroll);
    expect(q.phase()).toBe('answering');
    expect(q.asking()).toBe('Rolando o d20 de novo…');
    finish(heldAnswer());
    await done;
    expect(q.phase()).toBe('done');
    expect(q.note()).toBe('Você respondeu por Brisa: rolou de novo.');
  });

  it('goes on to the die’s question when the answer brings it', async () => {
    reply = () => Promise.resolve(heldAnswer({ next: heldDie(14, 5, 8, { holdId: 'h1' }, 1) }));
    build().sync([asked('h1', 12)]);
    const q = questions.entries()[0];
    await q.answer(reroll);
    expect(q.phase()).not.toBe('done');
    expect(q.controller.offer()?.question.case).toBe('die');
  });

  it('shows the countdown again when the server says it is too early, whatever the local clock says', async () => {
    // The generated enum does not name the reason yet: add the name to its reverse map as the new enum would.
    const names = ResourceBlockedReason as unknown as Record<number, string>;
    names[TOO_EARLY] = MASTER_TOO_EARLY_REASON;
    try {
      reply = () =>
        Promise.reject(
          new ConnectError('cedo', Code.FailedPrecondition, undefined, [
            {
              desc: ResourceBlockedSchema,
              value: create(ResourceBlockedSchema, { reason: TOO_EARLY as ResourceBlockedReason }),
            },
          ]),
        );
      build().sync([asked('h1', 12)]);
      const q = questions.entries()[0];
      // The server clock is behind ours: it still counts, so the buttons wait a little more.
      await q.answer(reroll);
      expect(q.phase()).toBe('too-early');
      expect(q.seconds()).toBeGreaterThanOrEqual(1);
      expect(q.alertLead()).toBe('Ainda é a vez de Brisa responder.');
      vi.advanceTimersByTime(5 * SECOND);
      expect(q.phase()).toBe('ready');
    } finally {
      delete names[TOO_EARLY];
    }
  });

  it('says the question was answered when the race is lost', async () => {
    reply = () => Promise.reject(new ConnectError('x', Code.FailedPrecondition));
    build().sync([asked('h1', 12)]);
    const q = questions.entries()[0];
    await q.answer(keep);
    expect(q.phase()).toBe('lost');
    expect(q.controller.error()).toBe(
      'Essa pergunta já foi respondida. O resultado está no registro do combate.',
    );
  });

  it('drops a question the server stopped listing, saying the roller answered, then forgets it', () => {
    build().sync([asked('h1', 3)]);
    questions.sync([]);
    const q = questions.entries()[0];
    expect(q.phase()).toBe('closed');
    expect(q.note()).toBe('Brisa respondeu.');
    expect(questions.count()).toBe(0);
    vi.advanceTimersByTime(NOTE_LINGER_MS + SECOND);
    expect(questions.entries()).toEqual([]);
    expect(vi.getTimerCount()).toBe(0);
  });

  it('keeps the master’s own answer on screen for a while, then forgets it', async () => {
    build().sync([asked('h1', 12)]);
    await questions.entries()[0].answer(keep);
    questions.sync([]);
    expect(questions.entries()[0].note()).toBe('Você respondeu por Brisa: ficou com o 1.');
    vi.advanceTimersByTime(NOTE_LINGER_MS + SECOND);
    expect(questions.entries()).toEqual([]);
  });

  it('does not drop a question whose answer is on the way', () => {
    reply = () => new Promise(() => undefined);
    build().sync([asked('h1', 12)]);
    void questions.entries()[0].answer(reroll);
    questions.sync([]);
    expect(questions.entries()[0].phase()).toBe('answering');
  });

  it('takes a new question of the same roll as the old one’s next', () => {
    build().sync([asked('h1', 12)]);
    const first = questions.entries()[0];
    questions.sync([heldDie(11, 5, 8, { holdId: 'h1', combatantId: 'c-brisa', ...askedAt(T0) })]);
    expect(questions.entries()[0]).toBe(first);
    expect(first.controller.offer()?.question.case).toBe('die');
  });
});
