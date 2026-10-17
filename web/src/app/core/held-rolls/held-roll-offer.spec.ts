import { create } from '@bufbuild/protobuf';

import {
  DiceRollSchema,
  HeldRollKind,
  InspirationOfferSchema,
  LuckyOfferSchema,
  OutsideInspirationOfferSchema,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { LUCKY_TRAIT } from '../combat/lucky';
import {
  HeldRollAsked,
  asksLucky,
  dieOf,
  heldOfferOf,
  luckyHeldLine,
  luckyReroll,
} from './held-roll-offer';
import { heldDie, heldLucky } from './held-roll-testing';

const d20 = create(DiceRollSchema, {
  diceCount: 1,
  diceSides: 20,
  faces: [1],
  modifier: 5,
  total: 6,
});

describe('the question of a held roll', () => {
  it('prefers the new offer over the older ones', () => {
    const held = heldDie();
    const out = heldOfferOf({
      heldRollOffer: held,
      luckyOffer: create(LuckyOfferSchema, { holdId: 'old' }),
    });
    expect(out).toBe(held);
  });

  it('reads an older Lucky offer, with the attack it names', () => {
    const out = heldOfferOf({
      luckyOffer: create(LuckyOfferSchema, {
        holdId: 'h',
        d20,
        luckyIndex: 1,
        attackKey: 'attack:axe',
        targetId: 'g',
      }),
    });
    expect(out?.holdId).toBe('h');
    expect(out?.question).toEqual({
      case: 'lucky',
      value: expect.objectContaining({ luckyIndex: 1 }),
    });
    expect(out?.kind).toBe(HeldRollKind.WEAPON_ATTACK);
    expect(out?.attackKey).toBe('attack:axe');
    expect(out?.targetId).toBe('g');
    expect(out?.d20).toBe(d20);
  });

  it('reads the older die offers of a combat and of a roll outside it', () => {
    const combat = heldOfferOf({
      inspirationOffer: create(InspirationOfferSchema, {
        holdId: 'c',
        d20,
        die: { sides: 6, fromLabel: 'Orla' },
        attackKey: 'attack:axe',
        targetId: 'g',
      }),
    });
    expect(dieOf(combat)).toMatchObject({ sides: 6, fromName: 'Orla' });
    expect(combat?.attackKey).toBe('attack:axe');
    const outside = heldOfferOf({
      inspirationOffer: create(OutsideInspirationOfferSchema, {
        holdId: 'o',
        d20,
        sides: 8,
        fromName: 'Aurora',
      }),
    });
    expect(dieOf(outside)).toMatchObject({ sides: 8, fromName: 'Aurora' });
    expect(outside?.kind).toBe(HeldRollKind.UNSPECIFIED);
  });

  it('asks Lucky’s question before the die’s when only the older offers arrive', () => {
    const out = heldOfferOf({
      luckyOffer: create(LuckyOfferSchema, { holdId: 'l', d20 }),
      inspirationOffer: create(OutsideInspirationOfferSchema, { holdId: 'd', sides: 6 }),
    });
    expect(asksLucky(out)).toBe(true);
    expect(heldOfferOf({})).toBeUndefined();
  });

  it('tells which question it is', () => {
    expect(asksLucky(heldLucky())).toBe(true);
    expect(dieOf(heldLucky())).toBeUndefined();
    expect(asksLucky(heldDie())).toBe(false);
    expect(dieOf(heldDie(11, 5, 6))).toMatchObject({ sides: 6, fromName: 'Orla' });
    expect(asksLucky(null)).toBe(false);
    expect(dieOf(undefined)).toBeUndefined();
  });

  it('says the held d20 and its bonus, and that the result comes after the answer', () => {
    expect(luckyHeldLine(heldLucky())).toBe(
      'Seu d20: 1d20 (1) + 5 = 6. O resultado só aparece depois da sua escolha.',
    );
    expect(luckyHeldLine(null)).toBe('');
  });

  it('finds the d20 Lucky rolled again, and nothing for a reroll of another feature', () => {
    const roll = create(DiceRollSchema, {
      faces: [14],
      rerolled: [
        { index: 0, from: 1, to: 4, sourceKey: '' },
        { index: 0, from: 1, to: 14, sourceKey: LUCKY_TRAIT },
      ],
    });
    expect(luckyReroll(roll)?.to).toBe(14);
    expect(luckyReroll(create(DiceRollSchema, { faces: [1] }))).toBeUndefined();
    expect(luckyReroll(undefined)).toBeUndefined();
  });

  it('carries the question and the combat of a cast that asked', () => {
    const asked = new HeldRollAsked(heldLucky(), {} as never);
    expect(asked).toBeInstanceOf(Error);
    expect(asked.offer.holdId).toBe('h1');
  });
});
