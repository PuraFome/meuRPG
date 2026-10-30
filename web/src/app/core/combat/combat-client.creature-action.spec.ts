import { TestBed } from '@angular/core/testing';
import { Code, ConnectError } from '@connectrpc/connect';

import { EncounterBlockedReason } from '../../../gen/meurpg/play/v1/combat_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';
import { CombatClient } from './combat-client';
import { blockedMessage } from './combat-errors';

function withRequests() {
  TestBed.configureTestingModule({
    providers: [CombatClient, { provide: CONNECT_TRANSPORT, useValue: {} }],
  });
  const client = TestBed.inject(CombatClient);
  const requests: Record<string, unknown>[] = [];
  const answers: (Error | null)[] = [];
  (client as unknown as { base: { client: unknown } }).base.client = {
    useCreatureAction: (req: Record<string, unknown>) => {
      requests.push(req);
      const failure = answers.shift();
      return failure
        ? Promise.reject(failure)
        : Promise.resolve({
            encounter: { id: 'enc' },
            result: { castId: 'cast', targets: [] },
            area: { origin: { col: 6, row: 4 }, squares: [{ col: 6, row: 4 }] },
          });
    },
  };
  return { client, requests, answers };
}

describe('CombatClient.useCreatureAction', () => {
  it('names the NPC, the action and the targets, and reads where an area landed', async () => {
    const { client, requests } = withRequests();
    const res = await client.casts.useCreatureAction('c', 'e', 'hound', 'm#fire-breath', [
      't1',
      't2',
    ]);
    expect(requests[0]).toMatchObject({
      campaignId: 'c',
      encounterId: 'e',
      combatantId: 'hound',
      actionKey: 'm#fire-breath',
      targetIds: ['t1', 't2'],
      area: { case: undefined },
    });
    expect(requests[0]['idempotencyKey']).toEqual(expect.any(String));
    expect(res.result.castId).toBe('cast');
    expect(res.area?.origin).toEqual({ col: 6, row: 4 });
  });

  it('places a cone with a direction and a sphere with a point', async () => {
    const { client, requests } = withRequests();
    await client.casts.useCreatureAction('c', 'e', 'hound', 'm#a', [], {
      direction: { dx: 1, dy: 0 },
    });
    await client.casts.useCreatureAction('c', 'e', 'hound', 'm#b', [], {
      origin: { col: 3, row: 2 },
    });
    expect(requests[0]['area']).toEqual({ case: 'direction', value: { dx: 1, dy: 0 } });
    expect(requests[1]['area']).toEqual({ case: 'origin', value: { col: 3, row: 2 } });
  });

  it('keeps the key on a retry of the same use, and takes a new one for another action', async () => {
    const { client, requests, answers } = withRequests();
    answers.push(new ConnectError('lost', Code.Unavailable), null, null);
    await expect(
      client.casts.useCreatureAction('c', 'e', 'hound', 'm#a', ['t1']),
    ).rejects.toBeInstanceOf(ConnectError);
    await client.casts.useCreatureAction('c', 'e', 'hound', 'm#a', ['t1']);
    await client.casts.useCreatureAction('c', 'e', 'hound', 'm#b', ['t1']);
    expect(requests[1]['idempotencyKey']).toBe(requests[0]['idempotencyKey']);
    expect(requests[2]['idempotencyKey']).not.toBe(requests[0]['idempotencyKey']);
  });
});

describe('the refusal of a creature action', () => {
  it('says the creature has no such action, or that it is made with the sheet attack', () => {
    expect(
      blockedMessage({ reason: EncounterBlockedReason.CREATURE_ACTION_UNAVAILABLE } as never),
    ).toBe('A criatura não tem essa ação, ou ela se faz com ataque: use o ataque da ficha.');
  });
});
