import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import {
  CharacterVitalsSchema,
  WatchGameSessionResponseSchema,
  GameSessionBlockedReason,
  GameSessionBlockedSchema,
} from '../../../gen/meurpg/play/v1/play_pb';
import { Recharge } from '../../../gen/meurpg/rules/v1/rules_pb';
import { TestBed } from '@angular/core/testing';
import type { Transport } from '@connectrpc/connect';

import { CONNECT_TRANSPORT } from '../../core/connect/transport';
import {
  LiveSessionSourceLive,
  classifyLiveError,
  toShownImageVm,
  toVitalsVm,
} from './live-session-source.live';

function blocked(reason: GameSessionBlockedReason): ConnectError {
  return new ConnectError('blocked', Code.FailedPrecondition, undefined, [
    { desc: GameSessionBlockedSchema, value: { reason } },
  ]);
}

describe('classifyLiveError', () => {
  it('reads the code and the typed detail, never the message', () => {
    expect(classifyLiveError(new ConnectError('x', Code.NotFound))).toBe('no-access');
    expect(classifyLiveError(new ConnectError('x', Code.Unauthenticated))).toBe('signed-out');
    expect(classifyLiveError(new ConnectError('x', Code.InvalidArgument))).toBe('invalid');
    expect(classifyLiveError(new ConnectError('x', Code.PermissionDenied))).toBe('forbidden');
    expect(classifyLiveError(blocked(GameSessionBlockedReason.NO_OPEN_SESSION))).toBe('no-session');
  });

  it('treats everything else as worth another try', () => {
    expect(classifyLiveError(new ConnectError('x', Code.Unavailable))).toBe('transient');
    expect(classifyLiveError(new ConnectError('x', Code.Aborted))).toBe('transient');
    expect(classifyLiveError(new TypeError('Failed to fetch'))).toBe('transient');
    expect(classifyLiveError(blocked(GameSessionBlockedReason.SESSION_ALREADY_OPEN))).toBe(
      'transient',
    );
    expect(classifyLiveError(new ConnectError('no open session', Code.FailedPrecondition))).toBe(
      'transient',
    );
  });
});

describe('toVitalsVm', () => {
  it('maps the wire vitals, hit dice by size as "2d10 e 1d8" with what is spent of each', () => {
    const vm = toVitalsVm(
      create(CharacterVitalsSchema, {
        characterId: 'c1',
        name: 'Toren',
        hitPointsCurrent: 31,
        hitPointsMax: 31,
        hitDice: [
          { faces: 10, count: 2 },
          { faces: 8, count: 1 },
        ],
        hitDiceTotal: 3,
        hitDiceUsed: 1,
        hitDiceUsedByDie: { 8: 1 },
        spellSlots: [{ level: 1, total: 2, used: 1 }],
        pactSlots: { slotLevel: 2, total: 2, used: 0 },
        revision: 7,
      }),
    );
    expect(vm.hitDice).toBe('2d10 e 1d8');
    expect(vm.hitDiceSizes).toEqual([
      { faces: 10, total: 2, used: 0 },
      { faces: 8, total: 1, used: 1 },
    ]);
    expect(vm.hitDiceTotal).toBe(3);
    expect(vm.hitDiceUsed).toBe(1);
    expect(vm.spellSlots).toEqual([{ level: 1, total: 2, used: 1, created: 0 }]);
    expect(vm.pactSlots).toEqual({ slotLevel: 2, total: 2, used: 0 });
    expect(vm.revision).toBe(7);
  });
});

describe('toVitalsVm and the resources', () => {
  it('maps the slots Flexible Casting created and when each resource comes back', () => {
    const vm = toVitalsVm(
      create(CharacterVitalsSchema, {
        characterId: 'c1',
        spellSlots: [{ level: 2, total: 3, used: 0, created: 1 }],
        resources: [
          { key: 'ki', namePt: 'Chi', total: 5, used: 2, recharge: Recharge.SHORT_REST },
          { key: 'rage', namePt: 'Fúria', total: 3, used: 0, recharge: Recharge.LONG_REST },
          { key: 'odd', namePt: 'Outro', total: 1, used: 0, recharge: Recharge.UNSPECIFIED },
        ],
      }),
    );
    expect(vm.spellSlots[0].created).toBe(1);
    expect(vm.resources?.map((r) => [r.key, r.recharge])).toEqual([
      ['ki', 'short_rest'],
      ['rage', 'long_rest'],
      ['odd', 'none'],
    ]);
  });
});

