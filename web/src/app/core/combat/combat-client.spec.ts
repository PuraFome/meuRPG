import { TestBed } from '@angular/core/testing';
import { ConnectError, Code } from '@connectrpc/connect';

import { CombatEffect, JumpKind } from '../../../gen/meurpg/play/v1/combat_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';
import { CombatClient } from './combat-client';

// A change the combat sends carries the key of its request: a retry after a lost answer reuses it (the server
// answers with what the first call did), other values take a new one, and a worked change frees the key.

interface Sent {
  readonly endTurn: { idempotencyKey: string; expectedCombatantId: string }[];
  readonly answers: (Error | null)[];
}

function clientWith(sent: Sent): CombatClient {
  TestBed.configureTestingModule({
    providers: [CombatClient, { provide: CONNECT_TRANSPORT, useValue: {} }],
  });
  const client = TestBed.inject(CombatClient);
  (client as unknown as { client: unknown }).client = {
    endTurn: (req: { idempotencyKey: string; expectedCombatantId: string }) => {
      sent.endTurn.push(req);
      const failure = sent.answers.shift();
      return failure ? Promise.reject(failure) : Promise.resolve({ encounter: { id: 'enc' } });
    },
  };
  return client;
}

describe('CombatClient keys', () => {
  it('sends the same key when the same change is tried again after a lost answer', async () => {
    const sent: Sent = { endTurn: [], answers: [new ConnectError('lost', Code.Unavailable), null] };
    const client = clientWith(sent);
    await expect(client.endTurn('c', 'e', 'a')).rejects.toBeInstanceOf(ConnectError);
    await client.endTurn('c', 'e', 'a');
    expect(sent.endTurn).toHaveLength(2);
    expect(sent.endTurn[1].idempotencyKey).toBe(sent.endTurn[0].idempotencyKey);
  });

  it('takes a new key for other values', async () => {
    const sent: Sent = { endTurn: [], answers: [new ConnectError('lost', Code.Unavailable), null] };
    const client = clientWith(sent);
    await expect(client.endTurn('c', 'e', 'a')).rejects.toBeInstanceOf(ConnectError);
    await client.endTurn('c', 'e', 'b');
    expect(sent.endTurn[1].idempotencyKey).not.toBe(sent.endTurn[0].idempotencyKey);
  });

  it('takes a new key for the same values once the change worked', async () => {
    const sent: Sent = { endTurn: [], answers: [] };
    const client = clientWith(sent);
    await client.endTurn('c', 'e', 'a');
    await client.endTurn('c', 'e', 'a');
    expect(sent.endTurn[1].idempotencyKey).not.toBe(sent.endTurn[0].idempotencyKey);
  });
});

describe('CombatClient area spells', () => {
  function withRequests(): { client: CombatClient; requests: Record<string, unknown>[] } {
    TestBed.configureTestingModule({
      providers: [CombatClient, { provide: CONNECT_TRANSPORT, useValue: {} }],
    });
    const client = TestBed.inject(CombatClient);
    const requests: Record<string, unknown>[] = [];
    (client as unknown as { client: unknown }).client = {
      castSpell: (req: Record<string, unknown>) => {
        requests.push(req);
        return Promise.resolve({
          encounter: { id: 'enc' },
          cast: { targets: [] },
          summonedCombatantIds: [],
          area: { origin: { col: 3, row: 2 }, squares: [{ col: 3, row: 2 }] },
          hiddenHits: ['g3'],
          pendingRevealId: '',
        });
      },
      previewSpellArea: (req: Record<string, unknown>) => {
        requests.push(req);
        return Promise.resolve({ targets: [] });
      },
      resolveHiddenReveal: (req: Record<string, unknown>) => {
        requests.push(req);
        return Promise.resolve({ encounter: { id: 'enc' } });
      },
    };
    return { client, requests };
  }

  it("casts at a point with the master's reveal choice, and reads where it landed", async () => {
    const { client, requests } = withRequests();
    const res = await client.castSpell(
      'c',
      'e',
      'zuk',
      'spell:fireball',
      { level: 3, pact: false },
      [],
      null,
      'k',
      undefined,
      '',
      { area: { origin: { col: 3, row: 2 } }, revealHidden: false },
    );
    expect(requests[0]['area']).toEqual({ case: 'origin', value: { col: 3, row: 2 } });
    expect(requests[0]['revealHidden']).toBe(false);
    expect(res.area?.origin).toEqual({ col: 3, row: 2 });
    expect(res.hiddenHits).toEqual(['g3']);
  });

  it('leaves the reveal out unless the master chose, and sends a direction for a cone', async () => {
    const { client, requests } = withRequests();
    await client.castSpell(
      'c',
      'e',
      'p',
      'spell:burning-hands',
      { level: 1, pact: false },
      [],
      null,
      'k',
      undefined,
      '',
      { area: { direction: { dx: 1, dy: -1 } } },
    );
    expect('revealHidden' in requests[0]).toBe(false);
    expect(requests[0]['area']).toEqual({ case: 'direction', value: { dx: 1, dy: -1 } });
  });

  it('previews a sphere around the caster with no point, and answers a question with a key', async () => {
    const { client, requests } = withRequests();
    await client.previewSpellArea('c', 'e', 'p', 'spell:x', null, null);
    expect(requests[0]['area']).toEqual({ case: undefined });
    await client.resolveHiddenReveal('c', 'e', 'q1', true);
    expect(requests[1]).toMatchObject({ pendingRevealId: 'q1', reveal: true });
    expect(requests[1]['idempotencyKey']).toEqual(expect.any(String));
  });
});

