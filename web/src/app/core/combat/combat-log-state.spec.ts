import { create } from '@bufbuild/protobuf';

import {
  CombatLogEntrySchema,
  CombatLogRoundSchema,
  ListCombatLogResponseSchema,
  type ListCombatLogResponse,
} from '../../../gen/meurpg/play/v1/combat_pb';
import type { CombatClient } from './combat-client';
import { CombatLogState } from './combat-log-state';

function entry(id: string, seq: number, extra: { undoable?: boolean; actorLabel?: string } = {}) {
  return create(CombatLogEntrySchema, { id, seq: BigInt(seq), ...extra });
}

function answer(
  rounds: [number, ReturnType<typeof entry>[]][],
  over: { lastSeq: number; full?: boolean; undoableEventId?: string },
): ListCombatLogResponse {
  return create(ListCombatLogResponseSchema, {
    rounds: rounds.map(([round, entries]) => create(CombatLogRoundSchema, { round, entries })),
    lastSeq: BigInt(over.lastSeq),
    full: over.full ?? false,
    undoableEventId: over.undoableEventId ?? '',
  });
}

describe('CombatLogState (since_seq)', () => {
  let asked: bigint[];
  let answers: ListCombatLogResponse[];
  let api: CombatClient;
  let log: CombatLogState;

  const ids = () => log.rounds().map((r) => r.entries.map((e) => e.id));

  beforeEach(() => {
    asked = [];
    answers = [];
    api = {
      turn: {
        log: (_c: string, _e: string, since = 0n) => {
          asked.push(since);
          return Promise.resolve(answers.shift()!);
        },
      },
    } as unknown as CombatClient;
    log = new CombatLogState();
  });

  it('reads the whole log first, then asks from the last_seq it holds and merges the delta', async () => {
    answers.push(
      answer(
        [
          [2, [entry('c', 30)]],
          [1, [entry('b', 20), entry('a', 10)]],
        ],
        { lastSeq: 30, full: true },
      ),
      answer(
        [
          [2, [entry('e', 50), entry('d', 40)]],
          [3, [entry('f', 60)]],
        ],
        { lastSeq: 60 },
      ),
    );
    await log.load(api, 'c1', 'e1');
    await log.load(api, 'c1', 'e1');
    expect(asked).toEqual([0n, 30n]);
    expect(ids()).toEqual([['f'], ['e', 'd', 'c'], ['b', 'a']]);
    answers.push(answer([], { lastSeq: 60 }));
    await log.load(api, 'c1', 'e1');
    expect(asked[2]).toBe(60n);
  });

  it('puts an entry that comes again where it stands, replaced by id', async () => {
    answers.push(
      answer([[1, [entry('b', 20), entry('a', 10)]]], { lastSeq: 20, full: true }),
      answer([[1, [entry('a', 25, { actorLabel: 'Goblin 2' })]]], { lastSeq: 25 }),
    );
    await log.load(api, 'c1', 'e1');
    await log.load(api, 'c1', 'e1');
    expect(log.rounds()[0].entries.map((e) => e.id)).toEqual(['a', 'b']);
    expect(log.rounds()[0].entries[0].actorLabel).toBe('Goblin 2');
    expect(log.rounds()[0].entries).toHaveLength(2);
  });

  it('replaces everything when the answer says full, and when asked for a full read', async () => {
    answers.push(
      answer([[1, [entry('b', 20), entry('a', 10)]]], { lastSeq: 20, full: true }),
      answer([[1, [entry('z', 5)]]], { lastSeq: 5, full: true }),
      answer([[1, [entry('y', 7)]]], { lastSeq: 7 }),
    );
    await log.load(api, 'c1', 'e1');
    await log.load(api, 'c1', 'e1');
    expect(ids()).toEqual([['z']]);
    await log.load(api, 'c1', 'e1', true);
    expect(asked).toEqual([0n, 20n, 0n]);
    expect(ids()).toEqual([['y']]);
  });

  it('asks the whole log again if the full read it asked for was replaced by a later one', async () => {
    let finish: (r: ListCombatLogResponse) => void = () => undefined;
    answers.push(answer([[1, [entry('a', 10)]]], { lastSeq: 10, full: true }));
    await log.load(api, 'c1', 'e1');
    api.turn.log = ((_c: string, _e: string, since = 0n) => {
      asked.push(since);
      return new Promise<ListCombatLogResponse>((resolve) => (finish = resolve));
    }) as never;
    const first = log.load(api, 'c1', 'e1', true);
    answers.push(answer([[1, [entry('a', 10)]]], { lastSeq: 10, full: true }));
    api.turn.log = ((_c: string, _e: string, since = 0n) => {
      asked.push(since);
      return Promise.resolve(answers.shift()!);
    }) as never;
    await log.load(api, 'c1', 'e1');
    finish(answer([], { lastSeq: 0 }));
    await first;
    expect(asked).toEqual([0n, 0n, 0n]);
  });

  it('derives undoable from the answer, not from the flag of a held entry', async () => {
    answers.push(
      answer([[1, [entry('b', 20, { undoable: true }), entry('a', 10)]]], {
        lastSeq: 20,
        full: true,
        undoableEventId: 'ev-b',
      }),
      // A new last action: the delta carries it flagged; the held entry 'b' still has its old flag.
      answer([[1, [entry('c', 30, { undoable: true })]]], {
        lastSeq: 30,
        undoableEventId: 'ev-c',
      }),
    );
    await log.load(api, 'c1', 'e1');
    expect(log.undoable()?.id).toBe('b');
    await log.load(api, 'c1', 'e1');
    expect(log.undoable()?.id).toBe('c');
    expect(log.rounds()[0].entries.filter((e) => e.undoable)).toHaveLength(1);
    expect(log.undoableId()).toBe('ev-c');
    // Nothing to undo any more: no held entry keeps the flag.
    answers.push(answer([], { lastSeq: 30 }));
    await log.load(api, 'c1', 'e1');
    expect(log.undoable()).toBeNull();
  });

  it('starts over for another combat', async () => {
    answers.push(
      answer([[1, [entry('a', 10)]]], { lastSeq: 10, full: true }),
      answer([[1, [entry('x', 3)]]], { lastSeq: 3, full: true }),
    );
    await log.load(api, 'c1', 'e1');
    await log.load(api, 'c1', 'e2');
    expect(asked).toEqual([0n, 0n]);
    expect(ids()).toEqual([['x']]);
  });
});
