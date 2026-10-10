import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { provideRouter } from '@angular/router';
import { Code, ConnectError } from '@connectrpc/connect';

import { flat } from '../../../core/creatures/creatures-testing';
import { MagicItemRarity } from '../../../../gen/meurpg/maps/v1/treasure_pb';
import { DescriptionLanguage } from '../../../core/text/description-language';
import { TreasureClient } from '../../../core/treasure/treasure-client';
import { FakeTreasureClient, magicItemResponse } from '../../../core/treasure/treasure-testing';
import { ItemSheet } from './item-sheet';

describe('ItemSheet: "Ver descrição" (MR-044, E10-10 state 3)', () => {
  let api: FakeTreasureClient;
  let close: ReturnType<typeof vi.fn>;

  async function setup(key = 'item:ring-of-protection', namePt = 'Anel de Proteção') {
    api = new FakeTreasureClient();
    api.items.set(
      'item:potion-of-healing-1',
      magicItemResponse({
        key: 'item:potion-of-healing-1',
        name: 'Potion of Healing',
        namePt: 'Poção de Cura',
        rarity: MagicItemRarity.COMMON,
        valuePo: 50,
        halved: true,
        consumable: true,
        attunement: false,
        description: ['Potion, common', 'You regain hit points when you drink this potion.'],
      }),
    );
    close = vi.fn();
    TestBed.configureTestingModule({
      providers: [
        provideRouter([]),
        { provide: TreasureClient, useValue: api },
        { provide: MAT_DIALOG_DATA, useValue: { campaignId: 'camp-1', key, namePt } },
        { provide: MatDialogRef, useValue: { close } },
      ],
    });
    const fixture = TestBed.createComponent(ItemSheet);
    const settle = async () => {
      for (let i = 0; i < 3; i++) {
        fixture.detectChanges();
        await fixture.whenStable();
      }
    };
    await settle();
    return { fixture, el: fixture.nativeElement as HTMLElement, settle };
  }

  it('shows the Portuguese name, the SRD name in English, the rarity and the attunement as words', async () => {
    const { el } = await setup();
    expect(flat(el.querySelector('.frame__title'))).toBe('Anel de Proteção');
    expect(el.querySelector('.names__en')?.getAttribute('lang')).toBe('en');
    expect(flat(el.querySelector('.names'))).toBe('Ring of Protection · item mágico do SRD 5.1');
    expect(Array.from(el.querySelectorAll('.tags li')).map((l) => flat(l))).toEqual([
      'Raro',
      'Exige sintonização',
    ]);
  });

  it('gives the value with the 2024 label, the credits link and the rule in words', async () => {
    const { el } = await setup();
    expect(flat(el.querySelector('.value__num'))?.replace(/\u00a0/g, ' ')).toBe('4.000 PO');
    expect(flat(el.querySelector('.value__src'))).toBe(
      'Valores do SRD 5.2.1 (regras de 2024) · Créditos (abre em outra aba)',
    );
    expect(el.querySelector('.value__src a')?.getAttribute('href')).toBe('/credits');
    expect(flat(el.querySelector('.value__rule'))).toBe(
      'Itens que se gastam valem a metade, menos os pergaminhos de magia. Este não se gasta.',
    );
  });

  it('an item with no translation yet shows the English with lang="en", one paragraph per entry, "(em inglês)" and no toggle', async () => {
    const { el } = await setup();
    expect(flat(el.querySelector('.text__src'))).toBe('Texto do SRD 5.1 (em inglês)');
    const body = el.querySelector('.text__body .srd')!;
    expect(body.getAttribute('lang')).toBe('en');
    expect(Array.from(body.querySelectorAll('p')).map((p) => p.textContent)).toEqual([
      'Ring, rare (requires attunement)',
      'You gain a +1 bonus to AC and saving throws while wearing this ring.',
    ]);
    expect(el.querySelector('app-description-lang-button')).toBeNull();
  });

  it('shows the Portuguese text first, with a table, and "Ver em inglês" flips it for the whole app', async () => {
    api = new FakeTreasureClient();
    api.items.set(
      'item:ring-of-protection',
      magicItemResponse({
        description: ['Ring, rare', 'A | B', '| Type | Roll |', '|---|---|', '| Silver | 1 |'],
        descriptionPt: ['Anel, raro', 'A | B', '| Tipo | Rolagem |', '|---|---|', '| Prata | 1 |'],
      }),
    );
    close = vi.fn();
    TestBed.configureTestingModule({
      providers: [
        provideRouter([]),
        { provide: TreasureClient, useValue: api },
        {
          provide: MAT_DIALOG_DATA,
          useValue: { campaignId: 'camp-1', key: 'item:ring-of-protection', namePt: 'Anel' },
        },
        { provide: MatDialogRef, useValue: { close } },
      ],
    });
    const fixture = TestBed.createComponent(ItemSheet);
    const settle = async () => {
      for (let i = 0; i < 3; i++) {
        fixture.detectChanges();
        await fixture.whenStable();
      }
    };
    await settle();
    const el = fixture.nativeElement as HTMLElement;
    expect(flat(el.querySelector('.text__src'))).toBe('Texto do SRD 5.1 (tradução nossa)');
    expect(el.querySelector('.text__body .srd')?.getAttribute('lang')).toBeNull();
    expect(flat(el.querySelector('th'))).toBe('Tipo');
    const button = el.querySelector<HTMLButtonElement>('app-description-lang-button button')!;
    expect(flat(button)).toBe('Ver em inglês');
    button.click();
    await settle();
    expect(el.querySelector('.text__body .srd')?.getAttribute('lang')).toBe('en');
    expect(flat(el.querySelector('th'))).toBe('Type');
    expect(flat(el.querySelector('app-description-lang-button button'))).toBe('Ver em português');
    expect(TestBed.inject(DescriptionLanguage).english()).toBe(true);
    TestBed.inject(DescriptionLanguage).toggle();
  });

  it('a potion says the halving: "Comum vale 100 PO; um item que se gasta vale a metade."', async () => {
    const { el } = await setup('item:potion-of-healing-1', 'Poção de Cura');
    expect(flat(el.querySelector('.value__num'))?.replace(/\u00a0/g, ' ')).toBe('50 PO');
    expect(flat(el.querySelector('.value__rule'))?.replace(/\u00a0/g, ' ')).toBe(
      'Comum vale 100 PO; um item que se gasta vale a metade.',
    );
    expect(Array.from(el.querySelectorAll('.tags li')).map((l) => flat(l))).toEqual([
      'Comum',
      'Consumível',
    ]);
  });

  it('"Fechar" closes, and it is the first focus target', async () => {
    const { el } = await setup();
    const button = el.querySelector<HTMLButtonElement>('button[data-initial-focus]')!;
    expect(flat(button)).toBe('Fechar');
    button.click();
    expect(close).toHaveBeenCalledTimes(1);
  });

  it('an item that is gone says so, with "Tentar de novo"', async () => {
    const { el, settle } = await setup('item:nothing', 'Nada');
    expect(el.querySelector('[role=alert]')?.textContent).toContain('não existe mais no SRD');
    api.items.set(
      'item:nothing',
      magicItemResponse({ key: 'item:nothing', namePt: 'Nada', name: 'Nothing' }),
    );
    Array.from(el.querySelectorAll<HTMLButtonElement>('button'))
      .find((b) => flat(b) === 'Tentar de novo')!
      .click();
    await settle();
    expect(flat(el.querySelector('.names'))).toContain('Nothing');
  });

  it('a lost server says it, not a code', async () => {
    api = new FakeTreasureClient();
    api.failWith.set('item', new ConnectError('down', Code.Unavailable));
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({
      providers: [
        provideRouter([]),
        { provide: TreasureClient, useValue: api },
        {
          provide: MAT_DIALOG_DATA,
          useValue: {
            campaignId: 'camp-1',
            key: 'item:ring-of-protection',
            namePt: 'Anel de Proteção',
          },
        },
        { provide: MatDialogRef, useValue: { close: vi.fn() } },
      ],
    });
    const fixture = TestBed.createComponent(ItemSheet);
    for (let i = 0; i < 3; i++) {
      fixture.detectChanges();
      await fixture.whenStable();
    }
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelector('[role=alert]')?.textContent).toContain('o servidor não respondeu');
    // The title is the row's name while nothing came back.
    expect(flat(el.querySelector('.frame__title'))).toBe('Anel de Proteção');
  });
});
