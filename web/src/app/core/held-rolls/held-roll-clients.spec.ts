import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';

import {
  LuckyOfferSchema,
  OutsideInspirationOfferSchema,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { CombatClient } from '../combat/combat-client';
import { ContestClient, InspirationAsked } from '../combat/contest-client';
import { CONNECT_TRANSPORT } from '../connect/transport';
import { EffectsClient } from '../effects/effects-client';
import { SceneClient } from '../play/scene-client';
import { sceneRoll } from '../play/scene-testing';
import { HeldRollAsked } from './held-roll-offer';
import { heldDie, heldLucky } from './held-roll-testing';

// The clients carry the question a roll that waits asks instead of its result: the new offer, or the older fields of a
// response that has only those (Lucky's first).

const legacyLucky = () => create(LuckyOfferSchema, { holdId: 'h1', luckyIndex: 0 });

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

  const cast = (client: CombatClient) =>
    client.casts.castSpell('c', 'e', 'p', 'spell:guiding-bolt', null, [], null, 'k');

  it('a spell attack that waits throws the question with the combat', async () => {
    const asked = heldLucky();
    const sent = cast(combatClient({ encounter: { id: 'enc' }, heldRollOffer: asked }));
    await expect(sent).rejects.toBeInstanceOf(HeldRollAsked);
    await sent.catch((err: unknown) => {
      expect((err as HeldRollAsked).offer).toBe(asked);
      expect((err as HeldRollAsked).encounter.id).toBe('enc');
    });
  });

  it('a spell attack of a response with only the older fields asks Lucky’s question before the die’s', async () => {
    const sent = cast(
      combatClient({
        encounter: { id: 'enc' },
        luckyOffer: legacyLucky(),
        inspirationOffer: create(OutsideInspirationOfferSchema, { holdId: 'h2', sides: 8 }),
      }),
    );
    await sent.catch((err: unknown) => {
      expect((err as HeldRollAsked).offer.holdId).toBe('h1');
      expect((err as HeldRollAsked).offer.question.case).toBe('lucky');
    });
    await expect(sent).rejects.toBeInstanceOf(HeldRollAsked);
  });

  it('a concentration save that waits answers the question and no result', async () => {
    const client = combatClient({ encounter: { id: 'enc' }, heldRollOffer: heldLucky() });
    const res = await client.attacks.resolveConcentrationSave('c', 'e', 'w', { kind: 'app' }, 'k');
    expect(res.held?.holdId).toBe('h1');
    expect(res.result).toBeUndefined();
  });

  it('an effect’s saving throw that waits answers the question and no result', async () => {
    TestBed.configureTestingModule({
      providers: [EffectsClient, { provide: CONNECT_TRANSPORT, useValue: {} }],
    });
    const client = TestBed.inject(EffectsClient);
    (client as unknown as { client: unknown }).client = {
      rollEffectSave: () =>
        Promise.resolve({ encounter: { id: 'enc' }, heldRollOffer: heldLucky() }),
    };
    const res = await client.rollEffectSave('c', 'e', 'w', { kind: 'app' }, 'k');
    expect(res.held?.holdId).toBe('h1');
    expect(res.result).toBeUndefined();
  });

  describe('a scene check', () => {
    function sceneClient(answer: Record<string, unknown>): SceneClient {
      TestBed.configureTestingModule({
        providers: [SceneClient, { provide: CONNECT_TRANSPORT, useValue: {} }],
      });
      const client = TestBed.inject(SceneClient);
      (client as unknown as { client: unknown }).client = {
        rollSceneCheck: () => Promise.resolve(answer),
      };
      return client;
    }

    it('answers the question when the d20 waits, else the roll', async () => {
      const asked = heldDie();
      expect(await sceneClient({ heldRollOffer: asked }).roll('c', 'a', { inApp: true }, 'k')).toBe(
        asked,
      );
    });

    it('reads the older fields, Lucky’s first', async () => {
      const res = await sceneClient({
        luckyOffer: legacyLucky(),
        inspirationOffer: create(OutsideInspirationOfferSchema, { holdId: 'h2', sides: 8 }),
      }).roll('c', 'a', { inApp: true }, 'k');
      expect(res).toMatchObject({ holdId: 'h1' });
    });

    it('answers the roll when nothing waits', async () => {
      const made = sceneRoll('r1', 'a1', 'Pipo', 17);
      expect(await sceneClient({ roll: made }).roll('c', 'a', { inApp: true }, 'k')).toBe(made);
    });
  });

  describe('a group check', () => {
    function contestClient(answer: Record<string, unknown>): ContestClient {
      TestBed.configureTestingModule({
        providers: [ContestClient, { provide: CONNECT_TRANSPORT, useValue: {} }],
      });
      const client = TestBed.inject(ContestClient);
      (client as unknown as { client: unknown }).client = {
        rollGroupCheck: () => Promise.resolve(answer),
      };
      return client;
    }
    const roll = (client: ContestClient) => client.rollGroupCheck('c', 'g', { inApp: true }, 'k');

    it('answers Lucky’s question to the sheet', async () => {
      const asked = heldLucky();
      expect(await roll(contestClient({ heldRollOffer: asked }))).toBe(asked);
    });

    it('leaves the die’s question to the session panel: the sheet closes', async () => {
      await expect(roll(contestClient({ heldRollOffer: heldDie() }))).rejects.toBeInstanceOf(
        InspirationAsked,
      );
    });
  });
});
