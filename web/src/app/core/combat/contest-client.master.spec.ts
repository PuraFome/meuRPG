import { TestBed } from '@angular/core/testing';

import {
  ContestKind,
  ContestPurpose,
  ContestSkill,
} from '../../../gen/meurpg/play/v1/contest_types_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';
import { encounter } from './combat-testing';
import { ContestClient } from './contest-client';
import { contestView, groupCheck, hideAttempt } from './contest-testing';

// The master's calls of a contest: each sends the request the contract names, with the key the caller made.

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
    startContest: answer('startContest', { encounter: encounter(), contest: contestView() }),
    closeContest: answer('closeContest', { encounter: encounter() }),
    releaseGrapple: answer('releaseGrapple', { encounter: encounter() }),
    resolveHide: answer('resolveHide', { encounter: encounter(), attempt: hideAttempt() }),
    clearHelp: answer('clearHelp', { encounter: encounter() }),
    setSurprised: answer('setSurprised', { encounter: encounter(), surprise: undefined }),
    getSurpriseSuggestion: answer('getSurpriseSuggestion', {
      suggestions: [{ combatantId: 'g1' }],
    }),
    requestGroupCheck: answer('requestGroupCheck', { groupCheck: groupCheck() }),
    rollForPlayer: answer('rollForPlayer', { groupCheck: groupCheck() }),
    closeGroupCheck: answer('closeGroupCheck', { groupCheck: groupCheck() }),
  };
  return { client, sent };
}

describe("ContestClient, the master's calls", () => {
  it("starts a creature's grapple with a fixed escape DC and no roll", async () => {
    const { client, sent } = clientWith();
    await client.start(
      {
        campaignId: 'c',
        encounterId: 'e',
        initiatorId: 'cobra',
        targetId: 'b',
        purpose: ContestPurpose.GRAPPLE,
        kind: ContestKind.ESCAPE_DC,
        skill: ContestSkill.ATHLETICS,
        escapeDc: 16,
      },
      'k1',
    );
    expect(sent[0]).toMatchObject({
      rpc: 'startContest',
      idempotencyKey: 'k1',
      kind: ContestKind.ESCAPE_DC,
      escapeDc: 16,
    });
    expect(sent[0]['roll']).toBeUndefined();
  });

  it('closes a contest, releases a grapple and takes a help back, each with its key', async () => {
    const { client, sent } = clientWith();
    await client.closeContest('c', 'e', 'ct1', 'k1');
    await client.releaseGrapple('c', 'e', 'b', 'k2');
    await client.clearHelp('c', 'e', 'hp1', 'k3');
    expect(sent).toMatchObject([
      { rpc: 'closeContest', contestId: 'ct1', idempotencyKey: 'k1' },
      { rpc: 'releaseGrapple', grappledId: 'b', idempotencyKey: 'k2' },
      { rpc: 'clearHelp', helpId: 'hp1', idempotencyKey: 'k3' },
    ]);
  });

  it('decides a Hide: applied with the ones that see clearly, or refused with the reason', async () => {
    const { client, sent } = clientWith();
    await client.resolveHide(
      'c',
      'e',
      { attemptId: 'hd1', refuse: false, refusal: '', seesClearlyIds: ['cap'] },
      'k1',
    );
    await client.resolveHide(
      'c',
      'e',
      { attemptId: 'hd1', refuse: true, refusal: 'Sem sombra aqui.', seesClearlyIds: [] },
      'k2',
    );
    expect(sent).toMatchObject([
      { rpc: 'resolveHide', attemptId: 'hd1', refuse: false, seesClearlyIds: ['cap'] },
      { rpc: 'resolveHide', refuse: true, refusal: 'Sem sombra aqui.' },
    ]);
  });

  it('marks surprise and reads the suggestion', async () => {
    const { client, sent } = clientWith();
    await client.setSurprised('c', 'e', 'nael', true, 'k1');
    const list = await client.surpriseSuggestion('c', 'e');
    expect(list).toHaveLength(1);
    expect(sent).toMatchObject([
      { rpc: 'setSurprised', combatantId: 'nael', surprised: true, idempotencyKey: 'k1' },
      { rpc: 'getSurpriseSuggestion', encounterId: 'e' },
    ]);
  });

  it('asks for a group check, rolls for a player and closes it', async () => {
    const { client, sent } = clientWith();
    await client.requestGroupCheck('c', { skillKey: 'skill:stealth', dc: 13, showDc: true }, 'k1');
    await client.rollForPlayer('c', 'gc1', 'ragna', { faces: [14] }, 'k2');
    await client.closeGroupCheck('c', 'gc1', 'k3');
    expect(sent).toMatchObject([
      { rpc: 'requestGroupCheck', skillKey: 'skill:stealth', dc: 13, showDc: true },
      {
        rpc: 'rollForPlayer',
        groupCheckId: 'gc1',
        characterId: 'ragna',
        roll: { roll: { case: 'd20Faces', value: { faces: [14] } } },
      },
      { rpc: 'closeGroupCheck', groupCheckId: 'gc1', idempotencyKey: 'k3' },
    ]);
  });
});
