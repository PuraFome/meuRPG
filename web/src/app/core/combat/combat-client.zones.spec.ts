import { TestBed } from '@angular/core/testing';
import { ConnectError, Code } from '@connectrpc/connect';

import { CONNECT_TRANSPORT } from '../connect/transport';
import { CombatClient } from './combat-client';

interface Req {
  readonly zoneId: string;
  readonly idempotencyKey: string;
  readonly origin?: { col: number; row: number };
  readonly campaignId: string;
  readonly encounterId: string;
}

function setup(answers: (Error | null)[] = []) {
  TestBed.configureTestingModule({
    providers: [CombatClient, { provide: CONNECT_TRANSPORT, useValue: {} }],
  });
  const client = TestBed.inject(CombatClient);
  const moves: Req[] = [];
  const ends: Req[] = [];
  const answer = (into: Req[]) => (req: Req) => {
    into.push(req);
    const failure = answers.shift();
    return failure ? Promise.reject(failure) : Promise.resolve({ encounter: { id: 'enc' } });
  };
  (client as unknown as { base: { client: unknown } }).base.client = {
    moveSpellZone: answer(moves),
    endSpellZone: answer(ends),
  };
  return { client, moves, ends };
}

describe('CombatClient spell zones', () => {
  it('moves a zone to a square and returns the encounter', async () => {
    const { client, moves } = setup();
    const e = await client.casts.moveSpellZone('c', 'e', 'z1', { col: 4, row: 5 });
    expect(e.id).toBe('enc');
    expect(moves[0]).toMatchObject({
      campaignId: 'c',
      encounterId: 'e',
      zoneId: 'z1',
      origin: { col: 4, row: 5 },
    });
    expect(moves[0].idempotencyKey).not.toBe('');
  });

  it('keeps the key of the same move after a lost answer, and renews it after success', async () => {
    const { client, moves } = setup([new ConnectError('lost', Code.Unavailable)]);
    await expect(
      client.casts.moveSpellZone('c', 'e', 'z1', { col: 4, row: 5 }),
    ).rejects.toBeInstanceOf(ConnectError);
    await client.casts.moveSpellZone('c', 'e', 'z1', { col: 4, row: 5 });
    await client.casts.moveSpellZone('c', 'e', 'z1', { col: 4, row: 5 });
    expect(moves[1].idempotencyKey).toBe(moves[0].idempotencyKey);
    expect(moves[2].idempotencyKey).not.toBe(moves[1].idempotencyKey);
  });

  it('takes a new key for another square', async () => {
    const { client, moves } = setup([new ConnectError('lost', Code.Unavailable)]);
    await expect(
      client.casts.moveSpellZone('c', 'e', 'z1', { col: 4, row: 5 }),
    ).rejects.toBeInstanceOf(ConnectError);
    await client.casts.moveSpellZone('c', 'e', 'z1', { col: 5, row: 5 });
    expect(moves[1].idempotencyKey).not.toBe(moves[0].idempotencyKey);
  });

  it('ends a zone with a key kept across retries and renewed after success', async () => {
    const { client, ends } = setup([new ConnectError('lost', Code.Unavailable)]);
    await expect(client.casts.endSpellZone('c', 'e', 'z1')).rejects.toBeInstanceOf(ConnectError);
    await client.casts.endSpellZone('c', 'e', 'z1');
    await client.casts.endSpellZone('c', 'e', 'z1');
    expect(ends[0]).toMatchObject({ campaignId: 'c', encounterId: 'e', zoneId: 'z1' });
    expect(ends[1].idempotencyKey).toBe(ends[0].idempotencyKey);
    expect(ends[2].idempotencyKey).not.toBe(ends[1].idempotencyKey);
  });
});
