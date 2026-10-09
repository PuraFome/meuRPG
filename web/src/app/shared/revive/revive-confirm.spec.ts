import { TestBed } from '@angular/core/testing';

import { ReviveConfirm } from './revive-confirm';

describe('ReviveConfirm', () => {
  async function render(name = 'Toren', busy = false) {
    const fixture = TestBed.createComponent(ReviveConfirm);
    fixture.componentRef.setInput('characterName', name);
    fixture.componentRef.setInput('busy', busy);
    fixture.detectChanges();
    await fixture.whenStable();
    const el = fixture.nativeElement as HTMLElement;
    const button = (text: string) =>
      Array.from(el.querySelectorAll('button')).find((b) => b.textContent?.trim() === text)!;
    return { fixture, el, button };
  }

  it('is an alertdialog named by its question, with the four lines', async () => {
    const { el } = await render();
    const dialog = el.querySelector('[role="alertdialog"]')!;
    expect(dialog.getAttribute('aria-labelledby')).toBe('rv-title');
    expect(el.querySelector('#rv-title')?.textContent).toBe('Reviver Toren?');
    const lines = Array.from(el.querySelectorAll('#rv-text p')).map((p) =>
      p.textContent!.replace(/\s+/g, ' ').trim(),
    );
    expect(lines).toEqual([
      'Toren volta com 1 PV, sem a condição Inconsciente e com os testes contra a morte zerados.',
      'Os espaços de magia e os usos de classe ficam como estavam.',
      'Com um combate aberto, ele volta à ordem de iniciativa onde estava e age no próximo turno dele.',
      'O registro diz “O mestre reviveu Toren”.',
    ]);
  });

  it('writes the third line in the feminine for a name that asks for it', async () => {
    const { el } = await render('Ilaria');
    expect(el.querySelector('#rv-text')?.textContent).toContain(
      'ela volta à ordem de iniciativa onde estava e age no próximo turno dela.',
    );
  });

  it('opens with the focus on the outlined "Reviver Toren"', async () => {
    const { button } = await render();
    const confirm = button('Reviver Toren');
    expect(confirm.classList).toContain('mat-mdc-outlined-button');
    expect(document.activeElement).toBe(confirm);
  });

  it('confirms with the button, cancels with "Cancelar" and with Escape', async () => {
    const { fixture, el, button } = await render();
    const confirmed = vi.fn();
    const cancelled = vi.fn();
    fixture.componentInstance.confirmed.subscribe(confirmed);
    fixture.componentInstance.cancelled.subscribe(cancelled);

    button('Reviver Toren').click();
    button('Cancelar').click();
    el.querySelector('[role="alertdialog"]')!.dispatchEvent(
      new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }),
    );

    expect(confirmed).toHaveBeenCalledTimes(1);
    expect(cancelled).toHaveBeenCalledTimes(2);
  });

  it('cannot confirm while the call is on its way', async () => {
    const { button } = await render('Toren', true);
    expect(button('Reviver Toren').disabled).toBe(true);
  });
});
