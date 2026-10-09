import { TestBed } from '@angular/core/testing';

import { RequestChanges } from './request-changes';

describe('RequestChanges', () => {
  function render(busy = false) {
    const fixture = TestBed.createComponent(RequestChanges);
    fixture.componentRef.setInput('characterName', 'Lyra');
    fixture.componentRef.setInput('playerName', 'Lia');
    fixture.componentRef.setInput('busy', busy);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const field = el.querySelector('textarea')!;
    const requested = vi.fn();
    fixture.componentInstance.requested.subscribe(requested);
    const type = (text: string) => {
      field.value = text;
      field.dispatchEvent(new Event('input'));
      fixture.detectChanges();
    };
    const send = () => {
      Array.from(el.querySelectorAll('button'))
        .find((b) => b.textContent?.trim() === 'Enviar pedido')!
        .click();
      fixture.detectChanges();
    };
    return { fixture, el, field, requested, type, send };
  }

  it('says what it asks, with the counter at 0 de 500 and the focus on the field', async () => {
    const { fixture, el, field } = render();
    await fixture.whenStable();
    expect(el.querySelector('h2')?.textContent).toBe('Pedir ajustes em Lyra');
    expect(el.textContent).toContain('O que Lia precisa ajustar');
    expect(el.textContent).toContain('Diga o que não está certo e o que fazer.');
    expect(el.textContent).toContain('0 de 500');
    expect(document.activeElement).toBe(field);
  });

  it('does not send an empty reason, or one of spaces: the field takes the error and the focus', async () => {
    const { fixture, el, field, requested, type, send } = render();
    await fixture.whenStable();
    type('   ');
    (el.querySelector('button') as HTMLElement).focus();

    send();

    expect(requested).not.toHaveBeenCalled();
    expect(el.querySelector('[role="alert"]')?.textContent).toContain(
      'Escreva o que Lia precisa ajustar. O pedido não vai sem motivo.',
    );
    expect(field.getAttribute('aria-invalid')).toBe('true');
    expect(document.activeElement).toBe(field);
  });

  it('accepts 500 characters and sends them trimmed, and says "passa de 500" from the 501st', () => {
    const { el, requested, type, send } = render();
    type(` ${'a'.repeat(500)} `);
    expect(el.textContent).toContain('500 de 500');
    send();
    expect(requested).toHaveBeenCalledExactlyOnceWith('a'.repeat(500));

    type('a'.repeat(501));
    expect(el.querySelector('[role="alert"]')?.textContent).toContain(
      'O motivo passa de 500 caracteres.',
    );
    send();
    expect(requested).toHaveBeenCalledTimes(1);
  });

  it('sends nothing while a call is on its way, and Cancelar says so', () => {
    const { fixture, el, requested, type, send } = render(true);
    const cancelled = vi.fn();
    fixture.componentInstance.cancelled.subscribe(cancelled);
    type('Falta o equipamento.');
    send();
    expect(requested).not.toHaveBeenCalled();
    Array.from(el.querySelectorAll('button'))
      .find((b) => b.textContent?.trim() === 'Cancelar')!
      .click();
    expect(cancelled).toHaveBeenCalledTimes(1);
  });
});
