import { type MessageInitShape, create } from '@bufbuild/protobuf';

import {
  ItemEmptyRule,
  ItemLoss,
  ItemRechargeKind,
  SheetAmmunitionSchema,
  SheetMagicItemSchema,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import {
  AMMUNITION_KINDS,
  ammunitionHint,
  ammunitionOf,
  articleOf,
  chargesLine,
  chargesOf,
  recoverOf,
} from './charges';

const item = (over: MessageInitShape<typeof SheetMagicItemSchema> = {}) =>
  create(SheetMagicItemSchema, { id: 'e1', chargesMax: 7, chargesLeft: 5, ...over });

describe('chargesOf', () => {
  it('has nothing for an item with no charge data', () => {
    expect(chargesOf(item({ chargesMax: 0, chargesLeft: 0 }))).toBeUndefined();
  });

  it('reads the state of the item from what the server sends', () => {
    expect(chargesOf(item())?.state).toBe('ready');
    expect(chargesOf(item({ chargesLeft: 0 }))?.state).toBe('empty');
    expect(chargesOf(item({ chargesLeft: -1 }))?.state).toBe('unset');
    expect(chargesOf(item({ chargesLeft: 0, lastRollPending: true }))?.state).toBe('pending');
    expect(chargesOf(item({ chargesLeft: 0, lost: ItemLoss.GONE }))?.state).toBe('destroyed');
    expect(chargesOf(item({ chargesLeft: 0, lost: ItemLoss.PLAIN_ITEM }))?.state).toBe('plain');
    expect(chargesOf(item({ chargesLeft: 0, lost: ItemLoss.PARTLY_MAGIC }))?.state).toBe('partly');
  });

  it('keeps the numbers, and never shows a count for a rolled start that is not set', () => {
    expect(chargesOf(item())).toMatchObject({ max: 7, left: 5 });
    expect(chargesOf(item({ chargesLeft: -1 }))?.left).toBe(0);
  });
});

describe('chargesLine', () => {
  it('says the line under each state, and nothing for a ready item', () => {
    const line = (over: Parameters<typeof item>[0]) => chargesLine(chargesOf(item(over))!);
    expect(line({})).toBeNull();
    expect(line({ chargesLeft: 0 })).toEqual({ icon: 'block', text: 'Sem cargas.' });
    expect(line({ chargesLeft: -1 })?.text).toBe('O mestre define as cargas iniciais.');
    expect(line({ chargesLeft: 0, lastRollPending: true })).toEqual({
      icon: 'warning',
      text: 'Falta rolar o d20 da última carga.',
      warn: true,
    });
    expect(line({ chargesLeft: 0, lost: ItemLoss.GONE })?.text).toBe('A última carga se foi.');
    expect(line({ chargesLeft: 0, lost: ItemLoss.PLAIN_ITEM })?.text).toBe('Virou um item comum.');
    expect(line({ chargesLeft: 0, lost: ItemLoss.PARTLY_MAGIC })?.text).toBe(
      'Ficou só o bônus no ataque e no dano.',
    );
  });
});

describe('chargesLine of an item that carries its recharge rules', () => {
  const dawn = { kind: ItemRechargeKind.DAWN, regainDice: '1d6+1' };
  const line = (over: Parameters<typeof item>[0], name = 'Varinha de teia') =>
    chargesLine(chargesOf(item(over))!, name);

  it('keeps the same item the same: no rules, no recharge line', () => {
    expect(line({})).toBeNull();
    expect(line({ chargesLeft: 0 })?.text).toBe('Sem cargas.');
  });

  it('says when a ready item gets its charges back', () => {
    expect(line({ recharge: dawn })).toEqual({
      icon: 'wb_twilight',
      text: 'Volta 1d6 + 1 ao amanhecer',
    });
    expect(line({ recharge: { kind: ItemRechargeKind.UNSPECIFIED } })).toEqual({
      icon: 'info',
      text: 'Não volta: cada carga gasta se vai.',
    });
  });

  it('says when an empty item gets them back', () => {
    expect(
      line({ chargesLeft: 0, recharge: { kind: ItemRechargeKind.DAWN, regainDice: '1d3' } }),
    ).toEqual({ icon: 'block', text: 'Sem cargas: volta 1d3 ao amanhecer.' });
  });

  it('says the dice of a rolled start', () => {
    expect(line({ chargesLeft: -1, recharge: { startDice: '1d4-1' } })?.text).toBe(
      'O mestre define as cargas iniciais (1d4 − 1).',
    );
  });

  it('says what is left of an item without its magic, by the rules of the catalog', () => {
    expect(
      line(
        { chargesLeft: 0, lost: ItemLoss.PLAIN_ITEM, recharge: { onLoss: ItemLoss.PLAIN_ITEM } },
        'Bordão de encanto',
      )?.text,
    ).toBe('Virou um bordão comum.');
    expect(
      line(
        {
          chargesLeft: 0,
          lost: ItemLoss.PARTLY_MAGIC,
          recharge: { onLoss: ItemLoss.PARTLY_MAGIC },
        },
        'Bordão do poder',
      )?.text,
    ).toBe('Ficou só o +2 no ataque e no dano.');
    expect(
      line({
        chargesLeft: 0,
        lost: ItemLoss.GONE,
        recharge: { empty: ItemEmptyRule.D20, onLoss: ItemLoss.GONE },
      })?.text,
    ).toBe('A última carga se foi e o d20 deu 1.');
  });

  it('keeps the d20 reminder of a pending roll', () => {
    expect(line({ chargesLeft: 0, lastRollPending: true, recharge: dawn })?.warn).toBe(true);
  });

  it('keeps the recharge rules on the view of the item', () => {
    expect(chargesOf(item({ recharge: dawn }))?.recharge?.regainDice).toBe('1d6+1');
    expect(chargesOf(item())?.recharge).toBeUndefined();
  });
});

const ammo = (key: string, over = {}) =>
  create(SheetAmmunitionSchema, { key, namePt: 'Flechas', quantity: 15, ...over });

describe('ammunition', () => {
  it('lists the four SRD kinds with their bundle', () => {
    expect(AMMUNITION_KINDS.map((k) => [k.key, k.namePt, k.bundle])).toEqual([
      ['equipment:arrow', 'Flechas', 20],
      ['equipment:crossbow-bolt', 'Virotes', 20],
      ['equipment:sling-bullet', 'Balas de funda', 20],
      ['equipment:blowgun-needle', 'Agulhas de zarabatana', 50],
    ]);
  });

  it('names the weapons of the catalog that fire each kind (effects/ammunition.json of the SRD content)', () => {
    // The keys the server uses: a weapon whose key is not here never gets a count tag.
    const catalog: Record<string, string> = {
      'equipment:blowgun': 'equipment:blowgun-needle',
      'equipment:crossbow-hand': 'equipment:crossbow-bolt',
      'equipment:crossbow-heavy': 'equipment:crossbow-bolt',
      'equipment:crossbow-light': 'equipment:crossbow-bolt',
      'equipment:longbow': 'equipment:arrow',
      'equipment:shortbow': 'equipment:arrow',
      'equipment:sling': 'equipment:sling-bullet',
    };
    const ours: Record<string, string> = {};
    for (const kind of AMMUNITION_KINDS) {
      for (const weapon of kind.weapons) {
        ours[weapon] = kind.key;
      }
    }
    expect(ours).toEqual(catalog);
  });

  it('says the kind in the one and the many, in lower case', () => {
    expect(AMMUNITION_KINDS.map((k) => [k.one, k.many])).toEqual([
      ['flecha', 'flechas'],
      ['virote', 'virotes'],
      ['bala de funda', 'balas de funda'],
      ['agulha de zarabatana', 'agulhas de zarabatana'],
    ]);
  });

  it('keeps what a kind fired and what half of it brings back', () => {
    expect(ammunitionOf([ammo('equipment:arrow', { spentLastCombat: 7 })])[0]).toMatchObject({
      quantity: 15,
      spent: 7,
      back: 3,
    });
  });

  it('offers to recover half of what was fired, in pieces of the one kind', () => {
    expect(recoverOf(ammunitionOf([ammo('a', { spentLastCombat: 6 })]))).toEqual({
      state: 'ready',
      count: 3,
      pieces: '3 flechas',
      label: 'Recuperar a metade: 3 flechas',
    });
  });

  it('says "peças" for one piece or for several kinds', () => {
    expect(recoverOf(ammunitionOf([ammo('a', { spentLastCombat: 3 })])).label).toBe(
      'Recuperar a metade: 1 peça',
    );
    expect(
      recoverOf(
        ammunitionOf([
          ammo('a', { spentLastCombat: 4 }),
          ammo('b', { namePt: 'Virotes', spentLastCombat: 2 }),
        ]),
      ).label,
    ).toBe('Recuperar a metade: 3 peças');
  });

  it('is done after the recovery and has nothing to recover without a shot', () => {
    const done = recoverOf(ammunitionOf([ammo('a', { spentLastCombat: 6, recovered: true })]));
    expect(done).toMatchObject({ state: 'done', count: 3 });
    expect(recoverOf(ammunitionOf([ammo('a', { spentLastCombat: 0 })])).state).toBe('nothing');
    expect(recoverOf(ammunitionOf([ammo('a', { spentLastCombat: 1 })])).state).toBe('nothing');
  });
});

describe('ammunitionHint', () => {
  it('names the free-text item that looks like ammunition, with its kind and count', () => {
    expect(
      ammunitionHint([
        { name: 'Mochila', quantity: 1 },
        { name: 'Flechas (20)', quantity: 1 },
      ]),
    ).toEqual({
      name: 'Flechas (20)',
      key: 'equipment:arrow',
      quantity: 20,
    });
    expect(ammunitionHint([{ name: 'Virotes', quantity: 12 }])).toEqual({
      name: 'Virotes',
      key: 'equipment:crossbow-bolt',
      quantity: 12,
    });
    expect(ammunitionHint([{ name: 'Balas de funda (20)', quantity: 1 }])?.key).toBe(
      'equipment:sling-bullet',
    );
    expect(ammunitionHint([{ name: 'Agulha de zarabatana', quantity: 1 }])?.key).toBe(
      'equipment:blowgun-needle',
    );
  });

  it('is absent when nothing looks like ammunition', () => {
    expect(ammunitionHint([{ name: 'Corda de cânhamo (15 m)', quantity: 1 }])).toBeNull();
    expect(ammunitionHint([])).toBeNull();
  });
});

describe('articleOf', () => {
  it('takes the article of the name from its first word', () => {
    expect(articleOf('Varinha de teia')).toBe('A');
    expect(articleOf('Bordão do poder')).toBe('O');
    expect(articleOf('Anel de estrelas cadentes')).toBe('O');
    expect(articleOf('Mapa de estrelas')).toBe('O');
  });
});
