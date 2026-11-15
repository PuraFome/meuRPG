import { create, type MessageInitShape } from '@bufbuild/protobuf';

import {
  ItemEmptyRule,
  ItemLoss,
  ItemRechargeKind,
  ItemRechargeSchema,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import {
  diceText,
  lastChargeBody,
  lostLine,
  noChargesWithRecharge,
  rechargeLine,
  rechargeShort,
  rechargeWithCap,
  startLine,
} from './recharge-text';

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

describe('rechargeLine: the line under an item row that has charges', () => {
  it('says the dice that come back at dawn, with the dawn icon', () => {
    expect(rechargeLine(recharge({ kind: ItemRechargeKind.DAWN, regainDice: '1d6+1' }))).toEqual({
      icon: 'wb_twilight',
      text: 'Volta 1d6 + 1 ao amanhecer',
    });
  });

  it('says all of them come back when the item names no dice', () => {
    expect(rechargeLine(recharge({ kind: ItemRechargeKind.DAWN }))?.text).toBe(
      'Volta tudo ao amanhecer',
    );
  });

  it('says how many dawns after the last charge, in the singular for one', () => {
    expect(
      rechargeLine(recharge({ kind: ItemRechargeKind.DAWNS_AFTER_EMPTY, waitDawns: 7 }))?.text,
    ).toBe('Volta tudo 7 amanheceres depois da última carga');
    expect(
      rechargeLine(recharge({ kind: ItemRechargeKind.DAWNS_AFTER_EMPTY, waitDawns: 1 }))?.text,
    ).toBe('Volta tudo 1 amanhecer depois da última carga');
  });

  it('says it does not come back, with the info icon', () => {
    expect(rechargeLine(recharge({ kind: ItemRechargeKind.UNSPECIFIED }))).toEqual({
      icon: 'info',
      text: 'Não volta: cada carga gasta se vai.',
    });
  });

  it('has nothing for an item with no rules', () => {
    expect(rechargeLine(undefined)).toBeNull();
  });
});

describe('rechargeWithCap: the footnote of the use sheet', () => {
  it('names the most the item holds after the dice', () => {
    expect(rechargeWithCap(recharge({ kind: ItemRechargeKind.DAWN, regainDice: '1d6+1' }), 7)).toBe(
      'Volta 1d6 + 1 ao amanhecer, até 7.',
    );
  });

  it('closes the other sentences with a full stop', () => {
    expect(rechargeWithCap(recharge({ kind: ItemRechargeKind.DAWN }), 3)).toBe(
      'Volta tudo ao amanhecer.',
    );
    expect(rechargeWithCap(recharge({ kind: ItemRechargeKind.UNSPECIFIED }), 3)).toBe(
      'Não volta: cada carga gasta se vai.',
    );
  });
});

describe('noChargesWithRecharge', () => {
  it('says the item comes back at dawn', () => {
    expect(
      noChargesWithRecharge(
        'Varinha de segredos',
        recharge({ kind: ItemRechargeKind.DAWN, regainDice: '1d3' }),
      ),
    ).toBe('Sem cargas. A Varinha de segredos volta ao amanhecer.');
  });

  it('says the dawns an item waits, and that one that never recharges does not', () => {
    expect(
      noChargesWithRecharge(
        'Estatueta de cabras',
        recharge({ kind: ItemRechargeKind.DAWNS_AFTER_EMPTY, waitDawns: 7 }),
      ),
    ).toBe('Sem cargas. A Estatueta de cabras volta tudo 7 amanheceres depois da última carga.');
    expect(
      noChargesWithRecharge(
        'Anel dos três desejos',
        recharge({ kind: ItemRechargeKind.UNSPECIFIED }),
      ),
    ).toBe('Sem cargas. O Anel dos três desejos não volta.');
  });
});

describe('the rolled start and the loss of the magic', () => {
  it('says the master sets the start, with the dice', () => {
    expect(startLine(recharge({ startDice: '1d4-1' }))).toBe(
      'O mestre define as cargas iniciais (1d4 − 1).',
    );
    expect(startLine(recharge({}))).toBe('O mestre define as cargas iniciais.');
  });

  it('says a plain item is left, by the noun of the name', () => {
    const plain = recharge({ onLoss: ItemLoss.PLAIN_ITEM });
    expect(lostLine('Bordão de encanto', plain, 'plain')).toBe('Virou um bordão comum.');
    expect(lostLine('Gema do brilho', plain, 'plain')).toBe('Virou uma gema comum.');
  });

  it('says what is left when only the bonus is', () => {
    expect(lostLine('Bordão do poder', recharge({ onLoss: ItemLoss.PARTLY_MAGIC }), 'partly')).toBe(
      'Ficou só o +2 no ataque e no dano.',
    );
  });

  it('says it was destroyed with the d20 only when the item rolled one', () => {
    expect(
      lostLine(
        'Varinha de teia',
        recharge({ empty: ItemEmptyRule.D20, onLoss: ItemLoss.GONE }),
        'destroyed',
      ),
    ).toBe('A última carga se foi e o d20 deu 1.');
    expect(
      lostLine(
        'Escaravelho',
        recharge({ empty: ItemEmptyRule.ALWAYS, onLoss: ItemLoss.GONE }),
        'destroyed',
      ),
    ).toBe('A última carga se foi.');
  });
});

describe('lastChargeBody: what the d20 of the last charge decides', () => {
  const parts = (name: string, r: Parameters<typeof recharge>[0]) => {
    const b = lastChargeBody(name, recharge(r));
    return `${b.intro}|${b.strong}|${b.rest}`;
  };

  it('writes the board: a 1 turns a wand to ashes and any other result waits for dawn', () => {
    expect(
      parts('Varinha de teia', {
        kind: ItemRechargeKind.DAWN,
        empty: ItemEmptyRule.D20,
        onLoss: ItemLoss.GONE,
      }),
    ).toBe(
      'O item pede um d20 agora.|Com 1, a varinha vira cinzas| e fica na ficha como destruída. Com 2 ou mais, ela espera o amanhecer.',
    );
  });

  it('says a staff keeps only a plain staff, or only its bonus', () => {
    expect(
      parts('Bordão de encanto', {
        kind: ItemRechargeKind.DAWN,
        empty: ItemEmptyRule.D20,
        onLoss: ItemLoss.PLAIN_ITEM,
      }),
    ).toBe(
      'O item pede um d20 agora.|Com 1, o bordão perde a magia| e vira um bordão comum. Com 2 ou mais, ele espera o amanhecer.',
    );
    expect(
      parts('Bordão do poder', {
        kind: ItemRechargeKind.DAWN,
        empty: ItemEmptyRule.D20,
        onLoss: ItemLoss.PARTLY_MAGIC,
      }),
    ).toContain('|Com 1, o bordão perde a magia| e fica só o +2 no ataque e no dano.');
  });

  it('adds what a 20 gives back, and a 2 to 19 waits', () => {
    const b = parts('Bordão do poder', {
      kind: ItemRechargeKind.DAWN,
      empty: ItemEmptyRule.D20,
      onLoss: ItemLoss.PARTLY_MAGIC,
      regainOnTwenty: '1d8+2',
    });
    expect(b).toContain('Com 2 a 19, ele espera o amanhecer. Com 20, devolve 1d8 + 2 cargas.');
  });

  it('has no clause for a 1 when the item is never lost', () => {
    expect(parts('Cajado do mago', { kind: ItemRechargeKind.DAWN, empty: ItemEmptyRule.D20 })).toBe(
      'O item pede um d20 agora.||Com qualquer resultado, ele espera o amanhecer.',
    );
  });
});