describe('CombatClient.endCombatEffect', () => {
  it('ends Ajuda on a combatant, with a key that a retry keeps and another effect or combatant changes', async () => {
    TestBed.configureTestingModule({
      providers: [CombatClient, { provide: CONNECT_TRANSPORT, useValue: {} }],
    });
    const client = TestBed.inject(CombatClient);
    const sent: { combatantId: string; effect: CombatEffect; idempotencyKey: string }[] = [];
    const answers: (Error | null)[] = [new ConnectError('lost', Code.Unavailable), null, null];
    (client as unknown as { client: unknown }).client = {
      endCombatEffect: (req: {
        combatantId: string;
        effect: CombatEffect;
        idempotencyKey: string;
      }) => {
        sent.push(req);
        const failure = answers.shift();
        return failure ? Promise.reject(failure) : Promise.resolve({ encounter: { id: 'enc' } });
      },
    };
    await expect(client.endCombatEffect('c', 'e', 'sal', CombatEffect.AID)).rejects.toBeInstanceOf(
      ConnectError,
    );
    await client.endCombatEffect('c', 'e', 'sal', CombatEffect.AID);
    await client.endCombatEffect('c', 'e', 'tor', CombatEffect.AID);
    expect(sent.map((s) => [s.combatantId, s.effect])).toEqual([
      ['sal', CombatEffect.AID],
      ['sal', CombatEffect.AID],
      ['tor', CombatEffect.AID],
    ]);
    expect(sent[1].idempotencyKey).toBe(sent[0].idempotencyKey);
    expect(sent[2].idempotencyKey).not.toBe(sent[0].idempotencyKey);
  });
});

describe('CombatClient.moveOptions', () => {
  function clientAsking() {
    TestBed.configureTestingModule({
      providers: [CombatClient, { provide: CONNECT_TRANSPORT, useValue: {} }],
    });
    const client = TestBed.inject(CombatClient);
    const sent: object[] = [];
    (client as unknown as { client: unknown }).client = {
      getMoveOptions: (req: object) => {
        sent.push(req);
        return Promise.resolve({});
      },
    };
    return { client, sent };
  }

  it('asks for the walk with no jump', async () => {
    const { client, sent } = clientAsking();
    await client.moveOptions('c', 'e', 't');
    expect(sent[0]).toMatchObject({ jump: JumpKind.UNSPECIFIED, jumpRunningStart: false });
  });

  it("asks for the long jump, with the running start, so the warning is the jump's", async () => {
    const { client, sent } = clientAsking();
    await client.moveOptions('c', 'e', 't', { runningStart: true });
    expect(sent[0]).toMatchObject({
      combatantId: 't',
      jump: JumpKind.LONG,
      jumpRunningStart: true,
    });
  });
});
