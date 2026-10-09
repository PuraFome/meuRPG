import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import {
  CharacterVitalsSchema,
  WatchGameSessionResponseSchema,
  GameSessionBlockedReason,
  GameSessionBlockedSchema,
} from '../../../gen/meurpg/play/v1/play_pb';
import { TestBed } from '@angular/core/testing';
import { GetCharacterResponseSchema } from '../../../gen/meurpg/characters/v1/characters_pb';
import { ProficiencyLevel } from '../../../gen/meurpg/rules/v1/rules_pb';
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

  it('keeps what Ajuda adds to the maximum, which the maximum already counts', () => {
    const vm = toVitalsVm(
      create(CharacterVitalsSchema, {
        characterId: 'c1',
        hitPointsCurrent: 43,
        hitPointsMax: 43,
        hitPointsMaxBonus: 5,
      }),
    );
    expect(vm.hitPointsMaxBonus).toBe(5);
    expect(vm.hitPointsMax).toBe(43);
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

  it('maps `spell_casts_changed` to its own event, with no content (MR-048)', async () => {
    expect(
      await events([
        create(WatchGameSessionResponseSchema, { event: { case: 'ready', value: {} } }),
        create(WatchGameSessionResponseSchema, { event: { case: 'spellCastsChanged', value: {} } }),
      ]),
    ).toEqual(['ready', 'spellCastsChanged']);
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

  it('still takes an event it does not know as a sign the stream is alive', async () => {
    // An empty `event` is what a newer server's oneof case looks like to this app.
    expect(await events([create(WatchGameSessionResponseSchema, {})])).toEqual(['heartbeat']);
  });
});

describe('LiveSessionSourceLive.getPlayerSheet (the skills a search and a check read)', () => {
  const skill = (key: string, namePt: string, bonus: number, proficiency: ProficiencyLevel) => ({
    key,
    namePt,
    bonus,
    proficiency,
  });

  function sourceWith(features: string[]) {
    const transport = {
      unary: async (method: { name: string }) => ({
        stream: false,
        service: {},
        method,
        header: new Headers(),
        trailer: new Headers(),
        message: create(GetCharacterResponseSchema, {
          character: {
            derived: {
              armorClass: 13,
              features: features.map((key) => ({ key })),
              skills: [
                skill('skill:perception', 'Percepção', 1, ProficiencyLevel.NONE),
                skill('skill:investigation', 'Investigação', 8, ProficiencyLevel.PROFICIENT),
                skill('skill:arcana', 'Arcanismo', 8, ProficiencyLevel.PROFICIENT),
                skill('skill:acrobatics', 'Acrobacia', 1, ProficiencyLevel.NONE),
                skill('skill:athletics', 'Atletismo', -1, ProficiencyLevel.NONE),
                skill('skill:stealth', 'Furtividade', 5, ProficiencyLevel.EXPERTISE),
              ],
            },
          },
        }),
      }),
    } as unknown as Transport;
    TestBed.configureTestingModule({
      providers: [LiveSessionSourceLive, { provide: CONNECT_TRANSPORT, useValue: transport }],
    });
    return TestBed.inject(LiveSessionSourceLive);
  }

  it('lists the other skills with the sheet bonus, alphabetical, without Percepção and Investigação', async () => {
    const sheet = await sourceWith([]).getPlayerSheet('camp', 'c1');
    expect(sheet.skills?.perception).toBe(1);
    expect(sheet.skills?.investigation).toBe(8);
    expect(sheet.skills?.others).toEqual([
      { key: 'skill:acrobatics', name: 'Acrobacia', bonus: 1 },
      { key: 'skill:arcana', name: 'Arcanismo', bonus: 8 },
      { key: 'skill:athletics', name: 'Atletismo', bonus: -1 },
      { key: 'skill:stealth', name: 'Furtividade', bonus: 5 },
    ]);
  });

  it('names the skills Talento Confiável raises (the proficient ones) only for a sheet that has the feature', async () => {
    expect((await sourceWith([]).getPlayerSheet('camp', 'c1')).skills?.reliableTalent).toEqual([]);
    TestBed.resetTestingModule();
    const rogue = await sourceWith(['feature:reliable-talent']).getPlayerSheet('camp', 'c1');
    expect(rogue.skills?.reliableTalent).toEqual([
      'skill:investigation',
      'skill:arcana',
      'skill:stealth',
    ]);
  });
});
