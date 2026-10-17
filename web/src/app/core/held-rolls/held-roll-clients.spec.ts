import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';

import {
  DiceRollSchema,
  LuckyOfferSchema,
  OutsideInspirationOfferSchema,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';
import { EffectsClient } from '../effects/effects-client';
import { CombatClient } from '../combat/combat-client';
import { HeldRollAsked } from './held-roll-offer';

// The halfling's Lucky on the combat's d20 rolls that are not a weapon attack: the clients carry the question the server
// asks instead of the result, and the sheets share the words of the held d20.

const offer = (faces: number[] = [1], modifier = 5) =>
  create(LuckyOfferSchema, {
    holdId: 'h1',
    luckyIndex: Math.max(0, faces.indexOf(1)),
    d20: create(DiceRollSchema, {
      diceCount: faces.length,
      diceSides: 20,
      faces,
      modifier,
      total: (faces[0] ?? 0) + modifier,
    }),
  });

describe('the clients carry the question of a held roll', () => {
  function combatClient(answer: Record<string, unknown>): CombatClient {
    TestBed.configureTestingModule({
      providers: [CombatClient, { provide: CONNECT_TRANSPORT, useValue: {} }],
    });
    const client = TestBed.inject(CombatClient);
    (client as unknown as { base: { client: unknown } }).base.client = {
      castSpell: () => Promise.resolve(answer),
      resolveConcentrationSave: () => Promise.resolve(answer),
    };
    return client;
  }

  it('a spell attack that waits for Lucky throws the question, before the die’s', async () => {
    const client = combatClient({
      encounter: { id: 'enc' },
      luckyOffer: offer(),
      inspirationOffer: create(OutsideInspirationOfferSchema, { holdId: 'h1', sides: 8 }),
    });
    const cast = client.casts.castSpell('c', 'e', 'p', 'spell:guiding-bolt', null, [], null, 'k');
    await expect(cast).rejects.toBeInstanceOf(HeldRollAsked);
    await cast.catch((err: unknown) => {
      expect((err as HeldRollAsked).offer.holdId).toBe('h1');
      expect((err as HeldRollAsked).encounter.id).toBe('enc');
    });
  });

  it('a concentration save that waits for Lucky answers the question and no result', async () => {
    const client = combatClient({ encounter: { id: 'enc' }, luckyOffer: offer() });
    const res = await client.attacks.resolveConcentrationSave('c', 'e', 'w', { kind: 'app' }, 'k');
    expect(res.lucky?.holdId).toBe('h1');
    expect(res.result).toBeUndefined();
    expect(res.offer).toBeUndefined();
  });

  it('an effect’s saving throw that waits for Lucky answers the question and no result', async () => {
    TestBed.configureTestingModule({
      providers: [EffectsClient, { provide: CONNECT_TRANSPORT, useValue: {} }],
    });
    const client = TestBed.inject(EffectsClient);
    (client as unknown as { client: unknown }).client = {
      rollEffectSave: () => Promise.resolve({ encounter: { id: 'enc' }, luckyOffer: offer() }),
    };
    const res = await client.rollEffectSave('c', 'e', 'w', { kind: 'app' }, 'k');
    expect(res.lucky?.holdId).toBe('h1');
    expect(res.result).toBeUndefined();
  });
});
