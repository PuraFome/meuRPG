import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import {
  CharacterVitalsSchema,
  WatchGameSessionResponseSchema,
  GameSessionBlockedReason,
  GameSessionBlockedSchema,
} from '../../../gen/meurpg/play/v1/play_pb';
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
  it('maps the wire vitals, hit dice as "2d10 + 1d8"', () => {
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
        spellSlots: [{ level: 1, total: 2, used: 1 }],
        pactSlots: { slotLevel: 2, total: 2, used: 0 },
        revision: 7,
      }),
    );
    expect(vm.hitDice).toBe('2d10 + 1d8');
    expect(vm.spellSlots).toEqual([{ level: 1, total: 2, used: 1 }]);
    expect(vm.pactSlots).toEqual({ slotLevel: 2, total: 2, used: 0 });
    expect(vm.revision).toBe(7);
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

  it('maps `reaction_window_opened` and `reaction_window_closed` to their own events (PM-04)', async () => {
    const out = [];
    const responses = [
      create(WatchGameSessionResponseSchema, { event: { case: 'ready', value: {} } }),
      create(WatchGameSessionResponseSchema, {
        event: { case: 'reactionWindowOpened', value: { encounterId: 'e1', windowId: 'w1' } },
      }),
      create(WatchGameSessionResponseSchema, {
        event: {
          case: 'reactionWindowClosed',
          value: {
            encounterId: 'e1',
            windowId: 'w1',
            closedByItself: true,
            textPt: 'Repreensão Infernal fechou. Você está inconsciente e não pode reagir.',
          },
        },
      }),
    ];
    for await (const e of sourceAnswering(responses).watch(
      'camp-1',
      new AbortController().signal,
    )) {
      out.push(e);
    }
    expect(out).toEqual([
      { kind: 'ready' },
      { kind: 'reactionWindowOpened', encounterId: 'e1', windowId: 'w1' },
      {
        kind: 'reactionWindowClosed',
        encounterId: 'e1',
        windowId: 'w1',
        closedByItself: true,
        text: 'Repreensão Infernal fechou. Você está inconsciente e não pode reagir.',
      },
    ]);
  });

  it('maps `content_changed` to its own event, with no content (RN-23, RN-10)', async () => {
    expect(
      await events([
        create(WatchGameSessionResponseSchema, { event: { case: 'ready', value: {} } }),
        create(WatchGameSessionResponseSchema, { event: { case: 'contentChanged', value: {} } }),
      ]),
    ).toEqual(['ready', 'contentChanged']);
  });

  it('maps the two hints of a character to one event with the character, and the revival to its own', async () => {
    expect(
      await events([
        create(WatchGameSessionResponseSchema, {
          event: { case: 'characterChangesRequested', value: { characterId: 'c-1' } },
        }),
        create(WatchGameSessionResponseSchema, {
          event: { case: 'characterResubmitted', value: { characterId: 'c-2' } },
        }),
        create(WatchGameSessionResponseSchema, {
          event: { case: 'characterRevived', value: { characterId: 'c-3' } },
        }),
      ]),
    ).toEqual(['characterChanged', 'characterChanged', 'characterRevived']);
  });

  it('maps `revivify_changed` to its own event, with no content (RN-10)', async () => {
    expect(
      await events([
        create(WatchGameSessionResponseSchema, { event: { case: 'revivifyChanged', value: {} } }),
      ]),
    ).toEqual(['revivifyChanged']);
  });

  it('still takes an event it does not know as a sign the stream is alive', async () => {
    // An empty `event` is what a newer server's oneof case looks like to this app.
    expect(await events([create(WatchGameSessionResponseSchema, {})])).toEqual(['heartbeat']);
  });
});

describe('LiveSessionSourceLive.getPlayerSheet', () => {
  function sheetWith(spells: { key: string; prepared: boolean }[]) {
    const characters = {
      getCharacter: async () => ({
        character: {
          derived: {
            classes: [{ namePt: 'Clérigo', level: 5 }],
            subraceNamePt: '',
            raceNamePt: 'Humano',
            skills: [],
            senses: [],
            spells: spells.map((s) => ({ spell: { key: s.key }, prepared: s.prepared })),
          },
        },
      }),
    };
    TestBed.configureTestingModule({
      providers: [LiveSessionSourceLive, { provide: CONNECT_TRANSPORT, useValue: {} }],
    });
    const source = TestBed.inject(LiveSessionSourceLive);
    (source as unknown as { characters: unknown }).characters = characters;
    return source;
  }

  it('says the character has Revivificar only when it is ready today, and gives the class line', async () => {
    const ready = await sheetWith([{ key: 'spell:revivify', prepared: true }]).getPlayerSheet(
      'c',
      'x',
    );
    expect(ready.revivify).toBe(true);
    expect(ready.classes).toBe('Clérigo 5');
    expect(ready.summary).toBe('Clérigo 5, Humano');
  });

  it('does not offer Revivificar for a spell on the sheet that is not prepared, nor for other spells', async () => {
    const notReady = await sheetWith([{ key: 'spell:revivify', prepared: false }]).getPlayerSheet(
      'c',
      'x',
    );
    expect(notReady.revivify).toBe(false);
    TestBed.resetTestingModule();
    const other = await sheetWith([{ key: 'spell:bless', prepared: true }]).getPlayerSheet(
      'c',
      'x',
    );
    expect(other.revivify).toBe(false);
  });
});
