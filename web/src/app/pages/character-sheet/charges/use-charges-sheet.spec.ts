import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';

import { type UseChargesData, UseChargesSheet } from './use-charges-sheet';

const flat = (n: Element | null | undefined) => n?.textContent?.replace(/\s+/g, ' ').trim();

describe('UseChargesSheet', () => {
  const spend = vi.fn();
  const close = vi.fn();

  beforeEach(() => {
    spend.mockReset().mockResolvedValue('');
    close.mockReset();
  });

  function render(over: Partial<UseChargesData> = {}) {
    const data: UseChargesData = {
      itemName: 'Varinha de mísseis mágicos',
      max: 7,
      left: 5,
      spend,
      ...over,
    };
    TestBed.configureTestingModule({
      providers: [
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close, disableClose: false } },
      ],
    });
    const fixture = TestBed.createComponent(UseChargesSheet);
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

  it('names the item and says how many charges are left', () => {
    const { el } = render();
    expect(flat(el.querySelector('h2'))).toBe('Usar · Varinha de mísseis mágicos');
    expect(flat(el)).toContain('Restam 5 de 7 cargas');
    expect(el.querySelectorAll('app-count-pips .pip--left')).toHaveLength(5);
    expect(flat(el)).toContain('Cargas a gastar');
    expect(flat(el)).toContain('De 1 a 5.');
  });

  it('says "Resta 1 de 7 cargas" for the last one', () => {
    const { el } = render({ left: 1 });
    expect(flat(el)).toContain('Resta 1 de 7 cargas');
    expect(flat(el)).toContain('Gastar 1 carga');
  });

  it('steps the amount from 1 to what is left and puts it on the button', () => {
    const { el, button, fixture } = render({ left: 3 });
    expect(flat(el.querySelector('.cs__value'))).toBe('1');
    button('Mais um').click();
    button('Mais um').click();
    button('Mais um').click();
    fixture.detectChanges();
    expect(flat(el.querySelector('.cs__value'))).toBe('3');
    expect(button('Gastar').textContent).toContain('Gastar 3 cargas');
    button('Menos um').click();
    fixture.detectChanges();
    expect(button('Gastar').textContent).toContain('Gastar 2 cargas');
  });

  it('spends the amount under one key and closes with true', async () => {
    const { button, settle } = render();
    button('Mais um').click();
    button('Gastar').click();
    await settle();
    expect(spend).toHaveBeenCalledWith(2, expect.any(String));
    expect(close).toHaveBeenCalledWith(true);
  });

  it('sends the same key again for the same amount after a failure, so nothing is spent twice', async () => {
    spend.mockResolvedValueOnce('Não foi possível gastar a carga. Nada mudou. Tente de novo.');
    const { el, button, settle } = render();
    button('Gastar').click();
    await settle();
    expect(flat(el.querySelector('[role=alert]'))).toContain('Nada mudou. Tente de novo.');
    expect(close).not.toHaveBeenCalled();
    button('Gastar').click();
    await settle();
    expect(spend.mock.calls[1][1]).toBe(spend.mock.calls[0][1]);
    expect(close).toHaveBeenCalledWith(true);
  });

  it('says "Gastando..." while the call is on its way and does not send twice', async () => {
    let done: (v: string) => void = () => undefined;
    spend.mockReturnValue(new Promise<string>((r) => (done = r)));
    const { button, settle } = render();
    button('Gastar').click();
    await settle();
    expect(flat(button('Gastando'))).toContain('Gastando...');
    button('Gastando').click();
    expect(spend).toHaveBeenCalledTimes(1);
    done('');
    await settle();
    expect(close).toHaveBeenCalledWith(true);
  });

  it('closes without spending on "Fechar"', () => {
    const { button } = render();
    button('Fechar').click();
    expect(spend).not.toHaveBeenCalled();
    expect(close).toHaveBeenCalled();
  });
});
