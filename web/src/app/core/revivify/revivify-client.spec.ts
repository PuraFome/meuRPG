import { TestBed } from '@angular/core/testing';

import { CONNECT_TRANSPORT } from '../connect/transport';
import { RevivifyClient } from './revivify-client';

describe('RevivifyClient', () => {
  function clientWith(fake: Record<string, (req: unknown) => unknown>): RevivifyClient {
    TestBed.configureTestingModule({
      providers: [RevivifyClient, { provide: CONNECT_TRANSPORT, useValue: {} }],
    });
    const client = TestBed.inject(RevivifyClient);
    (client as unknown as { revivify: unknown }).revivify = fake;
    return client;
  }

  it('previews a combat by its caster combatant and the outside by its character', async () => {
    const sent: unknown[] = [];
    const client = clientWith({
      previewRevivify: (req) => {
        sent.push(req);
        return { targets: [] };
      },
    });
    await client.preview('c', { encounterId: 'e', casterId: 'i' });
    await client.preview('c', { casterCharacterId: 'ilaria' });
    expect(sent).toEqual([
      { campaignId: 'c', encounterId: 'e', casterId: 'i' },
      { campaignId: 'c', casterCharacterId: 'ilaria' },
    ]);
  });

  it('requests the cast with the diamonds confirmed, the slot and the caller key', async () => {
    let sent: unknown;
    const client = clientWith({
      requestRevivify: (req) => {
        sent = req;
        return { request: { id: 'r' } };
      },
    });
    const request = await client.request('c', 'ilaria', 'toren', { level: 3, pact: false }, 'k1');
    expect(request?.id).toBe('r');
    expect(sent).toEqual({
      campaignId: 'c',
      casterCharacterId: 'ilaria',
      targetCharacterId: 'toren',
      slot: { level: 3, pact: false },
      materialConfirmed: true,
      idempotencyKey: 'k1',
    });
  });
});
