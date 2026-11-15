import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';

import {
  ItemRechargeKind,
  ItemRechargeSchema,
  SheetMagicItemSchema,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { ChargesClient } from './charges-client';
import { PartyCharges } from './party-charges-state';

const wand = create(SheetMagicItemSchema, {
  id: 'w1',
  namePt: 'Varinha de mísseis mágicos',
  chargesMax: 7,
  chargesLeft: 5,
  recharge: create(ItemRechargeSchema, { kind: ItemRechargeKind.DAWN, regainDice: '1d6+1' }),
});

describe('PartyCharges', () => {
  const sheetOf = vi.fn();
  const party = [
    { characterId: 'pensantus', name: 'Pensantus' },
    { characterId: 'brisa', name: 'Brisa' },
  ];

  beforeEach(() => {
    sheetOf.mockReset();
    sheetOf.mockImplementation((_c: string, id: string) =>
      Promise.resolve({ magicItems: id === 'pensantus' ? [wand] : [], ammunition: [] }),
    );
    TestBed.configureTestingModule({
      providers: [{ provide: ChargesClient, useValue: { sheetOf } }],
    });
  });

  it('reads the sheet of each character of the party and derives the panel and the dawn list', async () => {
    const store = TestBed.inject(PartyCharges);
    expect(store.status()).toBe('idle');
    await store.load('camp', party);
    expect(sheetOf).toHaveBeenCalledWith('camp', 'pensantus');
    expect(sheetOf).toHaveBeenCalledWith('camp', 'brisa');
    expect(store.status()).toBe('ready');
    expect(store.view().characters.map((c) => c.name)).toEqual(['Pensantus']);
    expect(store.view().quiet).toEqual(['Brisa']);
    expect(store.waiting().map((l) => l.text)).toEqual([
      'Varinha de mísseis mágicos: 1d6 + 1, até 7 (tem 5)',
    ]);
  });

  it('keeps what it knew and says it failed when a sheet cannot be read', async () => {
    const store = TestBed.inject(PartyCharges);
    await store.load('camp', party);
    sheetOf.mockRejectedValueOnce(new Error('down'));
    await store.load('camp', party);
    expect(store.status()).toBe('error');
    expect(store.view().characters.map((c) => c.name)).toEqual(['Pensantus']);
  });

  it('answers the name of a character of the party', async () => {
    const store = TestBed.inject(PartyCharges);
    await store.load('camp', party);
    expect(store.nameOf('brisa')).toBe('Brisa');
    expect(store.nameOf('ghost')).toBe('');
  });
});
