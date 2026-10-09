import { create } from '@bufbuild/protobuf';
import { TestBed } from '@angular/core/testing';

import {
  DiceRollSchema,
  InspirationOfferSchema,
  RollAttackResponseSchema,
  AttackRollSchema,
  EncounterSchema,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { LayOnHandsCure } from '../../../gen/meurpg/play/v1/resources_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';
import { ResourceClient } from './resources-client';

function clientWith(sent: unknown[], answer: unknown = {}): ResourceClient {
  TestBed.configureTestingModule({
    providers: [ResourceClient, { provide: CONNECT_TRANSPORT, useValue: {} }],
  });
  const client = TestBed.inject(ResourceClient);
  const call = (req: unknown) => {
    sent.push(req);
    return Promise.resolve(answer);
  };
  (client as unknown as { client: unknown }).client = {
    useLayOnHands: call,
    createSpellSlot: call,
    convertSpellSlot: call,
    giveBardicInspiration: call,
    answerBardicInspiration: call,
  };
  return client;
}

describe('ResourceClient: the class resource calls', () => {
  it('touches with an amount, or with a cure, under the key of the caller', async () => {
    const sent: unknown[] = [];
    const client = clientWith(sent);
    await client.useLayOnHands('c', 'e', 'a', 't', { amount: 8 }, 'k1');
    await client.useLayOnHands('c', 'e', 'a', 't', { cure: 'poison' }, 'k2');
    await client.useLayOnHands('c', 'e', 'a', 't', { cure: 'disease' }, 'k3');
    expect(sent).toEqual([
      {
        campaignId: 'c',
        encounterId: 'e',
        actorId: 'a',
        targetId: 't',
        idempotencyKey: 'k1',
        effect: { case: 'amount', value: 8 },
      },
      expect.objectContaining({ effect: { case: 'cure', value: LayOnHandsCure.POISON } }),
      expect.objectContaining({ effect: { case: 'cure', value: LayOnHandsCure.DISEASE } }),
    ]);
  });

  it('creates and converts a slot by its level', async () => {
    const sent: unknown[] = [];
    const client = clientWith(sent);
    await client.createSpellSlot('c', 'e', 'a', 2, 'k1');
    await client.convertSpellSlot('c', 'e', 'a', 3, 'k2');
    expect(sent).toEqual([
      { campaignId: 'c', encounterId: 'e', actorId: 'a', slotLevel: 2, idempotencyKey: 'k1' },
      { campaignId: 'c', encounterId: 'e', actorId: 'a', slotLevel: 3, idempotencyKey: 'k2' },
    ]);
  });

  it('gives the die to a creature', async () => {
    const sent: unknown[] = [];
    await clientWith(sent).giveBardicInspiration('c', 'e', 'a', 't', 'k');
    expect(sent).toEqual([
      { campaignId: 'c', encounterId: 'e', actorId: 'a', targetId: 't', idempotencyKey: 'k' },
    ]);
  });

  it('answers a held roll: using the die rolled in the app, with a typed face, or keeping it', async () => {
    const sent: unknown[] = [];
    const attack = create(RollAttackResponseSchema, {
      encounter: create(EncounterSchema, { id: 'e' }),
      roll: create(AttackRollSchema, { attackKey: 'x' }),
    });
    const client = clientWith(sent, { attack });
    const res = await client.answerBardicInspiration('c', 'e', 'h', true, { inApp: true }, 'k1');
    await client.answerBardicInspiration('c', 'e', 'h', true, { face: 6 }, 'k2');
    await client.answerBardicInspiration('c', 'e', 'h', false, { inApp: true }, 'k3');
    expect(res.roll.attackKey).toBe('x');
    expect(sent).toEqual([
      expect.objectContaining({ holdId: 'h', use: true, roll: { case: 'rollInApp', value: true } }),
      expect.objectContaining({ use: true, roll: { case: 'typedFace', value: 6 } }),
      // Keeping the die sends no roll at all.
      expect.objectContaining({ use: false, roll: { case: undefined } }),
    ]);
  });

  it('reads the d20 of a held attack from its offer: there is no outcome yet', async () => {
    const offer = create(InspirationOfferSchema, {
      holdId: 'h',
      d20: create(DiceRollSchema, {
        diceCount: 1,
        diceSides: 20,
        faces: [9],
        modifier: 5,
        total: 14,
      }),
    });
    const held = create(RollAttackResponseSchema, {
      encounter: create(EncounterSchema, { id: 'e' }),
      inspirationOffer: offer,
    });
    const res = await clientWith([], { attack: held }).answerBardicInspiration(
      'c',
      'e',
      'h',
      false,
      null,
      'k',
    );
    expect(res.offer?.holdId).toBe('h');
    expect(res.roll.d20?.total).toBe(14);
  });

  it('fails when the answer carries no attack', async () => {
    await expect(
      clientWith([], {}).answerBardicInspiration('c', 'e', 'h', false, null, 'k'),
    ).rejects.toThrow('without its attack');
  });
});
