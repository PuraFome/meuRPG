import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import {
  CharacterBlockedReason,
  CharacterBlockedSchema,
  ItemLoss,
  ItemRechargeKind,
  ItemRechargeSchema,
  SheetAmmunitionSchema,
  SheetMagicItemSchema,
} from '../../../../gen/meurpg/characters/v1/characters_pb';
import {
  ChargesBlockedReason,
  ChargesBlockedSchema,
  DawnRestoredSchema,
} from '../../../../gen/meurpg/play/v1/charges_pb';
import { ChargesClient } from '../../../core/play/charges-client';
import { pensantusVitals } from '../../../core/play/live-testing';
import { PartyChargesPanel } from './party-charges';

const flat = (n: Element | null | undefined) => n?.textContent?.replace(/\s+/g, ' ').trim();

const dawnRule = (dice: string) =>
  create(ItemRechargeSchema, { kind: ItemRechargeKind.DAWN, regainDice: dice });

const item = (id: string, name: string, max: number, left: number, dice = '1d6') =>
  create(SheetMagicItemSchema, {
    id,
    namePt: name,
    chargesMax: max,
    chargesLeft: left,
    recharge: dawnRule(dice),
  });

describe('PartyChargesPanel', () => {
  const sheets: Record<string, unknown> = {};
  const client = {
    sheetOf: vi.fn((_c: string, id: string) => Promise.resolve(sheets[id])),
    setCharges: vi.fn(),
    setAmmunition: vi.fn(),
    dawn: vi.fn(),
  };

  beforeEach(() => {
    for (const fn of Object.values(client)) {
      fn.mockClear();
    }
    client.setCharges.mockResolvedValue({});
    client.setAmmunition.mockResolvedValue({});
    client.dawn.mockResolvedValue({ restored: [] });
    sheets['pensantus'] = {
      magicItems: [
        item('w1', 'Varinha de mísseis mágicos', 7, 5, '1d6+1'),
        item('w2', 'Varinha de segredos', 3, 0, '1d3'),
        create(SheetMagicItemSchema, {
          id: 'w3',
          namePt: 'Varinha de teia',
          chargesMax: 7,
          lost: ItemLoss.GONE,
          recharge: dawnRule('1d6'),
        }),
      ],
      ammunition: [],
    };
    sheets['tor'] = {
      magicItems: [
        create(SheetMagicItemSchema, {
          ...item('r1', 'Anel de estrelas cadentes', 6, 4),
          attuned: true,
        }),
      ],
      ammunition: [
        create(SheetAmmunitionSchema, { key: 'equipment:arrow', namePt: 'Flechas', quantity: 15 }),
      ],
    };
    sheets['brisa'] = { magicItems: [], ammunition: [] };
    TestBed.configureTestingModule({ providers: [{ provide: ChargesClient, useValue: client }] });
  });

  afterEach(() => document.body.replaceChildren());

  async function render() {
    const fixture = TestBed.createComponent(PartyChargesPanel);
    document.body.appendChild(fixture.nativeElement);
    fixture.componentRef.setInput('campaignId', 'camp');
    fixture.componentRef.setInput('party', [
      pensantusVitals(),
      pensantusVitals({ characterId: 'tor', name: 'Tor' }),
      pensantusVitals({ characterId: 'brisa', name: 'Brisa' }),
    ]);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const settle = async () => {
      await fixture.whenStable();
      fixture.detectChanges();
      await fixture.whenStable();
    };
    await vi.waitFor(() => {
      fixture.detectChanges();
      expect(el.querySelector('.pc__row, .pc__empty, [role="alert"]')).not.toBeNull();
    });
    const button = (name: string) =>
      Array.from(el.querySelectorAll('button')).find((b) => flat(b)?.includes(name))!;
    return { fixture, el, settle, button };
  }

  it('lists the items with charges and the ammunition of each character, and who has none', async () => {
    const { el } = await render();
    expect(flat(el.querySelector('h2'))).toContain('Cargas e munição');
    const groups = Array.from(el.querySelectorAll('.pc__who-name'), flat);
    expect(groups).toEqual(['Pensantus', 'Tor']);
    const rows = Array.from(el.querySelectorAll('.pc__row'), flat);
    expect(rows[0]).toContain('Varinha de mísseis mágicos');
    expect(rows[0]).toContain('5 de 7');
    expect(rows[0]).toContain('volta 1d6 + 1');
    expect(rows[1]).toContain('0 de 3');
    expect(rows[2]).toContain('Varinha de teia');
    expect(rows[2]).toContain('Destruída');
    expect(rows[3]).toContain('Anel de estrelas cadentes');
    expect(rows[4]).toContain('Flechas');
    expect(rows[4]).toContain('15');
    expect(flat(el.querySelector('.pc__quiet'))).toBe(
      'Brisa não tem itens com cargas nem munição.',
    );
    expect(el.querySelector('.pc__attuned')?.getAttribute('aria-label')).toBe('Sintonizado');
  });

  it('corrects the charges of an item: the stepper starts at what it holds and says what it was', async () => {
    const { el, settle, button } = await render();
    button('Corrigir Varinha de mísseis').click();
    await settle();
    const edit = el.querySelector('.pc__edit')!;
    expect(flat(edit)).toContain('eram 5');
    const plus = edit.querySelector<HTMLButtonElement>('[aria-label^="Mais"]')!;
    plus.click();
    plus.click();
    await settle();
    button('Salvar').click();
    await settle();
    expect(client.setCharges).toHaveBeenCalledWith(
      'camp',
      'pensantus',
      'w1',
      7,
      ItemLoss.UNSPECIFIED,
      expect.any(String),
    );
    expect(el.querySelector('.pc__edit')).toBeNull();
    expect(client.sheetOf).toHaveBeenCalledTimes(9);
  });

  it('reads the sheets again when "Corrigir" opens, and says "eram N" from that read', async () => {
    const { el, settle, button } = await render();
    expect(client.sheetOf).toHaveBeenCalledTimes(3);
    sheets['pensantus'] = {
      magicItems: [item('w1', 'Varinha de mísseis mágicos', 7, 2, '1d6+1')],
      ammunition: [],
    };
    button('Corrigir Varinha de mísseis').click();
    await settle();
    expect(client.sheetOf).toHaveBeenCalledTimes(6);
    const edit = el.querySelector('.pc__edit')!;
    expect(flat(edit)).toContain('eram 2');
    expect(flat(edit.querySelector('app-count-stepper'))).toContain('2');
  });

  it('reads the sheets again when the ammunition "Corrigir" opens, and says "eram N" from that read', async () => {
    const { el, settle, button } = await render();
    sheets['tor'] = {
      magicItems: [],
      ammunition: [
        create(SheetAmmunitionSchema, { key: 'equipment:arrow', namePt: 'Flechas', quantity: 4 }),
      ],
    };
    button('Corrigir Flechas').click();
    await settle();
    expect(flat(el.querySelector('.pc__edit'))).toContain('eram 4');
  });

  it('does not open the correction when the read of the sheets fails', async () => {
    const { el, settle, button } = await render();
    client.sheetOf.mockRejectedValueOnce(new Error('down'));
    button('Corrigir Varinha de mísseis').click();
    await settle();
    expect(el.querySelector('.pc__edit')).toBeNull();
    expect(flat(el.querySelector('[role="alert"] p'))).toBe(
      'Não foi possível ler as cargas do grupo.',
    );
  });

  it('cancels a correction without calling the server and gives the focus back to its button', async () => {
    const { el, settle, button } = await render();
    const corrigir = button('Corrigir Varinha de segredos');
    corrigir.click();
    await settle();
    button('Cancelar').click();
    await settle();
    expect(client.setCharges).not.toHaveBeenCalled();
    expect(el.querySelector('.pc__edit')).toBeNull();
    expect(document.activeElement).toBe(button('Corrigir Varinha de segredos'));
  });

  it('marks an item destroyed through the switch, which sends no charges', async () => {
    const { el, settle, button } = await render();
    button('Corrigir Varinha de mísseis').click();
    await settle();
    el.querySelector<HTMLButtonElement>('[role="switch"]')!.click();
    await settle();
    button('Salvar').click();
    await settle();
    expect(client.setCharges).toHaveBeenCalledWith(
      'camp',
      'pensantus',
      'w1',
      0,
      ItemLoss.GONE,
      expect.any(String),
    );
  });

  it('brings a destroyed item back with the switch off, with the count it is given', async () => {
    const { el, settle, button } = await render();
    button('Corrigir Varinha de teia').click();
    await settle();
    const sw = el.querySelector<HTMLButtonElement>('[role="switch"]')!;
    expect(sw.getAttribute('aria-checked')).toBe('true');
    sw.click();
    await settle();
    el.querySelector<HTMLButtonElement>('[aria-label^="Mais"]')!.click();
    await settle();
    button('Salvar').click();
    await settle();
    expect(client.setCharges).toHaveBeenCalledWith(
      'camp',
      'pensantus',
      'w3',
      1,
      ItemLoss.UNSPECIFIED,
      expect.any(String),
    );
  });

  it('corrects the pieces of a kind of ammunition', async () => {
    const { el, settle, button } = await render();
    button('Corrigir Flechas').click();
    await settle();
    expect(flat(el.querySelector('.pc__edit'))).toContain('eram 15');
    expect(el.querySelector('[role="switch"]')).toBeNull();
    el.querySelector<HTMLButtonElement>('[aria-label^="Mais"]')!.click();
    await settle();
    button('Salvar').click();
    await settle();
    expect(client.setAmmunition).toHaveBeenCalledWith('camp', 'tor', 'equipment:arrow', 16);
  });

  it('says in Portuguese why a correction was refused, and keeps the editor open', async () => {
    client.setCharges.mockRejectedValue(
      new ConnectError('x', Code.FailedPrecondition, undefined, [
        {
          desc: CharacterBlockedSchema,
          value: create(CharacterBlockedSchema, { reason: CharacterBlockedReason.CHARACTER_DEAD }),
        },
      ]),
    );
    const { el, settle, button } = await render();
    button('Corrigir Varinha de segredos').click();
    await settle();
    button('Salvar').click();
    await settle();
    expect(flat(el.querySelector('.pc__edit [role="alert"] p'))).toContain('morto');
    expect(el.querySelector('.pc__edit')).not.toBeNull();
  });

  it('says nothing changed when the save did not get through', async () => {
    client.setAmmunition.mockRejectedValue(new ConnectError('x', Code.Unavailable));
    const { el, settle, button } = await render();
    button('Corrigir Flechas').click();
    await settle();
    button('Salvar').click();
    await settle();
    expect(flat(el.querySelector('.pc__edit [role="alert"] p'))).toBe(
      'Não foi possível salvar. Nada mudou. Tente de novo.',
    );
  });

  it('gives the dawn to the table and lists what each item regained', async () => {
    client.dawn.mockResolvedValue({
      restored: [
        create(DawnRestoredSchema, {
          characterId: 'pensantus',
          regained: 5,
          item: item('w1', 'Varinha de mísseis mágicos', 7, 7, '1d6+1'),
        }),
        create(DawnRestoredSchema, {
          characterId: 'tor',
          regained: 1,
          item: item('r1', 'Anel de estrelas cadentes', 6, 5),
        }),
      ],
    });
    const { el, settle, button } = await render();
    button('Amanhecer').click();
    await settle();
    expect(client.dawn).toHaveBeenCalledWith('camp', expect.any(String));
    const status = el.querySelector('.pc__dawn-region')!;
    expect(flat(status.querySelector('.pc__dawn-head'))).toBe(
      'Amanhecer. 2 itens receberam cargas:',
    );
    expect(Array.from(status.querySelectorAll('li'), flat)).toEqual([
      'Varinha de mísseis mágicos de Pensantus: 1d6 + 1 = 5, fica com 7 de 7.',
      'Anel de estrelas cadentes de Tor: 1d6 = 1, fica com 5 de 6.',
    ]);
    expect(client.sheetOf).toHaveBeenCalledTimes(6);
  });

  it('says no item waits when the dawn gave nothing', async () => {
    const { el, settle, button } = await render();
    button('Amanhecer').click();
    await settle();
    expect(flat(el.querySelector('.pc__dawn-head'))).toBe('Nenhum item espera o amanhecer.');
  });

  it('keeps the dawn off, with the reason, when no item waits for it', async () => {
    sheets['pensantus'] = { magicItems: [item('w1', 'Varinha', 7, 7)], ammunition: [] };
    sheets['tor'] = { magicItems: [], ammunition: [] };
    const { el, button } = await render();
    const dawn = button('Amanhecer');
    expect(dawn.getAttribute('aria-disabled')).toBe('true');
    expect(flat(el.querySelector('.pc__nodawn > span'))).toBe('Nenhum item espera o amanhecer.');
    dawn.click();
    expect(client.dawn).not.toHaveBeenCalled();
  });

  it('words a refused dawn and a dawn that did not get through', async () => {
    client.dawn.mockRejectedValue(
      new ConnectError('x', Code.FailedPrecondition, undefined, [
        {
          desc: ChargesBlockedSchema,
          value: create(ChargesBlockedSchema, { reason: ChargesBlockedReason.COMBAT_RUNNING }),
        },
      ]),
    );
    const { el, settle, button } = await render();
    button('Amanhecer').click();
    await settle();
    expect(flat(el.querySelector('[role="alert"] p'))).toContain('combate');
    client.dawn.mockRejectedValue(new ConnectError('x', Code.Unavailable));
    button('Amanhecer').click();
    await settle();
    expect(flat(el.querySelector('[role="alert"] p'))).toBe(
      'Não foi possível amanhecer. Nada mudou. Tente de novo.',
    );
  });

  it('says it could not read the sheets when a read fails', async () => {
    client.sheetOf.mockRejectedValueOnce(new Error('down'));
    const { el } = await render();
    expect(flat(el.querySelector('[role="alert"] p'))).toBe(
      'Não foi possível ler as cargas do grupo.',
    );
  });
});
