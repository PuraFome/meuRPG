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
