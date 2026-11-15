import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';

import { type AddAmmunitionData, AddAmmunitionSheet } from './add-ammunition-sheet';

const flat = (n: Element | null | undefined) => n?.textContent?.replace(/\s+/g, ' ').trim();

describe('AddAmmunitionSheet', () => {
  const add = vi.fn();
  const close = vi.fn();

  beforeEach(() => {
    add.mockReset().mockResolvedValue('');
    close.mockReset();
  });

  function render(over: Partial<AddAmmunitionData> = {}) {
    const data: AddAmmunitionData = { current: {}, add, ...over };
    TestBed.configureTestingModule({
      providers: [
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close, disableClose: false } },
      ],
    });
    const fixture = TestBed.createComponent(AddAmmunitionSheet);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const button = (name: string) =>
      Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) =>
        (b.getAttribute('aria-label') ?? flat(b) ?? '').includes(name),
      )!;
    const settle = async () => {
      await fixture.whenStable();
      fixture.detectChanges();
    };
    return { el, button, settle, fixture };
  }

  it('lists the four SRD kinds with what fires them and the bundle', () => {
    const { el } = render();
    expect(flat(el.querySelector('h2'))).toBe('Adicionar munição');
    const cards = Array.from(el.querySelectorAll('app-option-card'), flat);
    expect(cards).toHaveLength(4);
    expect(cards[0]).toContain('Flechas');
    expect(cards[0]).toContain('Arcos · vem 20');
    expect(cards[3]).toContain('Agulhas de zarabatana');
    expect(cards[3]).toContain('Zarabatana · vem 50');
  });

  it('starts on arrows with the bundle of the kind, and follows the kind picked', () => {
    const { el, button, fixture } = render();
    expect(flat(el.querySelector('.cs__value'))).toBe('20');
    expect(button('Adicionar').textContent).toContain('Adicionar 20 flechas');
    el.querySelectorAll<HTMLInputElement>('app-option-card input')[3].click();
    fixture.detectChanges();
    expect(flat(el.querySelector('.cs__value'))).toBe('50');
    expect(button('Adicionar').textContent).toContain('Adicionar 50 agulhas de zarabatana');
  });

  it('opens on the kind and the count the free text named', () => {
    const { el } = render({ kind: 'equipment:crossbow-bolt', quantity: 12 });
    expect(flat(el.querySelector('.cs__value'))).toBe('12');
    expect(el.querySelectorAll<HTMLInputElement>('app-option-card input')[1].checked).toBe(true);
  });

  it('steps the count and says "1 flecha" for one', () => {
    const { el, button, fixture } = render({ quantity: 2 });
    button('Menos uma').click();
    fixture.detectChanges();
    expect(flat(el.querySelector('.cs__value'))).toBe('1');
    expect(button('Adicionar').textContent).toContain('Adicionar 1 flecha');
  });

  it('sets what the kind holds now plus the count, and says so', async () => {
    const { el, button, settle } = render({ current: { 'equipment:arrow': 15 } });
    expect(flat(el)).toContain('Já tem 15: ficam 35.');
    button('Adicionar').click();
    await settle();
    expect(add).toHaveBeenCalledWith('equipment:arrow', 35);
    expect(close).toHaveBeenCalledWith(true);
  });

  it('shows the refusal and stays open', async () => {
    add.mockResolvedValue('A ficha está travada.');
    const { el, button, settle } = render();
    button('Adicionar').click();
    await settle();
    expect(flat(el.querySelector('[role=alert]'))).toContain('A ficha está travada.');
    expect(close).not.toHaveBeenCalled();
  });

  it('closes without adding on "Cancelar"', () => {
    const { button } = render();
    button('Cancelar').click();
    expect(add).not.toHaveBeenCalled();
    expect(close).toHaveBeenCalled();
  });
});
