import { TestBed } from '@angular/core/testing';

import { ActionRow } from './action-row';

describe('ActionRow', () => {
  function setup(inputs: Record<string, unknown>) {
    const fixture = TestBed.createComponent(ActionRow);
    for (const [k, v] of Object.entries(inputs)) {
      fixture.componentRef.setInput(k, v);
    }
    const pressed: number[] = [];
    fixture.componentInstance.press.subscribe(() => pressed.push(1));
    fixture.detectChanges();
    return { el: fixture.nativeElement as HTMLElement, pressed };
  }

  it('shows the name, the pill, the detail and the button with its own accessible name', () => {
    const { el, pressed } = setup({
      name: 'Raio de Fogo',
      pill: 'Truque',
      detail: '+6 para acertar · 1d10 de fogo · alcance 36 m',
      button: 'Atacar',
      buttonLabel: 'Atacar com Raio de Fogo',
    });
    expect(el.textContent).toContain('Truque');
    const button = el.querySelector('button')!;
    expect(button.getAttribute('aria-label')).toBe('Atacar com Raio de Fogo');
    button.click();
    expect(pressed).toEqual([1]);
  });

  it('keeps a disabled option in place and in the keyboard order, with the reason wired to the button', () => {
    const { el, pressed } = setup({ name: 'Teia', pill: '2º círculo', button: 'Conjurar', off: true, reason: 'Sem espaço de 2º círculo ou maior' });
    const button = el.querySelector('button')!;
    expect(button.disabled).toBe(false); // `disabledInteractive`: still a focus stop
    expect(button.getAttribute('aria-disabled')).toBe('true');
    const why = el.querySelector('.row__why')!;
    expect(why.textContent).toContain('Sem espaço de 2º círculo ou maior');
    expect(button.getAttribute('aria-describedby')).toBe(why.id);
    button.click();
    expect(pressed).toEqual([]);
  });

  it('has no button for a row that waits for something else (Escudo)', () => {
    const { el } = setup({ name: 'Escudo', detail: 'Quando você for atingido, o app pergunta se quer usar.' });
    expect(el.querySelector('button')).toBeNull();
  });
});
