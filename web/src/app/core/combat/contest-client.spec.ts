import { TestBed } from '@angular/core/testing';

import {
  ContestKind,
  ContestPurpose,
  ContestSkill,
  HelpKind,
  ShoveOutcome,
} from '../../../gen/meurpg/play/v1/contest_types_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';
import { encounter } from './combat-testing';
import { ContestClient, rollInput } from './contest-client';
import { contestView, groupCheck, hideAttempt, helpView } from './contest-testing';

// The player's calls of a contest: each sends the request the contract names, with the key the caller made, and hands back the
// combat and what the call answered.

type Sent = Record<string, unknown>;

function clientWith(): { client: ContestClient; sent: Sent[] } {
  TestBed.configureTestingModule({
    providers: [ContestClient, { provide: CONNECT_TRANSPORT, useValue: {} }],
  });
  const client = TestBed.inject(ContestClient);
  const sent: Sent[] = [];
  const answer = (name: string, res: object) => (req: Sent) => {
    sent.push({ rpc: name, ...req });
    return Promise.resolve(res);
  };
  (client as unknown as { client: unknown }).client = {
    getContestState: answer('getContestState', { contests: [contestView()] }),
    startContest: answer('startContest', { encounter: encounter(), contest: contestView() }),
    respondContest: answer('respondContest', { encounter: encounter(), contest: contestView() }),
    resolveShove: answer('resolveShove', { encounter: encounter(), contest: contestView() }),
    hide: answer('hide', { encounter: encounter(), attempt: hideAttempt() }),
    help: answer('help', { encounter: encounter(), help: helpView() }),
    getGroupCheck: answer('getGroupCheck', { groupCheck: groupCheck() }),
    rollGroupCheck: answer('rollGroupCheck', { groupCheck: groupCheck() }),
  };
  return { client, sent };
}

describe('ContestClient', () => {
  it('says the roll as the contract does: the app rolls, or the typed faces', () => {
    expect(rollInput({ inApp: true })).toEqual({ roll: { case: 'rollInApp', value: true } });
    expect(rollInput({ faces: [15, 8] })).toEqual({
      roll: { case: 'd20Faces', value: { faces: [15, 8] } },
    });
  });

  it('starts a grapple with the key of the caller and the roll in the app', async () => {
    const { client, sent } = clientWith();
    const res = await client.start(
      {
        campaignId: 'c',
        encounterId: 'e',
        initiatorId: 't',
        targetId: 'h',
        purpose: ContestPurpose.GRAPPLE,
        kind: ContestKind.CONTEST,
        skill: ContestSkill.ATHLETICS,
        die: { inApp: true },
      },
      'key-1',
    );
    expect(res.contest.id).toBe('ct1');
    expect(sent[0]).toMatchObject({
      rpc: 'startContest',
      campaignId: 'c',
      encounterId: 'e',
      idempotencyKey: 'key-1',
      initiatorId: 't',
      targetId: 'h',
      purpose: ContestPurpose.GRAPPLE,
      kind: ContestKind.CONTEST,
      skill: ContestSkill.ATHLETICS,
      roll: { roll: { case: 'rollInApp', value: true } },
    });
  });

  it('answers a contest with the skill and the typed d20, or leaves the roll to the master', async () => {
    const { client, sent } = clientWith();
    const base = { campaignId: 'c', encounterId: 'e', contestId: 'ct1', skill: ContestSkill.ACROBATICS };
    await client.respond({ ...base, die: { faces: [8] } }, 'k2');
    await client.respond({ ...base, die: null }, 'k3');
    expect(sent[0]).toMatchObject({
      rpc: 'respondContest',
      idempotencyKey: 'k2',
      contestId: 'ct1',
      skill: ContestSkill.ACROBATICS,
      deferToMaster: false,
      roll: { roll: { case: 'd20Faces', value: { faces: [8] } } },
    });
    expect(sent[1]).toMatchObject({ idempotencyKey: 'k3', deferToMaster: true });
    expect(sent[1]['roll']).toBeUndefined();
  });

  it('chooses what a won shove does, never the square', async () => {
    const { client, sent } = clientWith();
    await client.resolveShove('c', 'e', 'ct1', ShoveOutcome.PUSH, 'k4');
    expect(sent[0]).toEqual({
      rpc: 'resolveShove',
      campaignId: 'c',
      encounterId: 'e',
      idempotencyKey: 'k4',
      contestId: 'ct1',
      outcome: ShoveOutcome.PUSH,
    });
  });

  it('hides with the action that gives Hide and the roll', async () => {
    const { client, sent } = clientWith();
    const res = await client.hide('c', 'e', 'b', 'standard:hide', { inApp: true }, 'k5');
    expect(res.attempt.id).toBe('hd1');
    expect(sent[0]).toMatchObject({
      rpc: 'hide',
      combatantId: 'b',
      actionKey: 'standard:hide',
      idempotencyKey: 'k5',
    });
  });

  it('helps an ally with a task, or aims the help at a creature', async () => {
    const { client, sent } = clientWith();
    await client.help(
      {
        campaignId: 'c',
        encounterId: 'e',
        helperId: 'o',
        kind: HelpKind.CHECK,
        allyId: 'v',
        taskKey: 'skill:perception',
        targetId: '',
      },
      'k6',
    );
    expect(sent[0]).toMatchObject({
      rpc: 'help',
      combatantId: 'o',
      kind: HelpKind.CHECK,
      allyId: 'v',
      taskKey: 'skill:perception',
      idempotencyKey: 'k6',
    });
  });

  it('reads the contests and the group check, and rolls the group check once', async () => {
    const { client, sent } = clientWith();
    expect((await client.state('c', 'e')).contests).toHaveLength(1);
    expect((await client.groupCheck('c'))?.id).toBe('gc1');
    await client.rollGroupCheck('c', 'gc1', { faces: [12] }, 'k7');
    expect(sent.map((s) => s['rpc'])).toEqual(['getContestState', 'getGroupCheck', 'rollGroupCheck']);
    expect(sent[2]).toMatchObject({
      groupCheckId: 'gc1',
      idempotencyKey: 'k7',
      roll: { roll: { case: 'd20Faces', value: { faces: [12] } } },
    });
  });
});
