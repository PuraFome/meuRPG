import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';
import { Code, ConnectError, createRouterTransport } from '@connectrpc/connect';

import {
  GameSessionBlockedReason,
  GameSessionBlockedSchema,
  PlayService,
} from '../../../gen/meurpg/play/v1/play_pb';
import {
  GetNpcCheckResponseSchema,
  ListNpcCheckPickerResponseSchema,
  type RollNpcCheckRequest,
  RollNpcCheckResponseSchema,
  SkipNpcCheckAsksResponseSchema,
} from '../../../gen/meurpg/play/v1/scene_pb';
import {
  AnswerHeldRollResponseSchema,
  CuttingWordsAnsweredSchema,
  ResourceService,
  type AnswerHeldRollRequest,
} from '../../../gen/meurpg/play/v1/resources_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';
import { NpcCheckClient } from './npc-check-client';
import { candidate, npcCheckRead } from './npc-check-testing';

interface Seen {
  roll?: RollNpcCheckRequest;
  answer?: AnswerHeldRollRequest;
  picker?: string;
  skip?: string;
}

function setup(over: { latest?: () => unknown; answer?: () => unknown } = {}): { client: NpcCheckClient; seen: Seen } {
  TestBed.resetTestingModule();
  const seen: Seen = {};
  const transport = createRouterTransport(({ service }) => {
    service(PlayService, {
      listNpcCheckPicker: (req) => {
        seen.picker = req.monsterKey;
        return create(ListNpcCheckPickerResponseSchema, { candidates: [candidate()] });
      },
      rollNpcCheck: (req) => {
        seen.roll = req;
        return create(RollNpcCheckResponseSchema, { check: npcCheckRead() });
      },
      skipNpcCheckAsks: (req) => {
        seen.skip = req.holdId;
        return create(SkipNpcCheckAsksResponseSchema, { check: npcCheckRead({ settled: true }) });
      },
      getNpcCheck: () => {
        if (over.latest) {
          return over.latest() as never;
        }
        return create(GetNpcCheckResponseSchema, { check: npcCheckRead() });
      },
    });
    service(ResourceService, {
      answerHeldRoll: (req) => {
        seen.answer = req;
        return (over.answer?.() ??
          create(AnswerHeldRollResponseSchema, {
            result: { case: 'cuttingWords', value: create(CuttingWordsAnsweredSchema, { cut: req.answer.case === 'cuttingWords' && req.answer.value.cut, usesLeft: 2 }) },
          })) as never;
      },
    });
  });
  TestBed.configureTestingModule({ providers: [{ provide: CONNECT_TRANSPORT, useValue: transport }] });
  return { client: TestBed.inject(NpcCheckClient), seen };
}

describe('NpcCheckClient', () => {
  it('lists the creatures, asking for the SRD creature the master searched', async () => {
    const { client, seen } = setup();
    expect(await client.picker('c1', 'goblin')).toHaveLength(1);
    expect(seen.picker).toBe('goblin');
  });

  it('rolls an NPC of the table in the app, with the box and the key', async () => {
    const { client, seen } = setup();
    await client.roll('c1', { characterId: 'npc-1', monsterKey: '', checkKey: 'skill:deception', die: { inApp: true }, bardSees: true }, 'k1');
    expect(seen.roll?.npc).toEqual({ case: 'npcCharacterId', value: 'npc-1' });
    expect(seen.roll?.roll).toEqual({ case: 'rollInApp', value: true });
    expect(seen.roll?.bardSees).toBe(true);
    expect(seen.roll?.idempotencyKey).toBe('k1');
    expect(seen.roll?.checkKey).toBe('skill:deception');
  });

  it('rolls a creature of the bestiary with the typed d20', async () => {
    const { client, seen } = setup();
    await client.roll('c1', { characterId: '', monsterKey: 'spy', checkKey: 'skill:stealth', die: { face: 11 }, bardSees: false }, 'k2');
    expect(seen.roll?.npc).toEqual({ case: 'monsterKey', value: 'spy' });
    expect(seen.roll?.roll).toEqual({ case: 'd20Face', value: 11 });
  });

  it('skips the asks of the hold', async () => {
    const { client, seen } = setup();
    expect((await client.skip('c1', 'hold-1', 'k3')).settled).toBe(true);
    expect(seen.skip).toBe('hold-1');
  });

  it('reads the latest check, and reads none when no session is open or the caller is not the master', async () => {
    expect((await setup().client.latest('c1'))?.holdId).toBe('hold-1');
    const closed = new ConnectError('x', Code.FailedPrecondition, undefined, [
      { desc: GameSessionBlockedSchema, value: create(GameSessionBlockedSchema, { reason: GameSessionBlockedReason.NO_OPEN_SESSION }) },
    ]);
    expect(await setup({ latest: () => { throw closed; } }).client.latest('c1')).toBeNull();
    expect(await setup({ latest: () => { throw new ConnectError('x', Code.PermissionDenied); } }).client.latest('c1')).toBeNull();
    await expect(setup({ latest: () => { throw new ConnectError('x', Code.Unavailable); } }).client.latest('c1')).rejects.toThrow();
  });

  it('answers a cut with the app die or a typed face, and a pass with no die', async () => {
    const { client, seen } = setup();
    await client.answer('c1', 'hold-1', { cut: true, die: { inApp: true } }, 'k4');
    expect(seen.answer?.answer).toMatchObject({ case: 'cuttingWords', value: { cut: true, roll: { case: 'rollInApp', value: true } } });
    await client.answer('c1', 'hold-1', { cut: true, die: { face: 5 } }, 'k5');
    expect(seen.answer?.answer).toMatchObject({ value: { roll: { case: 'typedFace', value: 5 } } });
    const read = await client.answer('c1', 'hold-1', { cut: false }, 'k6');
    expect(read.cut).toBe(false);
    expect(seen.answer?.answer).toMatchObject({ value: { cut: false, roll: { case: undefined } } });
  });

  it('refuses an answer that is not the bard\'s', async () => {
    const { client } = setup({ answer: () => create(AnswerHeldRollResponseSchema, {}) });
    await expect(client.answer('c1', 'hold-1', { cut: false }, 'k7')).rejects.toThrow(/another result/);
  });
});
