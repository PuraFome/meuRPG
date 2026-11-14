import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';

import { RemoveMagicItemSheet, type RemoveMagicItemData } from './remove-magic-item-sheet';

const flat = (n: Element | null | undefined) => n?.textContent?.replace(/\s+/g, ' ').trim();

describe('RemoveMagicItemSheet', () => {
  const remove = vi.fn();
  const close = vi.fn();

  beforeEach(() => {
    remove.mockReset().mockResolvedValue('');
    close.mockReset();
  });

  function render(attunedBefore: number | null) {
    const data: RemoveMagicItemData = { itemName: 'Manto élfico', attunedBefore, remove };
    TestBed.configureTestingModule({
      providers: [
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close, disableClose: false } },
      ],
    });
    const fixture = TestBed.createComponent(RemoveMagicItemSheet);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const button = (name: string) =>
      Array.from(el.querySelectorAll('button')).find((b) => flat(b)?.endsWith(name))!;
    const settle = async () => {
      await fixture.whenStable();
      fixture.detectChanges();
    };
    return { el, button, settle };
  }

  it('names the item and says the attunement ends with it, with the count going down', () => {
    const { el } = render(3);
    expect(flat(el.querySelector('h2'))).toBe('Remover Manto élfico?');
    expect(flat(el.querySelector('p'))).toBe(
      'O item sai da ficha, e a sintonização acaba com ele: Sintonizados 3 → 2.',
    );
  });

  it('only says the item leaves when it was not attuned', () => {
    const { el } = render(null);
    expect(flat(el.querySelector('p'))).toBe('O item sai da ficha.');
  });

  it('removes and closes with true', async () => {
    const { button, settle } = render(null);
    button('Remover').click();
    await settle();
    expect(remove).toHaveBeenCalledTimes(1);
    expect(close).toHaveBeenCalledWith(true);
  });

  it('shows the refusal and stays open', async () => {
    remove.mockResolvedValue('A ficha mudou: leia de novo.');
    const { el, button, settle } = render(null);
    button('Remover').click();
    await settle();
    expect(flat(el.querySelector('[role=alert]'))).toContain('A ficha mudou: leia de novo.');
    expect(close).not.toHaveBeenCalled();
  });

  it('cancels without removing', () => {
    const { button } = render(null);
    button('Cancelar').click();
    expect(remove).not.toHaveBeenCalled();
    expect(close).toHaveBeenCalled();
  });
});