describe('toShownImageVm', () => {
  it('maps the wire image, and nothing when no image is shown', () => {
    expect(
      toShownImageVm({
        $typeName: 'meurpg.play.v1.ShownImage',
        id: 'img-1',
        name: 'Capitão Goblin',
        width: 400,
        height: 500,
        url: '/images/img-1',
        thumbnailUrl: '/images/img-1/thumb',
      }),
    ).toEqual({
      id: 'img-1',
      name: 'Capitão Goblin',
      width: 400,
      height: 500,
      url: '/images/img-1',
    });
    expect(toShownImageVm(undefined)).toBeNull();
  });
});

describe('LiveSessionSourceLive.watch', () => {
  /** A transport whose one stream answers `responses`, so the mapping is tested through the real client. */
  function sourceAnswering(
    responses: ReturnType<typeof create<typeof WatchGameSessionResponseSchema>>[],
  ) {
    const transport = {
      stream: async () => ({
        stream: true,
        header: new Headers(),
        trailer: new Headers(),
        message: (async function* () {
          yield* responses;
        })(),
      }),
    } as unknown as Transport;
    TestBed.configureTestingModule({
      providers: [LiveSessionSourceLive, { provide: CONNECT_TRANSPORT, useValue: transport }],
    });
    return TestBed.inject(LiveSessionSourceLive);
  }

  async function events(responses: Parameters<typeof sourceAnswering>[0]) {
    const out = [];
    for await (const e of sourceAnswering(responses).watch(
      'camp-1',
      new AbortController().signal,
    )) {
      out.push(e.kind);
    }
    return out;
  }

  it('maps `xp_changed` to its own event (MR-016)', async () => {
    expect(
      await events([
        create(WatchGameSessionResponseSchema, { event: { case: 'ready', value: {} } }),
        create(WatchGameSessionResponseSchema, { event: { case: 'xpChanged', value: {} } }),
      ]),
    ).toEqual(['ready', 'xpChanged']);
  });

  it('maps `creatures_changed` to its own event (MR-037)', async () => {
    expect(
      await events([
        create(WatchGameSessionResponseSchema, { event: { case: 'ready', value: {} } }),
        create(WatchGameSessionResponseSchema, { event: { case: 'creaturesChanged', value: {} } }),
      ]),
    ).toEqual(['ready', 'creaturesChanged']);
  });

  it("maps `puzzle_changed` to its own event, with the puzzle's ID and nothing else (MR-038)", async () => {
    const out = [];
    const responses = [
      create(WatchGameSessionResponseSchema, { event: { case: 'ready', value: {} } }),
      create(WatchGameSessionResponseSchema, {
        event: { case: 'puzzleChanged', value: { puzzleId: 'p-1' } },
      }),
    ];
    for await (const e of sourceAnswering(responses).watch(
      'camp-1',
      new AbortController().signal,
    )) {
      out.push(e);
    }
    expect(out).toEqual([{ kind: 'ready' }, { kind: 'puzzleChanged', puzzleId: 'p-1' }]);
  });

  it('maps `content_changed` to its own event, with no content (RN-23, RN-10)', async () => {
    expect(
      await events([
        create(WatchGameSessionResponseSchema, { event: { case: 'ready', value: {} } }),
        create(WatchGameSessionResponseSchema, { event: { case: 'contentChanged', value: {} } }),
      ]),
    ).toEqual(['ready', 'contentChanged']);
  });

  it('still takes an event it does not know as a sign the stream is alive', async () => {
    // An empty `event` is what a newer server's oneof case looks like to this app.
    expect(await events([create(WatchGameSessionResponseSchema, {})])).toEqual(['heartbeat']);
  });
});
