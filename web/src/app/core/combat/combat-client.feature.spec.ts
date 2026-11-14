import { TestBed } from '@angular/core/testing';

import { CONNECT_TRANSPORT } from '../connect/transport';
import { CombatClient } from './combat-client';

function fake(answer: Record<string, unknown>) {
  const calls: Record<string, unknown>[] = [];
  TestBed.configureTestingModule({
    providers: [CombatClient, { provide: CONNECT_TRANSPORT, useValue: {} }],
  });
  const client = TestBed.inject(CombatClient);
  (client.features as unknown as { client: unknown }).client = {
    useFeature: (req: Record<string, unknown>) => {
      calls.push(req);
      return Promise.resolve(answer);
    },
  };
  return { client, calls };
}

describe('CombatClient, using a feature', () => {
  it('sends the feature, the targets, the choice and the key it is given, and answers what the use gave', async () => {
    const { client, calls } = fake({ encounter: { id: 'enc' }, healed: 4, tempHp: 16 });
    const res = await client.features.useFeature(
      {
        campaignId: 'c',
        encounterId: 'e',
        combatantId: 'p1',
        actionKey: 'feature:symbiotic-entity',
        targetIds: ['z1'],
        choice: 'fire',
      },
      'k1',
    );
    expect(calls).toEqual([
      {
        campaignId: 'c',
        encounterId: 'e',
        combatantId: 'p1',
        actionKey: 'feature:symbiotic-entity',
        targetIds: ['z1'],
        choice: 'fire',
        idempotencyKey: 'k1',
      },
    ]);
    expect(res.encounter.id).toBe('enc');
    expect(res.healed).toBe(4);
    expect(res.tempHp).toBe(16);
  });

  it('sends no target and no choice when none was picked', async () => {
    const { client, calls } = fake({ encounter: { id: 'enc' }, healed: 0, tempHp: 0 });
    await client.features.useFeature(
      { campaignId: 'c', encounterId: 'e', combatantId: 'p1', actionKey: 'feature:x' },
      'k2',
    );
    expect(calls[0]).toMatchObject({ targetIds: [], choice: '', idempotencyKey: 'k2' });
  });

  it('fails when the server answers without the combat', async () => {
    const { client } = fake({ healed: 0, tempHp: 0 });
    await expect(
      client.features.useFeature(
        { campaignId: 'c', encounterId: 'e', combatantId: 'p1', actionKey: 'feature:x' },
        'k3',
      ),
    ).rejects.toThrow(/UseFeature/);
  });
});
