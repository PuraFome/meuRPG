import { TestBed } from '@angular/core/testing';

import { RejectConfirm } from './reject-confirm';

describe('RejectConfirm', () => {
  function render(busy = false) {
    const fixture = TestBed.createComponent(RejectConfirm);
    fixture.componentRef.setInput('characterName', 'Lyra');
    fixture.componentRef.setInput('playerName', 'Lia');
    fixture.componentRef.setInput('busy', busy);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const button = (text: string) =>
      Array.from(el.querySelectorAll('button')).find((b) => b.textContent?.trim() === text)!;
    return { fixture, el, button };
  }

  it('asks the way the design says, and points to "Pedir ajustes"', () => {
    const { el } = render();
    expect(el.querySelector('h2')?.textContent).toBe('Recusar Lyra?');
    expect(el.textContent).toContain(
      'O personagem é apagado e isto não se desfaz. Lia precisa de um convite novo para tentar de novo. Se a ficha só precisa de ajustes, use “Pedir ajustes”.',
    );
  });

  it('has "Confirmar recusa" outlined in the danger colour, never filled', () => {
    const { button } = render();
    expect(button('Confirmar recusa').classList).toContain('mat-mdc-outlined-button');
    expect(button('Confirmar recusa').classList).toContain('danger-outline');
    expect(button('Confirmar recusa').classList).not.toContain('mat-mdc-unelevated-button');
  });

  it('confirms or cancels, and cannot confirm while a call is on its way', () => {
    const { fixture, button } = render();
    const confirmed = vi.fn();
    const cancelled = vi.fn();
    fixture.componentInstance.confirmed.subscribe(confirmed);
    fixture.componentInstance.cancelled.subscribe(cancelled);
    button('Confirmar recusa').click();
    button('Cancelar').click();
    expect(confirmed).toHaveBeenCalledTimes(1);
    expect(cancelled).toHaveBeenCalledTimes(1);

    fixture.componentRef.setInput('busy', true);
    fixture.detectChanges();
    expect(button('Confirmar recusa').disabled).toBe(true);
  });
});
