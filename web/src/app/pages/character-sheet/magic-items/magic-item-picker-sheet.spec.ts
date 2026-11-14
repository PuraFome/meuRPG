import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';

import { AttunementClient, type MagicItemOptionVm } from '../../../core/characters/attunement';
import { MagicItemPickerSheet, type MagicItemPickerData } from './magic-item-picker-sheet';

const flat = (n: Element | null | undefined) => n?.textContent?.replace(/\s+/g, ' ').trim();

const cloak: MagicItemOptionVm = {
  key: 'cloak-of-elvenkind',
  namePt: 'Manto élfico',
  category: 'wondrous-item',
  rarity: 'uncommon',
  asks: true,
  restrictionPt: '',
  standalone: true,
  variants: [],
};
const armor: MagicItemOptionVm = {
  key: 'armor-of-resistance',
  namePt: 'Armadura de resistência',
  category: 'armor',
  rarity: 'rare',
  asks: false,
  restrictionPt: '',
  standalone: false,
  variants: [
    {
      key: 'armor-of-resistance-fire',
      namePt: 'Fogo',
      rarity: 'rare',
      asks: true,
      restrictionPt: 'Só para um conjurador',
    },
    {
      key: 'armor-of-resistance-cold',
      namePt: 'Frio',
      rarity: 'rare',
      asks: true,
      restrictionPt: '',
    },
  ],
};

describe('MagicItemPickerSheet', () => {
  const listMagicItems = vi.fn();
  const add = vi.fn();
  const close = vi.fn();

  beforeEach(() => {
    listMagicItems
      .mockReset()
      .mockResolvedValue({ items: [cloak, armor], total: 2, nextPageToken: '' });
    add.mockReset().mockResolvedValue('');
    close.mockReset();
  });

  async function render() {
    const data: MagicItemPickerData = { campaignId: 'camp', add };
    TestBed.configureTestingModule({
      providers: [
        { provide: AttunementClient, useValue: { listMagicItems } },
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close, disableClose: false } },
      ],
    });
    const fixture = TestBed.createComponent(MagicItemPickerSheet);
    fixture.detectChanges();
    const settle = async () => {
      await fixture.whenStable();
      fixture.detectChanges();
      await fixture.whenStable();
    };
    await settle();
    const el = fixture.nativeElement as HTMLElement;
    const addButton = () =>
      Array.from(el.querySelectorAll('button')).find((b) => flat(b)?.endsWith('Adicionar item'))!;
    return { fixture, el, settle, addButton };
  }

  it('lists the items as radio cards with category, rarity and what attunement asks', async () => {
    const { el } = await render();
    expect(listMagicItems).toHaveBeenCalledWith('camp', '');
    const cards = Array.from(el.querySelectorAll('.card'), flat);
    expect(cards[0]).toBe('Manto élfico Item maravilhoso · incomum linkPede sintonização');
    expect(cards[1]).toBe(
      'Armadura de resistência Armadura · raro · 2 variantes linkPede sintonização · Só para um conjurador',
    );
  });

  it('keeps "Adicionar item" off until the choice names one item, and a family asks for its variant', async () => {
    const { el, settle, addButton } = await render();
    expect(addButton().getAttribute('aria-disabled')).toBe('true');
    el.querySelectorAll<HTMLInputElement>('input[type=radio]')[1].click();
    await settle();
    expect(flat(el.querySelector('label[for=magic-item-variant]'))).toBe('Variante');
    expect(addButton().getAttribute('aria-disabled')).toBe('true');
    const select = el.querySelector<HTMLSelectElement>('#magic-item-variant')!;
    select.value = 'armor-of-resistance-cold';
    select.dispatchEvent(new Event('change'));
    await settle();
    expect(addButton().getAttribute('aria-disabled')).not.toBe('true');
    addButton().click();
    await settle();
    expect(add).toHaveBeenCalledWith('armor-of-resistance-cold');
    expect(close).toHaveBeenCalledWith(true);
  });

  it('says why a save was refused and stays open', async () => {
    add.mockResolvedValue('A ficha mudou: leia de novo.');
    const { el, settle, addButton } = await render();
    el.querySelectorAll<HTMLInputElement>('input[type=radio]')[0].click();
    await settle();
    addButton().click();
    await settle();
    expect(flat(el.querySelector('[role=alert]'))).toContain('A ficha mudou: leia de novo.');
    expect(close).not.toHaveBeenCalled();
  });

  it('asks the server again once the typing pauses, and explains an empty answer', async () => {
    const { fixture, el } = await render();
    vi.useFakeTimers();
    try {
      listMagicItems.mockResolvedValue({ items: [], total: 0, nextPageToken: '' });
      const input = el.querySelector<HTMLInputElement>('#magic-item-search')!;
      input.value = 'xyz';
      input.dispatchEvent(new Event('input'));
      await vi.advanceTimersByTimeAsync(249);
      expect(listMagicItems).toHaveBeenCalledTimes(1);
      await vi.advanceTimersByTimeAsync(1);
      expect(listMagicItems).toHaveBeenLastCalledWith('camp', 'xyz');
      await vi.advanceTimersByTimeAsync(0);
      fixture.detectChanges();
    } finally {
      vi.useRealTimers();
    }
    expect(flat(el.querySelector('.mr-muted'))).toContain('Nenhum item mágico com “xyz”.');
  });

  it('offers to try again when the list cannot be read', async () => {
    listMagicItems.mockRejectedValueOnce(new Error('down'));
    const { el, settle } = await render();
    expect(flat(el.querySelector('[role=alert]'))).toContain('Não deu para ler os itens.');
    Array.from(el.querySelectorAll('button'))
      .find((b) => flat(b) === 'Tentar de novo')!
      .click();
    await settle();
    expect(el.querySelectorAll('.card').length).toBe(2);
  });
});
