import { create, type MessageInitShape } from '@bufbuild/protobuf';

import {
  ItemRechargeKind,
  ItemRechargeSchema,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { diceText, rechargeShort } from './recharge-text';

const recharge = (init: MessageInitShape<typeof ItemRechargeSchema>) =>
  create(ItemRechargeSchema, init);

describe('diceText', () => {
  it('spaces the plus and draws the minus sign as the boards do', () => {
    expect(diceText('1d6+1')).toBe('1d6 + 1');
    expect(diceText('1d4-1')).toBe('1d4 − 1');
    expect(diceText('2d8+2')).toBe('2d8 + 2');
  });

  it('leaves plain dice and nothing as they are', () => {
    expect(diceText('1d3')).toBe('1d3');
    expect(diceText('')).toBe('');
  });
});

describe('rechargeShort', () => {
  it('says the dice that return at dawn', () => {
    expect(rechargeShort(recharge({ kind: ItemRechargeKind.DAWN, regainDice: '1d6+1' }))).toBe(
      'volta 1d6 + 1',
    );
  });

  it('says all of them return when the item names no dice', () => {
    expect(rechargeShort(recharge({ kind: ItemRechargeKind.DAWN }))).toBe('volta tudo');
  });

  it('says how many dawns an item waits after its last charge', () => {
    expect(
      rechargeShort(recharge({ kind: ItemRechargeKind.DAWNS_AFTER_EMPTY, waitDawns: 7 })),
    ).toBe('volta tudo 7 amanheceres depois da última carga');
  });

  it('says an item never recharges, and nothing for an item with no rules', () => {
    expect(rechargeShort(recharge({ kind: ItemRechargeKind.UNSPECIFIED }))).toBe('não volta');
    expect(rechargeShort(undefined)).toBe('');
  });
});
