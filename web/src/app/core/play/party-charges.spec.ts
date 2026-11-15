import { create, type MessageInitShape } from '@bufbuild/protobuf';

import {
  ItemLoss,
  ItemRechargeKind,
  ItemRechargeSchema,
  SheetAmmunitionSchema,
  SheetMagicItemSchema,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { DawnRestoredSchema } from '../../../gen/meurpg/play/v1/charges_pb';
import {
  dawnResultOf,
  dawnWaiting,
  partyChargesOf,
  quietText,
  type PartyMemberSheet,
} from './party-charges';

const wand = (init: MessageInitShape<typeof SheetMagicItemSchema> = {}) =>
  create(SheetMagicItemSchema, {
    id: 'w1',
    namePt: 'Varinha de mísseis mágicos',
    chargesMax: 7,
    chargesLeft: 5,
    recharge: create(ItemRechargeSchema, { kind: ItemRechargeKind.DAWN, regainDice: '1d6+1' }),
    ...init,
  });

const member = (init: Partial<PartyMemberSheet> = {}): PartyMemberSheet => ({
  characterId: 'pensantus',
  name: 'Pensantus',
  magicItems: [wand()],
  ammunition: [],
  ...init,
});

describe('partyChargesOf', () => {
  it('lists the items with charges and the counted ammunition of each character, in the party order', () => {
    const view = partyChargesOf([
      member({
        ammunition: [
          create(SheetAmmunitionSchema, {
            key: 'equipment:arrow',
            namePt: 'Flechas',
            quantity: 15,
          }),
        ],
      }),
      member({ characterId: 'brisa', name: 'Brisa', magicItems: [] }),
      member({
        characterId: 'tor',
        name: 'Tor',
        magicItems: [
          wand({
            id: 'r1',
            namePt: 'Anel de estrelas cadentes',
            attuned: true,
            chargesMax: 6,
            chargesLeft: 4,
          }),
        ],
      }),
    ]);
    expect(view.characters.map((c) => c.name)).toEqual(['Pensantus', 'Tor']);
    const first = view.characters[0]!;
    expect(first.items[0]).toMatchObject({
      entryId: 'w1',
      name: 'Varinha de mísseis mágicos',
      value: '5 de 7',
      left: 5,
      max: 7,
      volta: 'volta 1d6 + 1',
      empty: false,
      destroyed: false,
      attuned: false,
    });
    expect(first.ammunition).toEqual([{ key: 'equipment:arrow', name: 'Flechas', quantity: 15 }]);
    expect(view.characters[1]!.items[0]).toMatchObject({ attuned: true, left: 4, max: 6 });
    expect(view.quiet).toEqual(['Brisa']);
  });

  it('leaves out an item with no charge data and marks the empty and the destroyed ones', () => {
    const view = partyChargesOf([
      member({
        magicItems: [
          wand({ id: 'plain', chargesMax: 0 }),
          wand({ id: 'zero', namePt: 'Varinha de segredos', chargesMax: 3, chargesLeft: 0 }),
          wand({ id: 'gone', namePt: 'Varinha de teia', chargesMax: 7, lost: ItemLoss.GONE }),
        ],
      }),
    ]);
    const items = view.characters[0]!.items;
    expect(items.map((i) => i.entryId)).toEqual(['zero', 'gone']);
    expect(items[0]).toMatchObject({ empty: true, value: '0 de 3' });
    expect(items[1]).toMatchObject({ destroyed: true, volta: '' });
  });

  it('counts a rolled start the master has not set as no charges, so the master sets it', () => {
    const view = partyChargesOf([
      member({ magicItems: [wand({ chargesLeft: -1, chargesMax: 4 })] }),
    ]);
    expect(view.characters[0]!.items[0]).toMatchObject({ left: 0, max: 4, unset: true });
  });
});

describe('quietText', () => {
  it('names who has nothing to correct', () => {
    expect(quietText(['Brisa'])).toBe('Brisa não tem itens com cargas nem munição.');
    expect(quietText(['Brisa', 'Jabuti'])).toBe(
      'Brisa e Jabuti não têm itens com cargas nem munição.',
    );
    expect(quietText(['Brisa', 'Jabuti', 'Tor'])).toBe(
      'Brisa, Jabuti e Tor não têm itens com cargas nem munição.',
    );
    expect(quietText([])).toBe('');
  });
});

describe('dawnWaiting', () => {
  it('lists the items a dawn would touch with the dice, the maximum and what they hold', () => {
    const view = partyChargesOf([
      member({
        magicItems: [
          wand(),
          wand({ id: 'full', namePt: 'Cheia', chargesLeft: 7 }),
          wand({
            id: 'seg',
            namePt: 'Varinha de segredos',
            chargesMax: 3,
            chargesLeft: 0,
            recharge: create(ItemRechargeSchema, {
              kind: ItemRechargeKind.DAWN,
              regainDice: '1d3',
            }),
          }),
          wand({
            id: 'never',
            namePt: 'Anel dos três desejos',
            chargesMax: 3,
            chargesLeft: 2,
            recharge: create(ItemRechargeSchema, { kind: ItemRechargeKind.UNSPECIFIED }),
          }),
          wand({ id: 'gone', namePt: 'Varinha de teia', lost: ItemLoss.GONE }),
          wand({ id: 'pend', namePt: 'Pendente', chargesLeft: 0, lastRollPending: true }),
        ],
      }),
    ]);
    expect(dawnWaiting(view)).toEqual([
      { characterName: 'Pensantus', text: 'Varinha de mísseis mágicos: 1d6 + 1, até 7 (tem 5)' },
      { characterName: 'Pensantus', text: 'Varinha de segredos: 1d3, até 3 (tem 0)' },
    ]);
  });

  it('says "todas" for an item that gets all of them back, and counts the figurines only with none left', () => {
    const goats = (left: number) =>
      member({
        magicItems: [
          wand({
            namePt: 'Figurinhas de bodes',
            chargesMax: 3,
            chargesLeft: left,
            recharge: create(ItemRechargeSchema, {
              kind: ItemRechargeKind.DAWNS_AFTER_EMPTY,
              waitDawns: 7,
            }),
          }),
          wand({
            id: 'all',
            namePt: 'Cajado',
            chargesMax: 10,
            chargesLeft: 4,
            recharge: create(ItemRechargeSchema, { kind: ItemRechargeKind.DAWN }),
          }),
        ],
      });
    expect(dawnWaiting(partyChargesOf([goats(0)])).map((l) => l.text)).toEqual([
      'Figurinhas de bodes: todas depois de 7 amanheceres, até 3 (tem 0)',
      'Cajado: todas, até 10 (tem 4)',
    ]);
    expect(dawnWaiting(partyChargesOf([goats(2)])).map((l) => l.text)).toEqual([
      'Cajado: todas, até 10 (tem 4)',
    ]);
  });
});

describe('dawnResultOf', () => {
  const restored = (
    characterId: string,
    name: string,
    regained: number,
    left: number,
    max: number,
    dice: string,
  ) =>
    create(DawnRestoredSchema, {
      characterId,
      regained,
      item: create(SheetMagicItemSchema, {
        namePt: name,
        chargesMax: max,
        chargesLeft: left,
        recharge: create(ItemRechargeSchema, { kind: ItemRechargeKind.DAWN, regainDice: dice }),
      }),
    });
  const nameOf = (id: string) => ({ pensantus: 'Pensantus', tor: 'Tor' })[id] ?? '';

  it("says how many items got charges, with each one's dice, what it regained and where it stands", () => {
    const res = dawnResultOf(
      [
        restored('pensantus', 'Varinha de mísseis mágicos', 5, 7, 7, '1d6+1'),
        restored('tor', 'Anel de estrelas cadentes', 1, 5, 6, '1d6'),
      ],
      nameOf,
    );
    expect(res.head).toBe('Amanhecer. 2 itens receberam cargas:');
    expect(res.lines).toEqual([
      'Varinha de mísseis mágicos de Pensantus: 1d6 + 1 = 5, fica com 7 de 7.',
      'Anel de estrelas cadentes de Tor: 1d6 = 1, fica com 5 de 6.',
    ]);
  });

  it('uses the singular for one item and says all of them when the item names no dice', () => {
    const res = dawnResultOf([restored('tor', 'Cajado', 6, 6, 6, '')], nameOf);
    expect(res.head).toBe('Amanhecer. 1 item recebeu cargas:');
    expect(res.lines).toEqual(['Cajado de Tor: todas as cargas, fica com 6 de 6.']);
  });

  it('says no item waits for the dawn when none got charges', () => {
    expect(dawnResultOf([], nameOf)).toEqual({
      head: 'Nenhum item espera o amanhecer.',
      lines: [],
    });
  });
});
